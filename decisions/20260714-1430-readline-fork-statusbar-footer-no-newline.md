# Session Decision Summary: readline fork for footer-less persistent hint

Date: 2026-07-14 14:30

## Context

The readline library appends a CRLF after every persistent hint, which leaves an unavoidable blank row below the statusbar between turns. Multiple ANSI-level fixes in console.go were attempted and all failed because readline's cursor accounting breaks when external sequences move the cursor.

## Changes Made

- `third_party/readline/` — local fork of `reeflective/readline@v1.3.0` with two patches:
  - `internal/ui/hint.go`: `renderLocked()` no longer appends `NewlineReturn` after the final hint lane when only the persistent lane is active (standalone footer mode). Other lane combinations keep their trailing newline.
  - `internal/display/refresh.go`: `renderHelpers()` cursor restoration uses `hintRows - 1` when the hint has content, accounting for the missing trailing newline.
- `go.mod`: added `replace github.com/reeflective/readline => ./third_party/readline`.

## Decisions And Rationale

- Local fork over upstream workaround: all ANSI-level fixes in console.go broke readline's internal cursor bookkeeping. The only clean fix is in the renderer itself.
- Conditional newline: only the standalone persistent lane (no provided/transient/text lanes) omits the trailing newline. This preserves normal readline behavior for completions, isearch, and transient hints.
- Cursor accounting: `MoveCursorUp(hintRows - 1)` instead of `MoveCursorUp(hintRows)` + `MoveCursorUp(1)` because the last hint row now has no trailing CRLF.
