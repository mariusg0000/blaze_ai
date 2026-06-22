// shell_process_windows.go — Windows-specific shell timeout cleanup.
// Uses direct process kill as a best-effort fallback when process groups are not configured
// the same way as Unix in this implementation.
// Layer: tool execution. Dependencies: standard library process control.

//go:build windows

package tools

import "os/exec"

// prepareShellCommand leaves the command unchanged on Windows.
//
// WHAT:  No-op placeholder for platform-specific command setup.
// PARAMS: cmd — command to configure before Start.
func prepareShellCommand(cmd *exec.Cmd) {
	_ = cmd
}

// killShellCommand terminates the shell parent process on Windows.
//
// WHAT:  Best-effort timeout cleanup for the started shell process.
// WHY:   The shell tool still needs to stop the command on timeout on Windows.
// PARAMS: cmd — started command to terminate.
func killShellCommand(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
