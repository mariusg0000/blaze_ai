# Session Decision Summary: jq-helper-replace-block-warning

Date: 2026-06-28 20:43
Base commit: 40a66333d6dd889aecac46660a0a017f959a6057

## Context
A learning review report flagged recurrent `replace_block` misuse on JSON files (whitespace mismatch). The user decided the best fix is to embed the correct tool-selection rule directly in the `jq` helper description so the model sees the instruction whenever `jq` is available or listed as optional.

## Changes Made
Changed the `Description` field for the `jq` helper in `internal/helpers/helpers.go` from `"json_inspection_and_transformation"` to `"use jq to load, edit, and modify JSON files; never use replace_block on JSON"`.

## Decisions And Rationale
The helper description is the right place because it is rendered directly into the prompt's available/optional helpers section and is conditional on the helper's presence, keeping the rule out of the fixed system prompt and tied to the actual tool.

## Implementation Approach
Edited one string constant in `internal/helpers/helpers.go`. No test changes required (tests check helper availability, not description content). Validated with `go test ./internal/helpers ./internal/prompt ./internal/runtime` and `go test ./...`.

## Files Included
- `internal/helpers/helpers.go`: updated jq description.
- `decisions/2026-06-28-2043-jq-helper-replace-block-warning.md`

## Commit Linkage
This summary is committed together with the implementation change to keep rationale linked to code history.
