// spinner_test.go — tests for the thinking spinner.
package console

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestSpinnerStartStopNonTTY verifies non-TTY spinner prints "thinking..." and stops.
func TestSpinnerStartStopNonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	s := NewSpinner(out, false)
	s.Start()
	s.Stop()
	if !strings.Contains(out.String(), "thinking...") {
		t.Errorf("output = %q, want 'thinking...'", out.String())
	}
}

// TestSpinnerStartStopTTY verifies TTY spinner starts and stops without deadlock.
func TestSpinnerStartStopTTY(t *testing.T) {
	out := &bytes.Buffer{}
	s := NewSpinner(out, true)
	s.Start()
	time.Sleep(200 * time.Millisecond)
	s.Stop()
	// After stop, the spinner should have been erased.
	// We just verify no deadlock occurred.
}

// TestSpinnerStopWhenNotStarted verifies Stop is safe when never started.
func TestSpinnerStopWhenNotStarted(t *testing.T) {
	out := &bytes.Buffer{}
	s := NewSpinner(out, false)
	s.Stop()
}

// TestSpinnerDoubleStart verifies starting twice is safe.
func TestSpinnerDoubleStart(t *testing.T) {
	out := &bytes.Buffer{}
	s := NewSpinner(out, false)
	s.Start()
	s.Start()
	s.Stop()
}

// TestSpinnerDoubleStop verifies stopping twice is safe.
func TestSpinnerDoubleStop(t *testing.T) {
	out := &bytes.Buffer{}
	s := NewSpinner(out, false)
	s.Start()
	s.Stop()
	s.Stop()
}
