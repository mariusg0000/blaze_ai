//go:build linux

// turn_output_linux.go — Linux terminal output preservation for turn cancellation.
// Purpose: Keep newline output aligned while the ESC watcher temporarily uses raw input.
// Layer: console transport input/output terminal boundary. Dependencies: x/sys/unix.
package console

import "golang.org/x/sys/unix"

// preserveTerminalOutput restores output translation disabled by term.MakeRaw.
//
// WHAT: Keeps carriage-return/newline processing active during raw input polling.
// HOW: Reads the raw termios state and enables the output flags required for aligned lines.
func preserveTerminalOutput(fd int) error {
	state, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return err
	}
	state.Oflag |= unix.OPOST | unix.ONLCR
	return unix.IoctlSetTermios(fd, unix.TCSETS, state)
}
