//go:build !windows

// reader_input_unix.go — context-aware terminal byte polling.
// Layer: console input boundary. Dependencies: context, os, unix.
package console

import (
	"context"
	"os"

	"golang.org/x/sys/unix"
)

// readTerminalByte polls one raw terminal byte while honoring cancellation.
func readTerminalByte(ctx context.Context, tty *os.File) (byte, error) {
	fd := int(tty.Fd())
	flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if err != nil {
		return 0, err
	}
	if _, err = unix.FcntlInt(uintptr(fd), unix.F_SETFL, flags|unix.O_NONBLOCK); err != nil {
		return 0, err
	}
	defer unix.FcntlInt(uintptr(fd), unix.F_SETFL, flags)
	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		_, err := unix.Poll(fds, 10)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return 0, err
		}
		if fds[0].Revents&unix.POLLIN != 0 {
			var one [1]byte
			n, err := unix.Read(fd, one[:])
			if n == 1 {
				return one[0], nil
			}
			if err != nil && err != unix.EAGAIN && err != unix.EWOULDBLOCK {
				return 0, err
			}
		}
	}
}
