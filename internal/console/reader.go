// reader.go — raw-mode input reader for the terminal REPL.
// Handles Tab detection for mode cycling, Enter, Backspace, Ctrl-D,
// and cursor movement keys (left/right/home/end/delete).
// Uses term.MakeRaw to capture individual key presses.
// Layer: transport (console). Dependencies: golang.org/x/term.
package console

import (
	"bufio"
	"bytes"
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
// WHAT:  Reads user input with Tab (mode switch), Enter, Backspace, Ctrl-D,
//        and cursor movement (arrows, home, end, delete) support.
// WHY:   Tab key detection requires raw terminal mode per spec.
//        Cursor movement requires CSI escape sequence parsing.
// PARAMS: scanner — buffered line scanner for cooked-mode fallback;
//         isTTY — whether raw-mode key detection is active.
type Reader struct {
	scanner *bufio.Scanner
	isTTY   bool
	prompt  string
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

// SetPrompt stores the current prompt string for line redrawing.
//
// PARAMS: p — the prompt label printed before user input.
func (r *Reader) SetPrompt(p string) {
	r.prompt = p
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
// Backspace (delete char before cursor), and cursor movement keys
// (left/right arrows, home, end, delete).
// Returns the line, an event type, and error.
//
// WHAT:  Reads input with special key detection and inline editing.
// WHY:   Tab key cycles work modes; cursor keys need CSI parsing.
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
	os.Stdout.Write([]byte("\033[?2004h"))
	defer func() {
		os.Stdout.Write([]byte("\033[?2004l"))
		term.Restore(fd, oldState)
	}()

	var buf []byte
	pos := 0
	csiBuf := make([]byte, 0, 8)
	csiState := 0 // 0=normal, 1=saw ESC, 2=in CSI
	pasteMode := false

	for {
		b := make([]byte, 1)
		n, readErr := os.Stdin.Read(b)
		if readErr != nil {
			return "", "", readErr
		}
		if n == 0 {
			continue
		}

		ch := b[0]

		// --- CSI escape sequence state machine ---
		if csiState == 1 {
			if ch == '[' {
				csiState = 2
				csiBuf = csiBuf[:0]
				continue
			}
			// ESC followed by non-[. Reset and process ch normally below.
			csiState = 0
		} else if csiState == 2 {
			if ch == '~' {
				// Tilde-terminated CSI sequence
				csiState = 0
				params := string(csiBuf)
				switch params {
				case "3": // Delete key
					if pos < len(buf) {
						buf = append(buf[:pos], buf[pos+1:]...)
						r.redrawLine(buf, pos)
					}
				case "200": // Bracketed paste start
					pasteMode = true
				case "201": // Bracketed paste end
					pasteMode = false
				}
				continue
			}
			if ch >= 0x40 && ch <= 0x7E {
				// Letter-terminated CSI sequence
				csiState = 0
				switch ch {
				case 'A': // Up arrow
				case 'B': // Down arrow
				case 'C': // Right arrow
					if pos < len(buf) {
						pos++
						fmt.Fprint(os.Stdout, "\033[C")
					}
				case 'D': // Left arrow
					if pos > 0 {
						pos--
						fmt.Fprint(os.Stdout, "\033[D")
					}
				case 'H': // Home
					if pos > 0 {
						fmt.Fprintf(os.Stdout, "\033[%dD", pos)
						pos = 0
					}
				case 'F': // End
					if pos < len(buf) {
						fmt.Fprintf(os.Stdout, "\033[%dC", len(buf)-pos)
						pos = len(buf)
					}
				}
				continue
			}
			if ch >= 0x20 && ch <= 0x3F {
				// Parameter byte
				csiBuf = append(csiBuf, ch)
				continue
			}
			// Invalid CSI byte, reset
			csiState = 0
			continue
		}

		// --- Start of ESC sequence ---
		if ch == 0x1B {
			csiState = 1
			continue
		}

		// --- Normal character processing ---
		switch ch {
		case 0x09: // Tab
			if pasteMode {
				r.insertChar(&buf, &pos, '\t')
			} else {
				return "", "mode_switch", nil
			}
		case 0x0a, 0x0d: // Enter
			if pasteMode {
				r.insertChar(&buf, &pos, '\n')
			} else {
				fmt.Fprint(os.Stdout, "\r\n")
				return string(buf), "", nil
			}
		case 0x04: // Ctrl-D
			if len(buf) == 0 {
				return "", "", io.EOF
			}
		case 0x7f, 0x08: // Backspace
			if pos > 0 {
				buf = append(buf[:pos-1], buf[pos:]...)
				pos--
				r.redrawLine(buf, pos)
			}
		default:
			if ch >= 0x20 { // Printable
				r.insertChar(&buf, &pos, ch)
			}
		}
	}
}

// insertChar inserts a byte at the cursor position and updates the display.
//
// WHAT:  Inserts ch into buf at pos, advances pos, and redraws if needed.
// WHY:   Shared between printable chars, pasted tabs, and pasted newlines.
// PARAMS: buf — pointer to the input buffer; pos — pointer to cursor position; ch — byte to insert.
func (r *Reader) insertChar(buf *[]byte, pos *int, ch byte) {
	if *pos < len(*buf) {
		*buf = append(*buf, 0)
		copy((*buf)[*pos+1:], (*buf)[*pos:])
		(*buf)[*pos] = ch
		*pos++
		r.redrawLine(*buf, *pos)
	} else {
		*buf = append(*buf, ch)
		*pos++
		if ch == '\n' {
			fmt.Fprint(os.Stdout, "\r\n")
		} else {
			fmt.Fprint(os.Stdout, string(ch))
		}
	}
}

// redrawLine reprints the input line from column 0 and positions the cursor.
//
// WHAT:  Redraws the complete input line (prompt + buffer) and places
//        the cursor at the correct editing position.
// WHY:   Required after insert, delete, or any mutation in the middle
//        of the buffer. Appending at the end does not need a redraw.
// PARAMS: buf — the full input buffer; pos — desired cursor position (0..len(buf)).
func (r *Reader) redrawLine(buf []byte, pos int) {
	fmt.Fprint(os.Stdout, "\r")
	fmt.Fprint(os.Stdout, r.prompt)
	os.Stdout.Write(bytes.ReplaceAll(buf, []byte{'\n'}, []byte{'\r', '\n'}))
	fmt.Fprint(os.Stdout, "\033[K")
	if back := len(buf) - pos; back > 0 {
		fmt.Fprintf(os.Stdout, "\033[%dD", back)
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
