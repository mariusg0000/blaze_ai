// helper_exec.go — bounded execution for direct read-only helper wrappers.
// Runs external helpers without a shell and formats their output for the model.
// Layer: tool execution. Dependencies: standard-library process and context APIs.
package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// executeHelper runs fd or rg directly with bounded output and timeout.
// WHAT: Executes a discovered helper without shell interpretation.
// HOW: Uses a context deadline and a shared byte limiter that terminates the process.
func executeHelper(ctx context.Context, helperPath string, args []string, workDir string, maxBytes int, maxLines *int, timeout *int) string {
	if ctx == nil {
		ctx = context.Background()
	}
	timeoutSeconds := DefaultTimeout
	if timeout != nil && *timeout > 0 {
		timeoutSeconds = *timeout
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, helperPath, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	var stdout, stderr bytes.Buffer
	limiter := &helperOutputLimiter{maxBytes: maxBytes}
	cmd.Stdout = &helperLimitedWriter{buffer: &stdout, limiter: limiter}
	cmd.Stderr = &helperLimitedWriter{buffer: &stderr, limiter: limiter}

	if err := cmd.Start(); err != nil {
		return fmt.Sprintf("error: cannot start helper %q: %v", helperPath, err)
	}
	limiter.onExceeded = func() { _ = cmd.Process.Kill() }
	err := cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Sprintf("error: helper %q timed out after %ds", helperPath, timeoutSeconds)
	}
	if limiter.exceeded() {
		return fmt.Sprintf("error: helper %q output exceeded %d bytes", helperPath, maxBytes)
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return fmt.Sprintf("error: helper %q failed: %v", helperPath, err)
		}
	}
	output := strings.TrimRight(stdout.String(), "\n")
	if maxLines != nil && *maxLines > 0 {
		output = limitOutputLines(output, *maxLines)
	}
	var result strings.Builder
	fmt.Fprintf(&result, "exit_code: %d\n", exitCode)
	if output != "" {
		result.WriteString("stdout:\n")
		result.WriteString(output)
		result.WriteByte('\n')
	}
	if text := strings.TrimRight(stderr.String(), "\n"); text != "" {
		result.WriteString("stderr:\n")
		result.WriteString(text)
		result.WriteByte('\n')
	}
	return strings.TrimRight(result.String(), "\n")
}

// limitOutputLines keeps the first requested number of helper result lines.
func limitOutputLines(output string, maxLines int) string {
	lines := strings.Split(output, "\n")
	if len(lines) <= maxLines {
		return output
	}
	return strings.Join(lines[:maxLines], "\n") + fmt.Sprintf("\n[output truncated after %d lines]", maxLines)
}

// helperOutputLimiter bounds combined stdout and stderr from a helper process.
type helperOutputLimiter struct {
	mu         sync.Mutex
	maxBytes   int
	totalBytes int
	hitLimit   bool
	onExceeded func()
}

// helperLimitedWriter captures helper output until the shared byte cap is reached.
type helperLimitedWriter struct {
	buffer  *bytes.Buffer
	limiter *helperOutputLimiter
}

// Write captures as much output as remains within the configured cap.
func (w *helperLimitedWriter) Write(p []byte) (int, error) {
	w.limiter.mu.Lock()
	if w.limiter.hitLimit {
		w.limiter.mu.Unlock()
		return len(p), nil
	}
	remaining := w.limiter.maxBytes - w.limiter.totalBytes
	allowed := len(p)
	if allowed > remaining {
		allowed = remaining
	}
	w.limiter.totalBytes += allowed
	limited := allowed < len(p)
	if allowed < len(p) {
		w.limiter.hitLimit = true
	}
	onExceeded := w.limiter.onExceeded
	w.limiter.mu.Unlock()
	if allowed > 0 {
		_, _ = w.buffer.Write(p[:allowed])
	}
	if limited && onExceeded != nil {
		onExceeded()
	}
	return len(p), nil
}

// exceeded reports whether the helper exceeded its output budget.
func (l *helperOutputLimiter) exceeded() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.hitLimit
}
