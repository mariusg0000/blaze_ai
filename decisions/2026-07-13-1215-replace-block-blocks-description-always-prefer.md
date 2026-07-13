# Session Decision Summary: replace_block blocks description always prefer

Date: 2026-07-13 12:15

## Context

The user noticed that `blocks` description said "two or more edits in the same file", which discouraged LLM from using `blocks` for a single edit, contrary to the goal of preferring `blocks` always.

## Changes Made

- `internal/tools/replace_block.go` — `Description()`: removed "for multiple edits"
- `internal/tools/replace_block.go` — `blocks.description`: removed "when making two or more edits"

## Decisions And Rationale

- LLM should prefer `blocks` for any number of edits (one or many), not only for multiple edits.
- The legacy single-block form `old_block`/`new_block` remains supported for backward compatibility but is marked as "Legacy" to push LLM towards `blocks`.
- Simpler wording reduces cognitive load: "Prefer blocks." is unambiguous.
