//go:build !windows && !linux

// turn_output_unix_other.go — non-Linux Unix terminal output compatibility.
// Purpose: Provide the shared output-preservation hook for Unix platforms without Linux termios constants.
// Layer: console transport input/output terminal boundary. Dependencies: none.
package console

// preserveTerminalOutput leaves output handling unchanged on non-Linux Unix systems.
//
// WHAT: Keeps the shared watcher buildable on other Unix platforms.
// HOW: The platform terminal implementation owns any required output translation.
func preserveTerminalOutput(fd int) error {
	return nil
}
