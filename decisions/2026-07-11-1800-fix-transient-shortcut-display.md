# Session Decision Summary: Fix Transient Shortcut Display

Date: 2026-07-11 18:00

## Context

The previous status implementation put `shortcutStatus` into the multiline primary prompt and called `Display.Refresh()`. Readline's refresh path repaints only the last primary-prompt line, so Ctrl+\\ could execute without showing model status.

## Changes Made

- Restored the primary prompt to the normal console prompt.
- Enabled readline's `prompt-transient` option.
- Registered `Prompt.Transient` to render `shortcutStatus`.
- Changed shortcut status updates to call `Display.RefreshTransient()`.

## Decisions And Rationale

Readline's transient prompt is the correct replaceable status row: it is rendered above the active input and refreshed in place, without appending a new output line. This preserves the original shortcut UX while keeping cursor and input-buffer management inside readline.

## Validation

- `go test ./...` — passed
- `go build ./...` — passed
- `go vet ./...` — passed
- `git diff --check` — passed
