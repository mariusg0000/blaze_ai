// memory_test.go — tests for the memory file reader.
// Uses temp directories to avoid touching the real app home.
package memory

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReadFromExisting verifies that an existing file is read correctly.
func TestReadFromExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.md")
	want := "# Memory\n\n- fact one\n- fact two\n"
	if err := os.WriteFile(path, []byte(want), 0644); err != nil {
		t.Fatalf("cannot write test memory file: %v", err)
	}
	got, err := ReadFrom(path)
	if err != nil {
		t.Fatalf("ReadFrom() unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("ReadFrom() = %q, want %q", got, want)
	}
}

// TestReadFromMissing verifies that a missing file returns empty string and no error.
func TestReadFromMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.md")
	got, err := ReadFrom(path)
	if err != nil {
		t.Fatalf("ReadFrom() unexpected error for missing file: %v", err)
	}
	if got != "" {
		t.Errorf("ReadFrom() = %q, want empty string for missing file", got)
	}
}

// TestReadFromEmpty verifies that an empty file returns empty string.
func TestReadFromEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.md")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("cannot write empty test file: %v", err)
	}
	got, err := ReadFrom(path)
	if err != nil {
		t.Fatalf("ReadFrom() unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("ReadFrom() = %q, want empty string", got)
	}
}

// TestReadFromContent verifies that content with special characters is preserved.
func TestReadFromContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.md")
	want := "Unicode: émoji 🎉\nTabs\there\nLine1\nLine2\n"
	if err := os.WriteFile(path, []byte(want), 0644); err != nil {
		t.Fatalf("cannot write test file: %v", err)
	}
	got, err := ReadFrom(path)
	if err != nil {
		t.Fatalf("ReadFrom() unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("ReadFrom() = %q, want %q", got, want)
	}
}
