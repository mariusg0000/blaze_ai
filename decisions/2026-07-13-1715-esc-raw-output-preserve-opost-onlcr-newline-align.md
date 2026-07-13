# Session Decision Summary: ESC raw terminal output alignment

Date: 2026-07-13 17:15

## Context

After adding ESC as a turn-cancellation key, console output lines drifted horizontally — each new line started farther right as if centered. The user reported that text was not aligned left under the previous line.

## Changes Made

- `internal/console/turn_input_unix.go` — added call to `preserveTerminalOutput(fd)` immediately after `term.MakeRaw(fd)` inside `newTurnAbortWatcher`.
- `internal/console/turn_output_linux.go` — new Linux-specific file that reads the raw termios state and re-enables `OPOST | ONLCR` on the output flags, keeping `\n` map to `\r\n`.
- `internal/console/turn_output_unix_other.go` — new stub for non-Linux Unix platforms that returns nil, preserving existing behavior.

## Decisions And Rationale

- Use `preserveTerminalOutput` instead of a custom `MakeRaw` variant because `term.MakeRaw` is a standard call that also handles signal and input flags correctly; only the output flags need correction.
- Keep the fix Linux-only because `unix.OPOST` and `unix.ONLCR` are Linux `termios` constants; other Unix platforms (macOS, BSD) define different bit values and should use their own platform file if needed.
- Recovery still restores the original terminal state (captured by `MakeRaw`) so the output fix applies only during ESC monitoring, not after the watcher is stopped.
- Backup saved under `/home/marius/blazeai/backups/esc-output-fix/` before the change.
