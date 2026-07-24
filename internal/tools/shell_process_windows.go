// shell_process_windows.go — Windows-specific shell timeout cleanup.
// Terminates the shell process tree when process groups are not configured the same way as Unix.
// Layer: tool execution. Dependencies: standard library process control.

//go:build windows

package tools

import (
	"os/exec"
	"strconv"
)

// prepareShellCommand leaves the command unchanged on Windows.
//
// WHAT:  No-op placeholder for platform-specific command setup.
// PARAMS: cmd — command to configure before Start.
func prepareShellCommand(cmd *exec.Cmd) {
	_ = cmd
}

// killShellCommand terminates the shell process tree on Windows.
//
// WHAT:  Uses taskkill to terminate the started shell and all descendants.
// WHY:   Timeout cleanup must not leave background descendants holding shell pipes open.
// PARAMS: cmd — started command to terminate.
func killShellCommand(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
	_ = cmd.Process.Kill()
}
