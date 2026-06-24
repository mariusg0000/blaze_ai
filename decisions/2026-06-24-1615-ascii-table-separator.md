# Session Decision Summary: ASCII table separator

Date: 2026-06-24 16:15
Base commit: e9c0843

## Context
The end-of-message separator was a single-line divider with a compact token label. The user wanted a three-line ASCII table showing CTX tokens, working directory, and current model.

## Changes Made
- `console.go`: Replaced `responseSeparator()` with three-line ASCII table renderer. Top/bottom lines use `┌─┬─┐`/`└─┴─┘` (TTY) or `+-+`/`+-+` (ASCII). Middle line shows `│ CTX: xxk │ workdir │ model │`. Added `path/filepath` import.
- `console_test.go`: Updated `TestOnUsage` to check for `"CTX: 11k"` and `"test/test-model"` instead of old `"ctx 11k"` format.

## Decisions And Rationale
- Three-line table format: user explicitly requested ASCII table with corners and aligned connectors.
- `ctxSeparator()` (tool group) left unchanged — only `responseSeparator()` modified.
- `filepath.Base()` used for workdir to show folder name only, not full path.
- Cell widths derived from content length — no fixed padding.

## Files Included
- internal/console/console.go: ASCII table separator
- internal/console/console_test.go: updated test for new format
