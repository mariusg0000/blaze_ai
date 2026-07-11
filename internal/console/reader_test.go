// reader_test.go — tests for auxiliary console Reader behavior.
// Verifies cooked line input remains available after moving the main REPL to readline.
// Layer: console transport tests. Dependencies: internal/console Reader.
package console

import (
	"strings"
	"testing"
)

// TestReaderReadLine verifies auxiliary prompts still read a complete cooked line.
func TestAuxReaderReadLine(t *testing.T) {
	reader := NewReader(strings.NewReader("yes\n"), false)
	got, err := reader.ReadLine()
	if err != nil {
		t.Fatalf("ReadLine() error = %v", err)
	}
	if got != "yes" {
		t.Fatalf("ReadLine() = %q, want %q", got, "yes")
	}
}
