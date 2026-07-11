// reader_test.go — focused tests for atomic bracketed-paste buffer handling.
// Verifies that pasted newlines are preserved internally and trailing clipboard line endings are removed.
// Layer: console transport tests. Dependencies: internal/console reader helpers.
package console

import "testing"

// TestNormalizePastePreservesInternalNewlines verifies paste cleanup keeps log/list structure.
func TestNormalizePastePreservesInternalNewlines(t *testing.T) {
	got := string(normalizePaste([]byte("before\nlog line\nitem\r\n")))
	want := "before\nlog line\nitem"
	if got != want {
		t.Fatalf("normalizePaste() = %q, want %q", got, want)
	}
}

// TestInsertBytesAppendsAfterPastedText verifies completion text can follow a multiline paste.
func TestInsertBytesAppendsAfterPastedText(t *testing.T) {
	got, pos := insertBytes([]byte("prefix: "), len("prefix: "), []byte("a\nb\n"))
	got, pos = insertBytes(got, pos, []byte(" completed"))
	want := "prefix: a\nb\n completed"
	if string(got) != want {
		t.Fatalf("insertBytes() = %q, want %q", got, want)
	}
	if pos != len(got) {
		t.Fatalf("cursor position = %d, want %d", pos, len(got))
	}
}
