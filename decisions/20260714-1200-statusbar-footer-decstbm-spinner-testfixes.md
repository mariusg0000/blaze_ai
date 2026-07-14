# Session Decision Summary: bottom-pinned statusbar via DECSTBM + test fixes

Date: 2026-07-14 12:00

## Context

The statusbar should remain fixed at the bottom of the terminal during LLM streaming. Earlier attempts with `\033[F` redraws failed because reasoning text spans multiple lines. Terminal scrolling regions (DECSTBM) are the correct mechanism.

## Changes Made

- `internal/console/console.go`: Replaced `\033[F` redraw footer with DECSTBM scrolling region (`\033[1;H-1r`), `finishTurnStatusBar` resets region; removed spinner/footer mutex contention in `lockOutput`/`unlockOutput`; moved spinner interval to per-instance field (`spinnerInterval`) to eliminate shared-global race; added `turnStatusBarVisible` guard on spinner start.
- `internal/console/console_test.go`: Fixed 9 spinner-related data races by draining `stopSpinner()` before reading buffer; set per-console `spinnerInterval` instead of global variable; fixed `TestSpinnerStopsBeforeContent`/`TestSpinnerStopsBeforeToolCall` to simulate output directly.

## Decisions and Rationale

- DECSTBM over `\033[F`: terminal-native scroll regions pin the last line independently of output length; `\033[F` breaks when reasoning spans more than one line.
- Per-instance spinner interval: eliminates the race detector failures caused by concurrent test mutation of a shared `var spinnerFrameInterval`.
- `stopSpinner()` before buffer reads in tests: the spinner goroutine writes to the same buffer; reading `.String()` without draining creates a data race.
