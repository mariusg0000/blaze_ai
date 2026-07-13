# Session Decision Summary: Single-Line Shortcut Status

Date: 2026-07-11 18:15

## Context

The transient-prompt implementation displayed the correct model status but committed a new line to terminal scrollback after every shortcut change. The required behavior is one continuously replaced line regardless of how many mode/model/favorite changes occur.

## Changes Made

- Removed the transient prompt configuration and `Prompt.Transient` registration.
- Restored status rendering to the primary readline prompt.
- Rendered `shortcutStatus` and the normal input prompt on one physical line.
- Changed status refresh back to `Display.Refresh()`, which redraws the active input line in place.

## Decisions And Rationale

The primary prompt is readline's redrawable input surface, while the transient prompt is intentionally designed to leave output in scrollback. Keeping status on the primary line is the only simple library-supported approach that guarantees no new console lines while preserving readline cursor and buffer management.

## Validation

- `go test ./...` — passed
- `go build ./...` — passed
- `go vet ./...` — passed
- `git diff --check` — passed
