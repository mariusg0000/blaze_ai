# Session Decision Summary: Readline Console Input

Date: 2026-07-11 16:30

## Context

The custom raw input parser and manual redraw logic produced corrupted multiline paste display, including missing lines and incorrect cursor placement. The required workflow is a simple console: type or paste multiline text, continue typing before or after the paste, then press Enter to submit the complete buffer.

## Changes Made

- Added `github.com/reeflective/readline v1.3.0`.
- Replaced the main console REPL input loop with `reeflective/readline`.
- Enabled the library's bracketed-paste support explicitly.
- Configured `AcceptMultiline` so Enter accepts the full buffer while pasted newlines remain embedded.
- Removed the custom REPL raw parser, paste parser, cursor logic, and `redrawLine` implementation from `internal/console/reader.go`.
- Kept only auxiliary Reader functionality for sudo confirmation, hidden password input, and compatibility history helpers.
- Updated tests to validate the auxiliary Reader boundary rather than removed private paste helpers.

## Decisions And Rationale

A mature readline implementation is the correct ownership boundary for cursor movement, terminal redraw, history, bracketed paste, and multiline buffer editing. The application now receives one complete string only after Enter, so paste content is not interpreted as individual key events and the application never redraws the input buffer manually.

## Validation

- `go mod tidy` — passed
- `go test ./...` — passed
- `go build ./...` — passed
- `go vet ./...` — passed
- `git diff --check` — passed
