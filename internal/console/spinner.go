// spinner.go — thinking spinner for the console transport.
// Displays an animated spinner on TTY while the LLM is processing.
// Erased cleanly when the first content or tool event arrives.
// On non-TTY, prints a static "thinking..." line.
// Layer: transport (console). Dependencies: none.
package console

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// spinnerFrames are the animation frames for the TTY spinner.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner displays an animated thinking indicator on TTY or a static line on non-TTY.
//
// WHAT:  Shows a spinner animation while waiting for LLM response.
// WHY:   Visual feedback that the agent is processing.
// PARAMS: out — output writer; isTTY — whether output is a terminal.
type Spinner struct {
	out    io.Writer
	isTTY  bool
	stop   chan struct{}
	done   chan struct{}
	mu     sync.Mutex
	active bool
}

// NewSpinner creates a Spinner.
//
// PARAMS: out — output writer; isTTY — whether output is a terminal.
// RETURNS: *Spinner — ready to start.
func NewSpinner(out io.Writer, isTTY bool) *Spinner {
	return &Spinner{out: out, isTTY: isTTY}
}

// Start begins the spinner animation. On non-TTY, prints "thinking...".
//
// WHAT:  Starts the visual waiting indicator.
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return
	}
	s.active = true
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	s.mu.Unlock()

	if !s.isTTY {
		fmt.Fprint(s.out, "thinking...\n")
		close(s.done)
		return
	}

	go func() {
		defer close(s.done)
		frame := 0
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.stop:
				// Erase the spinner line: \r + spaces + \r
				fmt.Fprintf(s.out, "\r\033[K")
				return
			case <-ticker.C:
				fmt.Fprintf(s.out, "\r%s thinking...", spinnerFrames[frame])
				frame = (frame + 1) % len(spinnerFrames)
			}
		}
	}()
}

// Stop erases the spinner and stops the animation.
//
// WHAT:  Cleans up the spinner display before content is printed.
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return
	}
	s.active = false
	close(s.stop)
	s.mu.Unlock()
	<-s.done
}
