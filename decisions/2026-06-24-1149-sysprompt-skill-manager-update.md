# Session Decision Summary: sysprompt and skill-manager update

Date: 2026-06-24 11:49
Base commit: a4d3efe64ea3463e65648c85455c10e01a6fbe90

## Context
The worktree contained direct edits to the universal runtime prompt and the builtin `skill-manager` skill. The prompt added new operational guidance for skills, memories, related-item loading, and retention, while `skill-manager` was tightened to better describe skill structure and skill-vs-memory separation.

## Changes Made
Updated `prompts/sysprompt.md` to include clearer instructions for loading skills and memories, handling related items, and retaining active context. Updated `skills/skill-manager.md` to emphasize what belongs in a skill, what does not, and how to keep skill content distinct from memory-bank data.

## Decisions And Rationale
The runtime prompt should carry the live behavior rules, while `skill-manager` should define the content boundaries for skill files. Keeping those responsibilities separate makes the system easier to reason about and reduces overlap between behavior guidance and stored facts.

## Implementation Approach
This was a documentation-only change. The prompt text was updated in `prompts/sysprompt.md`, and the builtin skill content was updated in `skills/skill-manager.md`. No Go code changed.

## Alternatives Considered
No alternate implementation was needed because the request was to commit the existing edits as-is.

## Files Included
- `prompts/sysprompt.md`: updated runtime guidance for loading, related items, and retention.
- `skills/skill-manager.md`: tightened skill content guidance and memory separation rules.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
