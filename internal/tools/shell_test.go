// shell_test.go — tests for the shell tool.
package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"blazeai/internal/platform"
)

// TestShellExecuteSuccess verifies a simple command runs and returns output.
func TestShellExecuteSuccess(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	args := json.RawMessage(`{"command":"echo hello"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "hello") {
		t.Errorf("Execute() = %q, want output containing 'hello'", result)
	}
	if !strings.Contains(result, "exit_code: 0") {
		t.Errorf("Execute() = %q, want exit_code: 0", result)
	}
}

// TestShellExecuteStderr verifies stderr is captured.
func TestShellExecuteStderr(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	args := json.RawMessage(`{"command":"echo error_msg >&2"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "error_msg") {
		t.Errorf("Execute() = %q, want stderr containing 'error_msg'", result)
	}
}

// TestShellExecuteNonZeroExit verifies non-zero exit code is captured.
func TestShellExecuteNonZeroExit(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	args := json.RawMessage(`{"command":"exit 42"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "exit_code: 42") {
		t.Errorf("Execute() = %q, want exit_code: 42", result)
	}
}

// TestShellExecuteTimeout verifies timeout returns the correct message.
func TestShellExecuteTimeout(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	// Use a 1-second timeout with a 5-second sleep.
	args := json.RawMessage(`{"command":"sleep 5","timeout":1}`)
	result := tool.Execute(context.Background(), args)
	expected := "timeout 1s exceeded"
	if result != expected {
		t.Errorf("Execute() = %q, want %q", result, expected)
	}
}

// TestShellExecuteTimeoutKillsBackgroundChildren verifies timeout returns even when the
// shell command leaves a background child running.
func TestShellExecuteTimeoutKillsBackgroundChildren(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	args := json.RawMessage(`{"command":"sleep 30 & sleep 30","timeout":1}`)
	resultCh := make(chan string, 1)
	go func() {
		resultCh <- tool.Execute(context.Background(), args)
	}()

	select {
	case result := <-resultCh:
		if result != "timeout 1s exceeded" {
			t.Errorf("Execute() = %q, want %q", result, "timeout 1s exceeded")
		}
	case <-time.After(4 * time.Second):
		t.Fatal("Execute() hung with background child after timeout")
	}
}

// TestShellExecuteUserAbort verifies cancellation returns partial aborted output instead of timeout.
func TestShellExecuteUserAbort(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	args := json.RawMessage(`{"command":"echo start; sleep 30","timeout":60}`)
	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan string, 1)
	go func() {
		resultCh <- tool.Execute(ctx, args)
	}()
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case result := <-resultCh:
		if !strings.Contains(result, "aborted by user") {
			t.Errorf("Execute() = %q, want aborted by user", result)
		}
		if !strings.Contains(result, "start") {
			t.Errorf("Execute() = %q, want partial stdout", result)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("Execute() hung after user abort")
	}
}

// TestShellExecuteDefaultTimeout verifies default timeout is used when not specified.
func TestShellExecuteDefaultTimeout(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	// Just verify the tool works without a timeout parameter.
	args := json.RawMessage(`{"command":"echo quick"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "quick") {
		t.Errorf("Execute() = %q, want output containing 'quick'", result)
	}
}

// TestShellExecuteEmptyCommand verifies error on empty command.
func TestShellExecuteEmptyCommand(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	args := json.RawMessage(`{"command":""}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "error") {
		t.Errorf("Execute() = %q, want error message", result)
	}
}

// TestShellExecuteInvalidArgs verifies error on invalid JSON.
func TestShellExecuteInvalidArgs(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	args := json.RawMessage(`{invalid}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "error") {
		t.Errorf("Execute() = %q, want error message", result)
	}
}

// TestShellName verifies the tool name.
func TestShellName(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	if tool.Name() != "shell" {
		t.Errorf("Name() = %q, want 'shell'", tool.Name())
	}
}

// TestShellDescription verifies description is non-empty.
func TestShellDescription(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	if tool.Description() == "" {
		t.Error("Description() is empty")
	}
}

// TestShellParameters verifies parameters is valid JSON.
func TestShellParameters(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	params := tool.Parameters()
	if !json.Valid(params) {
		t.Errorf("Parameters() is not valid JSON: %s", params)
	}
	schemaIncludesRequiredPurpose(t, params)
}

// TestShellExecuteMultilineOutput verifies multiline command output.
func TestShellExecuteMultilineOutput(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	args := json.RawMessage(`{"command":"echo line1 && echo line2"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "line1") || !strings.Contains(result, "line2") {
		t.Errorf("Execute() = %q, want both lines", result)
	}
}

// TestShellExecuteOutputLimitStdout verifies broad stdout output is stopped at the hard cap.
func TestShellExecuteOutputLimitStdout(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	args := json.RawMessage(`{"command":"yes x | head -c 200000"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "shell output exceeded the 150 kB limit") {
		t.Fatalf("Execute() = %q, want output limit error", result)
	}
	if !strings.Contains(result, "Refine the command") || !strings.Contains(result, "sequential chunks below 150 kB") {
		t.Fatalf("Execute() = %q, want refinement guidance", result)
	}
	if strings.Contains(result, "stdout:\n") {
		t.Fatalf("Execute() = %q, should not include partial stdout", result)
	}
}

// TestShellExecuteOutputLimitStderr verifies broad stderr output is stopped at the hard cap.
func TestShellExecuteOutputLimitStderr(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	args := json.RawMessage(`{"command":"yes x | head -c 200000 1>&2"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "shell output exceeded the 150 kB limit") {
		t.Fatalf("Execute() = %q, want output limit error", result)
	}
	if !strings.Contains(result, "stderr_bytes=") {
		t.Fatalf("Execute() = %q, want stderr byte metadata", result)
	}
	if strings.Contains(result, "stderr:\n") {
		t.Fatalf("Execute() = %q, should not include partial stderr", result)
	}
}
