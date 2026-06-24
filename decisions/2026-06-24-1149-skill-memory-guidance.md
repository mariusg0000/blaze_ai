# Session Decision Summary: skill memory guidance

Date: 2026-06-24 11:49
Base commit: a4d3efe64ea3463e65648c85455c10e01a6fbe90

## Context
The working tree contained prompt and builtin skill text updates that add clearer guidance for skill loading, memory loading, related-item handling, and skill-memory separation.

## Changes Made
Updated `prompts/sysprompt.md` to include clearer instructions for loading skills and memories, deciding when related items should be loaded, and how to retain or unload active context. Updated `skills/skill-manager.md` to emphasize skill structure, procedural content, memory separation, and related memory guidance.

## Decisions And Rationale
The prompt now makes the runtime rules for loading and retaining skills/memories more explicit, while `skill-manager` focuses on keeping procedural behavior in skills and persistent facts in memory banks. This keeps the two systems aligned without mixing responsibilities.

## Implementation Approach
This was a documentation-only change. The runtime prompt text was updated in `prompts/sysprompt.md`, and the builtin skill text was updated in `skills/skill-manager.md`. No Go code changed.

## Alternatives Considered
No alternative implementation was needed because the request was to commit the existing edits, not redesign them.

## Files Included
- `prompts/sysprompt.md`: added clearer load, related-item, and retention guidance.
- `skills/skill-manager.md`: tightened skill-manager guidance around skill content and memory separation.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
