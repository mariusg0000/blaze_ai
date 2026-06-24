# Session Decision Summary: prompt null rendering

Date: 2026-06-24 10:17
Base commit: f1a942de2fd140ef6d9a693609b4d30611a3c17c

## Context
The runtime prompt template needed to remain fully literal and predictable: every section label must stay visible, and any missing value must still render in the prompt instead of being removed or silently reshaped.

## Changes Made
Removed the special-case text pruning logic from prompt assembly and changed placeholder injection so missing or empty known values render as `NULL`. Updated tests to assert the uniform `NULL` behavior across host helpers, skills, memories, and AGENTS.md.

## Decisions And Rationale
`NULL` was chosen over `NONE` because it reads as a technical missing-value marker and keeps the prompt behavior explicit and consistent.

## Implementation Approach
`internal/prompt/prompt.go` now returns `NULL` for empty injected values and no longer deletes any rendered prompt text. The prompt template in `prompts/sysprompt.md` remains the single source of section labels, and the tests assert the exact visible `NULL` output.

## Alternatives Considered
Leaving empty sections out was rejected because it made the final prompt inconsistent with the literal template text.

## Files Included
- `internal/prompt/prompt.go`: removed pruning and made empty injected values render as `NULL`.
- `internal/prompt/prompt_test.go`: updated expectations for literal `NULL` output.
- `internal/console/console_test.go`: kept prompt fixtures aligned with the unified runtime template.
- `internal/runtime/runtime_test.go`: kept prompt fixtures aligned with the unified runtime template.
- `prompts/sysprompt.md`: retained as the single prompt layout.
- `prompts/readme.md`: documents the final placeholder set.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
