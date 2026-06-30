# Session Decision Summary: input-editor-bracketed-paste

Date: 2026-06-30 20:30
Base commit: 923a522

## Context
Users reported two input problems in the raw-mode REPL:
1. Arrow keys (left/right) and Delete key printed garbage codes instead of moving the cursor.
2. Pasting multi-line text lost newlines and submitted only the first line.

## Changes Made
- Implemented CSI escape sequence parsing in `ReadEvent` for cursor movement keys.
- Added cursor position tracking within the input buffer.
- Implemented full line redraw on insert/delete in middle of buffer.
- Added bracketed paste mode (`\033[?2004h`/`\033[?2004l`) for safe multi-line paste.
- Added `insertChar` helper shared between printable chars, pasted tabs, and pasted newlines.
- Added `SetPrompt` on Reader + prompt storage for accurate line redraw.
- Newlines in paste mode displayed as `\r\n` for correct terminal alignment.

## Decisions And Rationale
- CSI parsing state machine: simpler than adding a full readline dependency, no new external deps.
- `~` checked before `[0x40,0x7E]` range: critical fix, `~` (0x7E) was accidentally consumed by the letter-terminated CSI branch.
- Bracketed paste over timing heuristics: standard DEC mode, supported by all modern terminals, zero false positives.
- No auto-submit on paste end: user explicitly wants to edit pasted text before pressing Enter.
- `insertChar` extracted from `default: printable` — now shared with paste Tab/Enter to keep insert behavior consistent.
- Cursor movement across multi-line visual lines not implemented — a future concern; current UX allows editing at buffer end.

## Implementation Approach
- `internal/console/reader.go`: rewrote `ReadEvent` with CSI state machine, cursor pos tracking, `redrawLine`, `insertChar`. Added bracketed paste enable/disable, `SetPrompt`, `pasteMode` flag, `bytes` import.
- `internal/console/console.go`: set prompt on Reader before each `ReadEvent` call.

## Files Included
- `internal/console/reader.go`: full input rewrite
- `internal/console/console.go`: prompt sync
- `decisions/2026-06-30-2030-input-editor-bracketed-paste.md`: this summary
