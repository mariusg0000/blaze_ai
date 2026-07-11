# Session Decision Summary: Simple Console Bracketed Paste

Date: 2026-07-11 16:15

## Context

The Bubble Tea migration was reverted to `checkpoint-before-tui-migration` because the requested workflow did not require typed multiline editing. The required behavior is: type text or paste multiline content, continue typing after the paste, then press Enter to submit the complete buffer.

## Changes Made

- Kept the existing simple raw console and Enter-to-submit behavior.
- Changed bracketed paste handling in `internal/console/reader.go`:
  - collects the entire paste payload before modifying the input buffer;
  - preserves internal newlines;
  - removes only trailing CR/LF added by clipboard tools;
  - inserts the paste atomically at the cursor;
  - redraws once after paste, then allows normal typing to continue.
- Added `internal/console/reader_test.go` covering internal newline preservation and appending text after a pasted block.

## Decisions And Rationale

A full TUI is unnecessary for paste-based multiline input. The existing console already has bracketed-paste mode and Enter submission; the fragile behavior came from processing every pasted byte as an interactive key and redrawing repeatedly. Atomic paste insertion keeps the simple console while allowing logs, lists, and code to be pasted and extended before submission.

## Validation

- `go test ./...` — passed
- `go build ./...` — passed
- `go vet ./...` — passed
- `git diff --check` — passed
