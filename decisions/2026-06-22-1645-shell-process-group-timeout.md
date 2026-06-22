# Session Decision Summary: Kill shell process group on timeout

Date: 2026-06-22 16:45
Base commit: 39c2922

## Context
An 15s timeout shell command (`python app.py & sleep 2; curl ...`) never returned. The session ended
with `closed_cleanly: false` and a hung tool call. The root cause was that `exec.CommandContext` kills
only the shell parent, not its background children. If a child holds stdout/stderr, `cmd.Run()` stays
blocked past the timeout.

## Fix
- **shell.go**: Replaced `CommandContext`+`Run` with `Command`+`Start`+goroutine `Wait`. On `ctx.Done()`,
  explicitly calls `killShellCommand` then drains the Wait goroutine before returning the timeout message.
- **shell_process_unix.go**: Sets `Setpgid: true` before Start, kills with `syscall.Kill(-pid, SIGKILL)`.
- **shell_process_windows.go**: Falls back to `Process.Kill()`.
- **shell_test.go**: Added regression test that runs `sleep 30 & sleep 30` with 1s timeout and asserts
  it returns immediately rather than hanging.

## Files Changed
- `internal/tools/shell.go`: process group management, Start/Wait/drain pattern
- `internal/tools/shell_process_unix.go`: Unix Setpgid + SIGKILL for full process group
- `internal/tools/shell_process_windows.go`: Windows Process.Kill fallback
- `internal/tools/shell_test.go`: regression test for background children on timeout
