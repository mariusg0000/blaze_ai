// reader.go — auxiliary console input for non-REPL prompts.
// The main REPL uses reeflective/readline; this type retains history helpers and hidden password input.
// Layer: console transport. Dependencies: bufio, os, golang.org/x/term.
package console

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// isTerminal reports whether a file is a character-device terminal.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// Reader reads auxiliary console input outside the main readline REPL.
type Reader struct {
	scanner *bufio.Scanner
	isTTY   bool

	history       []string
	historyPos    int
	historyDraft  string
	historyActive bool
}

// NewReader creates a Reader from an io.Reader.
//
// WHAT:  Creates a buffered reader for sudo confirmation and compatibility helpers.
// WHY:   Auxiliary prompts still need line and hidden-password input outside the main REPL.
// PARAMS: r — input source; isTTY — whether the source is a terminal.
// RETURNS: configured Reader.
func NewReader(r io.Reader, isTTY bool) *Reader {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	return &Reader{scanner: scanner, isTTY: isTTY}
}

// SetPrompt is retained for compatibility with auxiliary callers; readline owns the REPL prompt.
func (r *Reader) SetPrompt(string) {}

// AddHistory stores a submitted non-empty input for compatibility and tests.
// Consecutive duplicates are ignored.
func (r *Reader) AddHistory(input string) {
	if input == "" || (len(r.history) > 0 && r.history[len(r.history)-1] == input) {
		return
	}
	r.history = append(r.history, input)
	r.historyPos = len(r.history)
}

// History returns a copy of the compatibility history.
func (r *Reader) History() []string { return append([]string(nil), r.history...) }

// resetHistoryNavigation resets compatibility history navigation state.
func (r *Reader) resetHistoryNavigation() {
	r.historyPos = len(r.history)
	r.historyDraft = ""
	r.historyActive = false
}

// ReadLine reads one cooked-mode line for auxiliary prompts.
//
// WHAT:  Reads one line from the buffered scanner.
// RETURNS: line text, or an input error/EOF.
func (r *Reader) ReadLine() (string, error) {
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return r.scanner.Text(), nil
}

// ReadApproval reads a visible one-line confirmation from the controlling terminal.
//
// WHAT: Opens /dev/tty and reads Y/N input byte by byte in raw mode.
// WHY: stdin belongs to the readline lifecycle and may not be the controlling terminal.
// RETURNS: submitted line, or an explicit terminal/input error.
func (r *Reader) ReadApproval(ctx context.Context) (string, error) {
	tty, err := openApprovalTTY()
	if err != nil {
		return "", err
	}
	defer tty.Close()
	return readTerminalLine(ctx, tty, true)
}

// ReadEvent is no longer used by the main REPL, which is managed by readline.
// It fails explicitly instead of silently reactivating the removed raw parser.
func (r *Reader) ReadEvent() (string, string, error) {
	return "", "", fmt.Errorf("terminal input is managed by readline; raw console reader removed")
}

// navigateHistory updates compatibility history state for legacy callers and tests.
func (r *Reader) navigateHistory(buf *[]byte, pos *int, older bool) {
	if len(r.history) == 0 {
		return
	}
	if !r.historyActive {
		r.historyDraft = string(*buf)
		r.historyPos = len(r.history)
		r.historyActive = true
	}
	if older {
		if r.historyPos > 0 {
			r.historyPos--
		}
	} else if r.historyPos < len(r.history)-1 {
		r.historyPos++
	} else {
		*buf = []byte(r.historyDraft)
		*pos = len(*buf)
		r.resetHistoryNavigation()
		return
	}
	*buf = []byte(r.history[r.historyPos])
	*pos = len(*buf)
}

// deleteBeforeCursor removes one byte before a compatibility cursor position.
func deleteBeforeCursor(buf []byte, pos int) ([]byte, int) {
	if pos <= 0 || pos > len(buf) {
		return buf, pos
	}
	copy(buf[pos-1:], buf[pos:])
	return buf[:len(buf)-1], pos - 1
}

// ReadHiddenInput reads a password without echoing characters.
//
// WHAT:  Reads a hidden line for sudo approval.
// WHY:   Password input is an auxiliary prompt and must not echo secrets.
// PARAMS: prompt — label printed before the password.
// RETURNS: password, or cancellation/input error.
func (r *Reader) ReadHiddenInput(ctx context.Context, prompt string) (string, error) {
	fmt.Fprint(os.Stdout, prompt)
	tty, err := openApprovalTTY()
	if err != nil {
		return "", err
	}
	defer tty.Close()
	return readTerminalLine(ctx, tty, false)
}

// openApprovalTTY opens the controlling terminal independently of stdin/readline.
func openApprovalTTY() (*os.File, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot open controlling terminal /dev/tty: %w", err)
	}
	return tty, nil
}

// readTerminalLine reads one raw terminal line and optionally echoes printable input.
func readTerminalLine(ctx context.Context, tty *os.File, echo bool) (string, error) {
	fd := int(tty.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", fmt.Errorf("cannot enter raw terminal mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	var buf []byte
	for {
		one, err := readTerminalByte(ctx, tty)
		if err != nil {
			return "", err
		}
		switch one {
		case 0x03, 0x1b:
			fmt.Fprint(os.Stdout, "\r\n")
			return "", context.Canceled
		case 0x0a, 0x0d:
			fmt.Fprint(os.Stdout, "\r\n")
			return string(buf), nil
		case 0x04:
			if len(buf) == 0 {
				return "", io.EOF
			}
		case 0x7f, 0x08:
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
				if echo {
					fmt.Fprint(os.Stdout, "\b \b")
				}
			}
		default:
			if one >= 0x20 {
				buf = append(buf, one)
				if echo {
					fmt.Fprintf(os.Stdout, "%c", one)
				}
			}
		}
	}
}

var _ = (*Reader).resetHistoryNavigation
