// reader.go — raw-mode input reader for the terminal REPL.
// Handles Tab detection for mode cycling, Enter, Backspace, and Ctrl-D.
// Uses term.MakeRaw to capture individual key presses.
// Layer: transport (console). Dependencies: golang.org/x/term.
package console

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// isTerminal checks if a file is a terminal (character device).
//
// WHAT:  Detects whether output goes to a real terminal or is piped/redirected.
// WHY:   TTY detection controls colors, spinner, and visual separators.
// PARAMS: f — the file to check.
// RETURNS: bool — true if the file is a terminal.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

// Reader reads input from the terminal with raw-mode key detection.
//
// WHAT:  Reads user input with Tab (mode switch), Enter, Backspace, and Ctrl-D support.
// WHY:   Tab key detection requires raw terminal mode per spec.
// PARAMS: scanner — buffered line scanner for cooked-mode fallback (sudo, interactive prompts);
//
//	isTTY — whether raw-mode key detection is active.
type Reader struct {
	scanner *bufio.Scanner
	isTTY   bool
}

// NewReader creates a Reader from an io.Reader.
//
// PARAMS: r — the input reader; isTTY — whether input is from a terminal.
// RETURNS: *Reader — ready to read lines.
func NewReader(r io.Reader, isTTY bool) *Reader {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	return &Reader{scanner: scanner, isTTY: isTTY}
}

// ReadLine reads one line from the buffered scanner.
// Used by sudo approval and interactive prompts — not the main REPL prompt (which uses ReadEvent).
//
// WHAT:  Reads one line of cooked-mode input.
// RETURNS: string — the user input; error if reading fails or EOF.
func (r *Reader) ReadLine() (string, error) {
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return r.scanner.Text(), nil
}

// ReadEvent reads one input event from the console.
// Enters raw mode to detect Tab (mode switch), Enter (submit), Ctrl-D (EOF),
// and Backspace (delete). Returns the line, an event type, and error.
//
// WHAT:  Reads input with special key detection.
// WHY:   Tab key cycles work modes; raw mode is required to detect it.
// RETURNS: string — input line; string — event type ("", "mode_switch"); error — read error or EOF.
func (r *Reader) ReadEvent() (string, string, error) {
	if !r.isTTY {
		return "", "", fmt.Errorf("ReadEvent requires a terminal")
	}

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", "", fmt.Errorf("cannot enter raw terminal mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	var buf []byte
	for {
		b := make([]byte, 1)
		n, readErr := os.Stdin.Read(b)
		if readErr != nil {
			return "", "", readErr
		}
		if n == 0 {
			continue
		}

		switch b[0] {
		case 0x09: // Tab
			return "", "mode_switch", nil
		case 0x0a, 0x0d: // Enter
			fmt.Fprint(os.Stdout, "\r\n")
			return string(buf), "", nil
		case 0x04: // Ctrl-D
			if len(buf) == 0 {
				return "", "", io.EOF
			}
		case 0x7f, 0x08: // Backspace
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
				fmt.Fprint(os.Stdout, "\b \b")
			}
		default:
			if b[0] >= 0x20 { // Printable
				buf = append(buf, b[0])
				fmt.Fprint(os.Stdout, string(b[0]))
			}
		}
	}
}

// ReadHiddenInput reads one line from the terminal without echoing characters.
// Used for password entry. Backspace is supported but not echoed.
//
// WHAT:  Reads a single line of hidden input (password).
// RETURNS: string — the input text; error — read error, cancellation, or EOF.
func (r *Reader) ReadHiddenInput(prompt string) (string, error) {
	fmt.Fprint(os.Stdout, prompt)

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", fmt.Errorf("cannot enter raw terminal mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	var buf []byte
	for {
		b := make([]byte, 1)
		n, readErr := os.Stdin.Read(b)
		if readErr != nil {
			return "", readErr
		}
		if n == 0 {
			continue
		}

		switch b[0] {
		case 0x03: // Ctrl-C — cancel
			fmt.Fprint(os.Stdout, "\r\n")
			return "", fmt.Errorf("cancelled")
		case 0x0a, 0x0d: // Enter — submit
			fmt.Fprint(os.Stdout, "\r\n")
			return string(buf), nil
		case 0x04: // Ctrl-D
			if len(buf) == 0 {
				return "", io.EOF
			}
		case 0x7f, 0x08: // Backspace
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
			}
		default:
			if b[0] >= 0x20 {
				buf = append(buf, b[0])
				// Intentionally no echo.
			}
		}
	}
}
