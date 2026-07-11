# Session Decision Summary: Console Input History

Date: 2026-07-11

## Context

The raw console input editor had no history navigation. Users needed to retype previous prompts, including multiline prompts. The console already handled CSI arrow sequences but ignored Up and Down.

## Changes Made

- Added in-memory input history to `internal/console/reader.go`.
- Added Up/Down navigation through CSI `A` and `B` events.
- Preserved the current draft when entering history and restored it after moving past the newest entry.
- Suppressed consecutive duplicate entries.
- Exited history navigation when the recalled entry was edited.
- Added normal submitted messages to history from `internal/console/console.go`.
- Added unit tests for history storage, duplicate suppression, copy safety, navigation, and draft restoration in `internal/console/console_test.go`.

## Decisions And Rationale

History is kept in memory only for the current application session. Persistence was intentionally not added because the immediate requirement was fast navigation of recent console input, while persistent history would introduce a separate storage policy and privacy concern.

Slash commands and empty inputs are not stored. Consecutive duplicates are suppressed to keep navigation useful without adding unrelated persistence or filtering rules.

The existing raw reader and redraw path were retained to keep this change focused. Up and Down currently navigate command history rather than moving the cursor vertically inside the current multiline draft.

## Validation

- `go test ./...` — passed
- `go build ./...` — passed
- `git diff --check` — passed

Implementation commit: `ced4d7c`
