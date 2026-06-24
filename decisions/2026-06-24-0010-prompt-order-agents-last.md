# Session Decision Summary: Move AGENTS.md to end of prompt build order

Date: 2026-06-24 00:10
Base commit: (working tree)

## Context
User requested AGENTS.md to be injected last in the system prompt, after the active skills section. Previously it was at position 4 (before memory and skills).

## Changes Made
Moved the AGENTS.md injection block from position 4 to position 7 (last) in `BuildRuntimePart()`. Updated all order documentation and tests.

## Decisions And Rationale
AGENTS.md contains project-specific rules that may reference skills or memory content. Placing it last ensures it can see the full context of what's available in the prompt, including loaded skills.

## Implementation Approach
Cut the AGENTS.md block (readFileOptional + injectVariables + header wrap) from between helpers and memory, reinserted it after the skills section. Updated comment on line 184, doc.go header, and two order-verifying tests.

## Files Included
- internal/prompt/prompt.go: moved AGENTS.md block, updated order comment
- internal/prompt/doc.go: updated order in package header
- internal/prompt/prompt_test.go: updated TestBuildRuntimePartOrder and TestBuildRuntimePartHelperOrder

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
