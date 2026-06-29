# Session Decision Summary: FormatArgs Purpose No Truncate

Date: 2026-06-29 06:18
Base commit: 8b3b7e8

## Context

User noticed tool purpose text was being truncated with `...` in the console display. The `ask_a_friend` tool showed `Consulting summarization: The summarization model will analyze the current Bl...`. Requirement: purpose from LLM must never be truncated, only fallback text (when purpose is missing) should be limited to 50 chars.

## Changes Made

- `internal/tools/shell.go`: Fallback truncation limit 80→50. Purpose path was already correct (no truncation).
- `internal/tools/replace_block.go`: Removed `truncateDisplay` from all purpose paths. Fallback path-only display uses 50 char limit.
- `internal/tools/ask_friend.go`: Removed `truncateDisplay` from all purpose paths. Fallback role-only display uses 50 char limit.
- `internal/tools/tools_test.go`: `TestShellFormatArgsFallbackTruncated` updated for 50 char limit.

## Files Included
- `internal/tools/shell.go`: fallback 80→50
- `internal/tools/replace_block.go`: purpose no truncation, fallback 80→50
- `internal/tools/ask_friend.go`: purpose no truncation, fallback 80→50
- `internal/tools/tools_test.go`: test updated for 50 char

## Commit Linkage
This summary is committed together with the implementation changes.
