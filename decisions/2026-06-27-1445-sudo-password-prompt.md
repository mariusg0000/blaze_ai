# Session Decision Summary: sudo-password-prompt

Date: 2026-06-27 14:45
Base commit: d06d77f

## Context
When the agent runs `sudo systemctl restart blazeai-telegram@nas`, the shell tool captures stderr and blocks forever waiting for a password. The user never sees the password prompt, and the tool times out at 60s with no usable result.

Previous workaround: kill the process and restart manually, bypassing systemd. The user wants a proper solution: prompt the user for approval and password at runtime.

## Changes Made
- Added `RequestSudoApproval(command string) (approved bool, password string)` to the `Handler` interface in `internal/runtime/runtime.go:53`.
- In `RunTurn` tool call loop, before shell execution: detect `sudo` in command, call `RequestSudoApproval`, pass approved password via `BLAZE_SUDO_PASSWORD` env var (cleared each iteration to prevent cross-call leaks, cleaned by executeShell after reading).
- Added `containsSudo(command string) bool` in `runtime.go` that detects `sudo ` at command boundaries (start, after pipe, `;`, `&&`, `||`).
- `Console.RequestSudoApproval` in `console.go:612`: prints sudo command, asks `[y/N]`, delegates to `Reader.ReadHiddenInput`.
- `Reader.ReadHiddenInput` in `reader.go:119`: on TTY, enters raw mode, reads byte-by-byte without echo, supports Backspace and Ctrl-C. On non-TTY, falls back to `ReadLine` (echo visible — acceptable for tests/pipes).
- `executeShell` in `shell.go:148`: checks `BLAZE_SUDO_PASSWORD`, inserts `-S` flag (only if not already present), pipes password via `cmd.Stdin`.
- Telegram handler returns `false` for `RequestSudoApproval` — sudo commands are not allowed via the Telegram bridge for security.

## Decisions And Rationale
- **Env var over context**: tools package can't import runtime (circular). Env var is simple, cleaned immediately after read by executeShell, and each iteration starts with `os.Unsetenv` to prevent cross-call leaks.
- **`sudo -S` insertion**: sudo without `-S` reads from TTY (not stdin). Adding `-S` forces sudo to read the password from stdin. Only inserted if not already present.
- **Hidden input via raw mode**: `term.MakeRaw` + byte-by-byte read without echo. Handles Backspace (no echo mark), Ctrl-C (cancel), Enter (submit).
- **Telegram no-sudo**: the bridge has no interactive terminal for password entry. Marking sudo declined is the safe default.

## Implementation Approach
- Seven files modified across three layers: interface (runtime), handler (console), execution (tools).
- Test mocks updated for all three Handler implementations (console_test, runtime_test, telegram).
- Safety: env var scoped per iteration, cleared by executeShell immediately, never stored in session JSON or echoed.

## Files Included
- `internal/runtime/runtime.go`: handler interface, RunTurn sudo flow, containsSudo
- `internal/console/console.go`: RequestSudoApproval implementation
- `internal/console/reader.go`: ReadHiddenInput (raw-mode no-echo reader)
- `internal/tools/shell.go`: executeShell sudo -S + stdin pipe
- `internal/console/console_test.go`: mock updated
- `internal/runtime/runtime_test.go`: mock updated
- `internal/telegram/handler.go`: no-sudo policy for Telegram transport

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
