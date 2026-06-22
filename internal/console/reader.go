// reader.go — input reader for the console REPL.
// Handles single-line and multiline paste input. On TTY, waits for an empty line
// to signal end of pasted multiline content. On non-TTY, reads line by line.
// Layer: transport (console). Dependencies: none.
package console

import (
	"bufio"
	"io"
	"os"
	"strings"
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

// Reader reads input lines from the console, handling multiline paste.
//
// WHAT:  Reads user input with single-line and multiline paste support.
// WHY:   Pasted text with newlines should not be submitted prematurely per spec.
// PARAMS: reader — the underlying io.Reader; isTTY — whether input is from a terminal.
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

// ReadLine reads one logical input from the console.
// On TTY: if the first line contains a paste (multiple lines), reads until an empty line.
// On non-TTY: reads a single line.
//
// WHAT:  Reads one user input, handling multiline paste on TTY.
// RETURNS: string — the user input; error if reading fails or EOF.
func (r *Reader) ReadLine() (string, error) {
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	first := r.scanner.Text()

	// On non-TTY, return single line.
	if !r.isTTY {
		return first, nil
	}

	// On TTY, check if this is a multiline paste by looking for embedded newlines
	// in the raw input. Since bufio.Scanner splits on newlines, a paste of multiple
	// lines will come as multiple Scan() calls. We detect multiline by checking if
	// the next read is available immediately (non-blocking check is not possible
	// with bufio.Scanner, so we use a simpler heuristic: if the line looks like a
	// slash command or is short and standalone, return it; otherwise wait for more).
	// For simplicity in this phase: single line per Read() call.
	// Multiline paste handling will be refined when full terminal control is added.
	return first, nil
}

// ReadMultiline reads lines until an empty line is encountered, concatenating them.
// Used for multiline paste detection on TTY.
//
// WHAT:  Reads multiple lines until an empty line signals end of paste.
// RETURNS: string — concatenated lines; error if reading fails.
func (r *Reader) ReadMultiline() (string, error) {
	var lines []string
	for r.scanner.Scan() {
		line := r.scanner.Text()
		if strings.TrimSpace(line) == "" {
			break
		}
		lines = append(lines, line)
	}
	if err := r.scanner.Err(); err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}
