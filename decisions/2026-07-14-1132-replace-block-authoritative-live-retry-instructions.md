# Session Decision Summary: replace block authoritative live retry instructions

Date: 2026-07-14 11:32

## Context

After partial replace_block success, the agent sometimes retried with stale old_block text or resent already-applied blocks. The tool itself worked correctly, but the failure report did not make the authoritative live file state explicit enough.

## Changes Made

- `internal/tools/replace_block.go`
  - Added explicit `applied_blocks` listing in partial-failure reports.
  - Marked the returned live file content as authoritative for retries.
  - Strengthened tool description and block schema wording around post-partial-success retry rules.
- `internal/tools/replace_block_test.go`
  - Added assertions for authoritative retry markers in partial-failure reporting.

## Decisions And Rationale

- Kept partial-success behavior instead of all-or-nothing replacement.
- Added plain structured markers instead of hashes, avoiding overengineering.
- Used mandatory wording in tool output/schema to reduce agent retry mistakes.

### Included unrelated or pre-existing changes

- None.
