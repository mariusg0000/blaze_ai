//go:build windows

// reader_input_windows.go — context-aware Windows console byte polling.
// Layer: console input boundary. Dependencies: context, os, windows.
package console

import (
	"context"
	"os"

	"golang.org/x/sys/windows"
)

// readTerminalByte polls one console input byte while honoring cancellation.
func readTerminalByte(ctx context.Context, tty *os.File) (byte, error) {
	h := windows.Handle(tty.Fd())
	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		status, err := windows.WaitForSingleObject(h, 10)
		if err != nil {
			return 0, err
		}
		if status == waitTimeout {
			continue
		}
		var chars [1]uint16
		var read uint32
		if err := windows.ReadConsole(h, &chars[0], 1, &read, nil); err != nil {
			return 0, err
		}
		if read == 1 {
			return byte(chars[0]), nil
		}
	}
}
