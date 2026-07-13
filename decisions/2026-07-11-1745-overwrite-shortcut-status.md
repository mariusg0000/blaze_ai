# Session Decision Summary: Overwrite Shortcut Status Lines

Date: 2026-07-11 17:45

## Context

The original console behavior reused one status line for consecutive mode/model/favorite/reasoning shortcut changes. The readline migration used `PrintTransientf`, which intentionally prints a status line and pushes the prompt below it, causing a new console line on every shortcut press.

## Changes Made

- Added `shortcutStatus` to the console state.
- Extended the readline primary prompt callback to render the status above the input prompt.
- Replaced `PrintTransientf` shortcut feedback with a status setter that updates the prompt and calls `Display.Refresh()`.
- Cleared the status when Enter accepts the input buffer, so it remains tied to the active editing prompt.

## Decisions And Rationale

The status is now part of readline's redrawable prompt rather than application output. Readline can repaint the prompt and buffer in place, preserving the original one-line overwrite behavior without manually moving the terminal cursor or appending transient lines.

## Validation

- `go test ./...` — passed
- `go build ./...` — passed
- `go vet ./...` — passed
- `git diff --check` — passed
