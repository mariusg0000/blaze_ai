// shell_process_unix.go — Unix-specific shell process group handling.
// Starts shell commands in their own process group and kills the full group on timeout
// so background children do not keep stdout/stderr pipes open forever.
// Layer: tool execution. Dependencies: standard library process control.

//go:build linux || darwin

package tools

import (
	"os/exec"
	"syscall"
)

// prepareShellCommand isolates the shell command in its own process group.
//
// WHAT:  Configures the command so timeout cleanup can kill all child processes.
// WHY:   Background children inherit stdio and can otherwise keep Wait blocked forever.
// PARAMS: cmd — command to configure before Start.
func prepareShellCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killShellCommand terminates the shell command and its children.
//
// WHAT:  Sends SIGKILL to the whole process group when available.
// WHY:   Timeout must clean up background descendants, not just the shell parent.
// PARAMS: cmd — started command to terminate.
func killShellCommand(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	_ = cmd.Process.Kill()
}
