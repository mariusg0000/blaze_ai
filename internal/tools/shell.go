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
	"sync"
	"time"

	"blazeai/internal/platform"
)

// MaxShellOutputBytes is the absolute cap for combined stdout and stderr returned by shell.
const MaxShellOutputBytes = 150 * 1024

// ShellArgs are the arguments for the shell tool.
//
// WHAT:  Parsed arguments from the LLM tool call.
// PARAMS: Command — the shell command string; Timeout — optional timeout in seconds (default 60).
type ShellArgs struct {
	Command string `json:"command"`
	Timeout *int   `json:"timeout,omitempty"`
	Purpose string `json:"purpose,omitempty"`
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
	if err != nil {
		return ""
	}
	if strings.TrimSpace(parsed.Purpose) != "" {
		return strings.TrimSpace(parsed.Purpose)
	}
	if parsed.Command == "" {
		return ""
	}
	return truncateDisplay(parsed.Command, 80)
}

// Description returns the human-readable description for the LLM.
func (s *ShellTool) Description() string {
	return "Execute a shell command on the host. Returns stdout, stderr, and exit_code. Avoid broad recursive commands. If the target may be large, refine the command or read it in sequential chunks below 150 kB."
}

// Parameters returns the JSON schema for the tool's parameters.
func (s *ShellTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"purpose": {
				"type": "string",
				"description": "A concise 1-2 sentence summary of this shell call. State what it does, why it is needed, the exact command it uses, and the file or files or folders it inspects or changes when relevant."
			},
			"command": {
				"type": "string",
				"description": "The shell command to execute. Avoid broad recursive commands. If the target may be large, refine the path or pattern, or read it in sequential chunks below 150 kB."
			},
			"timeout": {
				"type": "integer",
				"description": "Optional timeout in seconds. Default: 60."
			}
		},
		"required": ["purpose", "command"]
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
// PARAMS: ctx — turn cancellation context; args — raw JSON with command and optional timeout.
// RETURNS: string — formatted stdout/stderr/exit_code, or "timeout <N>s exceeded" on timeout.
func (s *ShellTool) Execute(ctx context.Context, args json.RawMessage) string {
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

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
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
	limiter := &shellOutputLimiter{maxBytes: MaxShellOutputBytes}
	cmd.Stdout = &limitedStreamWriter{buffer: &stdout, limiter: limiter}
	cmd.Stderr = &limitedStreamWriter{buffer: &stderr, limiter: limiter}

	if err := cmd.Start(); err != nil {
		return fmt.Sprintf("error: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	limiter.onExceeded = func() {
		killShellCommand(cmd)
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
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Sprintf("timeout %ds exceeded", timeoutSec)
		}
		return formatAbortedToolOutput(stdout.String(), stderr.String())
	}

	if limiter.exceeded() {
		return formatShellOutputExceeded(limiter.stdoutBytes(), limiter.stderrBytes())
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

// shellOutputLimiter tracks combined stdout and stderr and stops accepting bytes after a hard cap.
type shellOutputLimiter struct {
	mu         sync.Mutex
	maxBytes   int
	totalBytes int
	stdoutSize int
	stderrSize int
	hitLimit   bool
	onExceeded func()
}

// limitedStreamWriter writes to one stream buffer while enforcing the shared shell output cap.
type limitedStreamWriter struct {
	buffer   *bytes.Buffer
	limiter  *shellOutputLimiter
	isStderr bool
}

// Write appends only while within the shared cap. Once exceeded, the process is killed and
// subsequent bytes are discarded from the conversation output.
func (w *limitedStreamWriter) Write(p []byte) (int, error) {
	allowed, triggered := w.limiter.reserve(len(p), w.isStderr)
	if allowed > 0 {
		_, _ = w.buffer.Write(p[:allowed])
	}
	if triggered {
		w.limiter.kill()
	}
	return len(p), nil
}

// reserve claims up to n bytes from the shared budget and records whether the cap was hit.
func (l *shellOutputLimiter) reserve(n int, isStderr bool) (allowed int, triggered bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.hitLimit {
		return 0, false
	}
	remaining := l.maxBytes - l.totalBytes
	if remaining <= 0 {
		l.hitLimit = true
		return 0, true
	}
	allowed = n
	if allowed > remaining {
		allowed = remaining
	}
	l.totalBytes += allowed
	if isStderr {
		l.stderrSize += allowed
	} else {
		l.stdoutSize += allowed
	}
	if allowed < n {
		l.hitLimit = true
		triggered = true
	}
	return allowed, triggered
}

// exceeded reports whether the output cap was reached.
func (l *shellOutputLimiter) exceeded() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.hitLimit
}

// stdoutBytes returns captured stdout bytes before the cap stopped the command.
func (l *shellOutputLimiter) stdoutBytes() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.stdoutSize
}

// stderrBytes returns captured stderr bytes before the cap stopped the command.
func (l *shellOutputLimiter) stderrBytes() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.stderrSize
}

// kill stops the process the first time the limit is hit.
func (l *shellOutputLimiter) kill() {
	l.mu.Lock()
	fn := l.onExceeded
	l.onExceeded = nil
	l.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// formatShellOutputExceeded returns the explicit guidance message sent back to the LLM.
func formatShellOutputExceeded(stdoutBytes, stderrBytes int) string {
	return fmt.Sprintf(
		"error: shell output exceeded the 150 kB limit and was stopped. stdout_bytes=%d stderr_bytes=%d limit_bytes=%d. Refine the command to a narrower path, pattern, or depth, or read the target in sequential chunks below 150 kB.",
		stdoutBytes,
		stderrBytes,
		MaxShellOutputBytes,
	)
}

// formatAbortedToolOutput returns the partial output captured before user cancellation.
func formatAbortedToolOutput(stdout, stderr string) string {
	var sb strings.Builder
	sb.WriteString("aborted by user")
	if stdout != "" {
		sb.WriteString("\nstdout:\n")
		sb.WriteString(stdout)
	}
	if stderr != "" {
		sb.WriteString("\nstderr:\n")
		sb.WriteString(stderr)
	}
	return sb.String()
}
