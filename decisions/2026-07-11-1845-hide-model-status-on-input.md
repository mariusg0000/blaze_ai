# Session Decision Summary: Hide Model Status On Input

Date: 2026-07-11 18:45

## Context

The compact prompt correctly showed `[mode][model]` after a shortcut, but because the model bracket was part of the primary readline prompt, it could remain attached to submitted user input. The required behavior is to show the model only while the shortcut status is active and the input buffer is empty; submitted text and the next prompt must show only `[mode]`.

## Changes Made

- Added `promptLabelWithStatus(includeStatus bool)`.
- Made the readline primary prompt include shortcut status only when `rl.Line().Len() == 0`.
- Preserved the model bracket immediately after mode after a shortcut.
- Ensured typed or pasted input, accepted messages, and subsequent prompts use only the mode label.

## Decisions And Rationale

The prompt callback can observe the readline buffer during redraw. Hiding the one-shot status as soon as the buffer contains user text avoids embedding the model label into accepted input while retaining same-line shortcut feedback and native readline editing.

## Validation

- `go test ./...` — passed
- `go build ./...` — passed
- `go vet ./...` — passed
- `git diff --check` — passed
