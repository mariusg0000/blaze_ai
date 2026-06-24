# Session Decision Summary: sysprompt update

Date: 2026-06-24 11:03
Base commit: 8f234a21dc470452e03a25a40c41d602febd491e

## Context
The universal runtime prompt in `prompts/sysprompt.md` was updated directly in the worktree. The change rearranges prompt sections and adds new guidance for tool batching, skill and memory loading, related-item loading, and retention.

## Changes Made
Updated `prompts/sysprompt.md` to add sequential tool-call batching guidance, explicit skill and memory loading guidance, related-item loading rules, and stronger retention instructions. The file also keeps the existing runtime section structure and placeholder-driven injection model.

## Decisions And Rationale
The prompt now gives the model more direct guidance on how to batch tool calls and how to treat skills and memories as related runtime context. The goal is to make load/unload behavior and tool sequencing more explicit without changing code.

## Implementation Approach
This was a prompt-only change: the updated guidance lives in `prompts/sysprompt.md`, and no Go code changed. Validation was done with the full Go test suite to ensure the prompt update did not break prompt assembly expectations.

## Alternatives Considered
No code-side change was needed because the request was limited to the prompt text already in the repository.

## Files Included
- `prompts/sysprompt.md`: updated runtime guidance and section ordering.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
