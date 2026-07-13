# Session Decision Summary: replace_block multiblock exact match with live file feedback

Date: 2026-07-13 12:00

## Context

User requested upgrading `replace_block` from single-block-first-match to multiblock with strict exact match, duplicate detection, partial progress, and live file attachment for retry.

## Changes Made

- `internal/tools/replace_block.go` — full rewrite of args, validation, matching, execution, and output format
- `internal/tools/replace_block_test.go` — replaced `TestReplaceBlockOnlyFirstOccurrence` with `TestReplaceBlockDuplicateMatch`, added `TestReplaceBlockMultiplePartialSuccess` and `TestReplaceBlockMultipleSuccess`

## Decisions And Rationale

- **Multiblock via `blocks` array**: single call replaces N exact blocks in one file; `blocks` rejects mixed format with legacy `old_block`/`new_block`.
- **Exact match with unique requirement**: each `old_block` must appear exactly once; zero matches → `match not found`, multiple matches → `ambiguous`. No fallback normalization.
- **Partial progress**: successful blocks are written; failed blocks are reported without rolling back successes.
- **Live file attachment**: on failure, tool reads the live file from disk and attaches content if under 500000 bytes. LLM retries only the failed blocks without calling `read_file`.
- **Permissions preserved**: write uses `originalInfo.Mode()` instead of hardcoded `0644`.
- **Legacy compatibility**: single `old_block`/`new_block` still works as a one-element `blocks` list.
- **`oneOf` schema validation**: JSON Schema enforces exactly one of `blocks` or the legacy pair.
