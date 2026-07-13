//go:build !windows

// turn_input_unix.go — Unix terminal input polling for turn cancellation.
// Purpose: Detect ESC without waiting for a completed input line.
// Layer: console transport input boundary. Dependencies: os, term, unix.
package console

import (
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// newTurnAbortWatcher starts a Unix raw-mode ESC watcher for a terminal stdin.
//
// WHAT: Reports ESC while an agent turn is running.
// WHY: Cooked terminal input does not deliver ESC until Enter is pressed.
// HOW: Uses raw mode plus nonblocking reads, then restores both terminal state and file flags.
func newTurnAbortWatcher(input *os.File) (*turnAbortWatcher, error) {
	if input == nil {
		return nil, fmt.Errorf("cannot monitor ESC: stdin is unavailable")
	}
	fd := int(input.Fd())
	if !term.IsTerminal(fd) {
		return nil, fmt.Errorf("cannot monitor ESC: stdin is not a terminal")
	}

	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("cannot enable raw terminal input for ESC: %w", err)
	}
	flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if err != nil {
		_ = term.Restore(fd, state)
		return nil, fmt.Errorf("cannot inspect terminal input flags: %w", err)
	}
	if _, err := unix.FcntlInt(uintptr(fd), unix.F_SETFL, flags|unix.O_NONBLOCK); err != nil {
		_ = term.Restore(fd, state)
		return nil, fmt.Errorf("cannot enable nonblocking terminal input: %w", err)
	}

	aborted := make(chan struct{}, 1)
	stop := make(chan struct{})
	done := make(chan struct{})
	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			close(stop)
			<-done
			_, _ = unix.FcntlInt(uintptr(fd), unix.F_SETFL, flags)
			_ = term.Restore(fd, state)
		})
	}

	go func() {
		defer close(done)
		buf := make([]byte, 32)
		for {
			select {
			case <-stop:
				return
			default:
			}
			n, readErr := unix.Read(fd, buf)
			if n > 0 {
				for _, b := range buf[:n] {
					if b == 0x1b {
						aborted <- struct{}{}
						return
					}
				}
			}
			if readErr != nil && readErr != unix.EAGAIN && readErr != unix.EWOULDBLOCK {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	return &turnAbortWatcher{aborted: aborted, stop: cleanup}, nil
}
