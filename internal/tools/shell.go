// shell.go — shell tool implementation for executing commands on the host.
// Runs commands via the platform-selected shell with an optional timeout.
// Returns raw stdout, stderr, and exit_code, or "timeout <N>s exceeded" on timeout.
// Layer: tool execution. Dependencies: internal/platform.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"blazeai/internal/platform"
)

// ShellArgs are the arguments for the shell tool.
//
// WHAT:  Parsed arguments from the LLM tool call.
// PARAMS: Command — the shell command string; Timeout — optional timeout in seconds (default 60).
type ShellArgs struct {
	Command string `json:"command"`
	Timeout *int   `json:"timeout,omitempty"`
}

// ShellTool executes shell commands on the host via the platform-selected shell.
//
// WHAT:  Runs a shell command with timeout and returns raw output.
// WHY:   Shell is the primary tool for the agent — most tasks go through it.
// PARAMS: os — the detected OS for shell selection.
type ShellTool struct {
	os platform.OS
}

// NewShellTool creates a ShellTool for the given OS.
//
// PARAMS: os — the detected operating system.
// RETURNS: *ShellTool — ready to execute commands.
func NewShellTool(os platform.OS) *ShellTool {
	return &ShellTool{os: os}
}

// Name returns the tool's unique identifier.
func (s *ShellTool) Name() string {
	return "shell"
}

// FormatArgs extracts the command for display.
func (s *ShellTool) FormatArgs(args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[ShellArgs](args)
	if err != nil || parsed.Command == "" {
		return ""
	}
	return parsed.Command
}

// Description returns the human-readable description for the LLM.
func (s *ShellTool) Description() string {
	return "Execute a shell command on the host. Returns stdout, stderr, and exit_code."
}

// Parameters returns the JSON schema for the tool's parameters.
func (s *ShellTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "The shell command to execute."
			},
			"timeout": {
				"type": "integer",
				"description": "Optional timeout in seconds. Default: 60."
			}
		},
		"required": ["command"]
	}`)
}

// Execute runs the shell command with timeout and returns the result.
//
// WHAT:  Executes the command via the platform shell and returns formatted output.
// WHY:   This is the primary execution path for the agent.
// HOW:   Resolves the shell, starts it in its own process group when supported, waits with timeout,
//
//	and kills the full process group on timeout to avoid background children keeping pipes open.
//
// PARAMS: args — raw JSON with command and optional timeout.
// RETURNS: string — formatted stdout/stderr/exit_code, or "timeout <N>s exceeded" on timeout.
func (s *ShellTool) Execute(args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[ShellArgs](args)
	if err != nil {
		return fmt.Sprintf("error: invalid arguments: %v", err)
	}
	if parsed.Command == "" {
		return "error: command is required"
	}

	timeoutSec := DefaultTimeout
	if parsed.Timeout != nil && *parsed.Timeout > 0 {
		timeoutSec = *parsed.Timeout
	}

	shellPath, err := platform.SelectShell(s.os)
	if err != nil {
		return fmt.Sprintf("error: cannot find shell: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	var flag string
	if s.os == platform.Windows {
		flag = "-Command"
	} else {
		flag = "-c"
	}

	cmd := exec.Command(shellPath, flag, parsed.Command)
	prepareShellCommand(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Sprintf("error: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err = <-done:
	case <-ctx.Done():
		killShellCommand(cmd)
		<-done
		return fmt.Sprintf("timeout %ds exceeded", timeoutSec)
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return fmt.Sprintf("error: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("exit_code: %d\n", exitCode))
	if stdout.Len() > 0 {
		sb.WriteString(fmt.Sprintf("stdout:\n%s\n", stdout.String()))
	}
	if stderr.Len() > 0 {
		sb.WriteString(fmt.Sprintf("stderr:\n%s\n", stderr.String()))
	}
	return sb.String()
}
