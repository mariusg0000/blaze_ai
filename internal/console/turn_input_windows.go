//go:build windows

// turn_input_windows.go — Windows console input polling for turn cancellation.
// Purpose: Detect ESC without waiting for a completed input line.
// Layer: console transport input boundary. Dependencies: os, sync, x/sys/windows.
package console

import (
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

const (
	waitObject0  = 0
	waitTimeout  = 258
	waitInfinite = 0xffffffff
)

// newTurnAbortWatcher starts a Windows console ESC watcher.
//
// WHAT: Reports ESC while an agent turn is running.
// WHY: Line-buffered Windows console input otherwise hides ESC until Enter.
// HOW: Polls the console handle and restores the original console mode on exit.
func newTurnAbortWatcher(input *os.File) (*turnAbortWatcher, error) {
	if input == nil {
		return nil, fmt.Errorf("cannot monitor ESC: stdin is unavailable")
	}
	handle := windows.Handle(input.Fd())
	var original uint32
	if err := windows.GetConsoleMode(handle, &original); err != nil {
		return nil, fmt.Errorf("cannot read Windows console mode for ESC: %w", err)
	}
	mode := original &^ (windows.ENABLE_LINE_INPUT | windows.ENABLE_ECHO_INPUT)
	if err := windows.SetConsoleMode(handle, mode); err != nil {
		return nil, fmt.Errorf("cannot enable character input for ESC: %w", err)
	}

	aborted := make(chan struct{}, 1)
	stop := make(chan struct{})
	done := make(chan struct{})
	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			close(stop)
			<-done
			_ = windows.SetConsoleMode(handle, original)
		})
	}

	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			status, err := windows.WaitForSingleObject(handle, 10)
			if err != nil || status == waitInfinite {
				return
			}
			if status == waitTimeout {
				continue
			}
			if status != waitObject0 {
				return
			}
			var chars [8]uint16
			var read uint32
			if err := windows.ReadConsole(handle, &chars[0], uint32(len(chars)), &read, nil); err != nil {
				return
			}
			for _, char := range chars[:read] {
				if char == 0x1b {
					aborted <- struct{}{}
					return
				}
			}
			time.Sleep(time.Millisecond)
		}
	}()

	return &turnAbortWatcher{aborted: aborted, stop: cleanup}, nil
}
