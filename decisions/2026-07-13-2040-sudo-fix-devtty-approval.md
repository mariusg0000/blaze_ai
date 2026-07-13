# Session Decision Summary: Sudo approval fix — independent controlling terminal

Date: 2026-07-13 20:40

## Context

The user reported that sudo approval stopped working after commit `745501f` ("Replace raw console input with readline"): the `Execute with sudo? [y/N]` prompt appeared but the response was always `declined by user`. A previous attempt to fix this with a second readline shell caused duplicate prompts and `getCursorPos() not supported by terminal emulator` errors.

## Changes Made

- `internal/console/console.go` — `RequestSudoApproval` now calls `c.Reader.ReadApproval()` instead of `c.Reader.ReadLine()`.
- `internal/console/reader.go` — added `ReadApproval()`, `openApprovalTTY()`, and `readTerminalLine()`. Rewrote `ReadHiddenInput()` to use the same explicit `/dev/tty` path instead of `os.Stdin`.

## Root Cause

The input path changed from:

```
stdin → raw Reader BlazeAI (controlling terminal)
```

to:

```
stdin → readline → REPL
```

After the readline migration, `os.Stdin` was no longer the raw controlling terminal during sudo approval. A separate readline shell failed because cursor capabilities were unavailable in the nested context. The previous `c.Reader.ReadLine()` relied on `bufio.Scanner` which could not read from the readline-managed terminal.

## Decision And Rationale

- Open `/dev/tty` explicitly for sudo approval and password input.
- `/dev/tty` always refers to the controlling terminal regardless of stdin state.
- Same `readTerminalLine()` helper for both visible (Y/N) and hidden (password) input: echo flag controls character visibility.
- Works because the main readline REPL has already returned control before `RequestSudoApproval` is called, so `/dev/tty` is available.
- Password input uses the same raw byte-by-byte loop with hidden echo (no visible characters, no echo).
- Backup saved under `/home/marius/blazeai/backups/sudo-fix/`.
