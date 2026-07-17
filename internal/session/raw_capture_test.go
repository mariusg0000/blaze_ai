// raw_capture_test.go — tests for stream-scoped raw capture.
// Layer: session storage tests. Dependencies: testing and temporary files.
package session

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRawCaptureWritesExactLines verifies buffered payload persistence and truncation.
func TestRawCaptureWritesExactLines(t *testing.T) {
	folder := t.TempDir()
	capture, err := NewRawCapture(folder, "llm-raw.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := capture.Append([]byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := capture.Append(nil); err != nil {
		t.Fatal(err)
	}
	if err := capture.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(folder, "llm-raw.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "{\"a\":1}\n\n"; got != want {
		t.Fatalf("capture = %q, want %q", got, want)
	}

	capture, err = NewRawCapture(folder, "llm-raw.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := capture.Append([]byte("next")); err != nil {
		t.Fatal(err)
	}
	if err := capture.Close(); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(folder, "llm-raw.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "next\n"; got != want {
		t.Fatalf("capture = %q, want %q", got, want)
	}
}
