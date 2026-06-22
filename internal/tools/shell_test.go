// shell_test.go — tests for the shell tool.
package tools

import (
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
	result := tool.Execute(args)
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
	result := tool.Execute(args)
	if !strings.Contains(result, "error_msg") {
		t.Errorf("Execute() = %q, want stderr containing 'error_msg'", result)
	}
}

// TestShellExecuteNonZeroExit verifies non-zero exit code is captured.
func TestShellExecuteNonZeroExit(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	args := json.RawMessage(`{"command":"exit 42"}`)
	result := tool.Execute(args)
	if !strings.Contains(result, "exit_code: 42") {
		t.Errorf("Execute() = %q, want exit_code: 42", result)
	}
}

// TestShellExecuteTimeout verifies timeout returns the correct message.
func TestShellExecuteTimeout(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	// Use a 1-second timeout with a 5-second sleep.
	args := json.RawMessage(`{"command":"sleep 5","timeout":1}`)
	result := tool.Execute(args)
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
		resultCh <- tool.Execute(args)
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

// TestShellExecuteDefaultTimeout verifies default timeout is used when not specified.
func TestShellExecuteDefaultTimeout(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	// Just verify the tool works without a timeout parameter.
	args := json.RawMessage(`{"command":"echo quick"}`)
	result := tool.Execute(args)
	if !strings.Contains(result, "quick") {
		t.Errorf("Execute() = %q, want output containing 'quick'", result)
	}
}

// TestShellExecuteEmptyCommand verifies error on empty command.
func TestShellExecuteEmptyCommand(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	args := json.RawMessage(`{"command":""}`)
	result := tool.Execute(args)
	if !strings.Contains(result, "error") {
		t.Errorf("Execute() = %q, want error message", result)
	}
}

// TestShellExecuteInvalidArgs verifies error on invalid JSON.
func TestShellExecuteInvalidArgs(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	args := json.RawMessage(`{invalid}`)
	result := tool.Execute(args)
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
}

// TestShellExecuteMultilineOutput verifies multiline command output.
func TestShellExecuteMultilineOutput(t *testing.T) {
	tool := NewShellTool(platform.Linux)
	args := json.RawMessage(`{"command":"echo line1 && echo line2"}`)
	result := tool.Execute(args)
	if !strings.Contains(result, "line1") || !strings.Contains(result, "line2") {
		t.Errorf("Execute() = %q, want both lines", result)
	}
}
