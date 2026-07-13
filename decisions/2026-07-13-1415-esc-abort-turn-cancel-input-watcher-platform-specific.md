# Session Decision Summary: ESC turn cancellation

Date: 2026-07-13 14:15

## Context

User requested ESC key instead of Ctrl+C to abort an active agent turn. Initial implementation used a separate goroutine reading from `os.File.Read`, but ESC leaked into the readline prompt as `^[`. Investigation revealed that `os.File.Read` did not reliably consume the byte from the raw terminal without leaving it buffered for readline.

## Changes Made

- `internal/console/console.go` — integrated `newTurnAbortWatcher` in `runAgentTurn`, added ESC to shortcut list, updated comments.
- `internal/console/turn_input.go` — shared `turnAbortWatcher` type with `aborted` event channel and `stop` cleanup.
- `internal/console/turn_input_unix.go` — Unix implementation using `term.MakeRaw`, `unix.FcntlInt(O_NONBLOCK)`, and `unix.Read` directly from the file descriptor.
- `internal/console/turn_input_windows.go` — Windows implementation using native console API (`GetConsoleMode`, `SetConsoleMode`, `WaitForSingleObject`, `ReadConsole`).

## Decisions And Rationale

- Use `unix.Read(fd, buf)` instead of `input.Read(buf)` on Unix. The `os.File.Read` path did not reliably consume the ESC byte from the Fd before readline polled it again, causing `^[` in the prompt. Direct `unix.Read` coupled with `F_SETFL O_NONBLOCK` ensures the byte is consumed synchronously before the polling goroutine yields.
- Use a buffered event channel (capacity 1) so the watcher goroutine never blocks on delivery and the stop-cleanup sequence is reliable.
- Keep SIGINT as a parallel abort path. Ctrl+C continues to work as before, ensuring compatibility for users who prefer it or when the terminal does not deliver raw ESC (e.g., some ssh-forwarded terminals).
- Cross-compile Windows with `go build` to validate the `go:build windows` gate at compile time. Tests are not runnable cross-platform but build errors are caught.
- Back up originals under `/home/marius/blazeai/backups/` before the fix iteration.
