# Session Decision Summary: status bar layout arrow prompt remove shortcut messages

Date: 2026-07-14 11:08

## Context

The console transport needed a cleaner persistent footer layout, a stable user input marker, and removal of transient shortcut feedback in the prompt. Mode/model/reasoning/directory/context information should appear only in the persistent status bar.

## Changes Made

- `internal/console/console.go`
  - Reordered status bar to `mode | model | reasoning level | directory | CTX/CH/CM`.
  - Made mode bold and yellow in the status bar.
  - Replaced the prompt with a stable arrow marker.
  - Removed prompt shortcut messages for mode/model/favorite/reasoning changes while keeping visible error output.
  - Disabled the obsolete boxed end-of-turn separator.
- `internal/console/console_test.go`
  - Updated prompt tests for the new stable arrow marker.
  - Removed obsolete shortcut-status prompt expectations.

## Decisions And Rationale

- Status-only feedback avoids prompt instability during multiline editing.
- Removed boxed separator because the persistent footer now covers the same information continuously.
- Mode segment styling uses direct ANSI resets plus background restore to preserve the dark footer background.

### Included unrelated or pre-existing changes

- None.
