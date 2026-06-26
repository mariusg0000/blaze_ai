# Session Decision Summary: add-project-map-placeholder

Date: 2026-06-26 15:45
Base commit: 9440348

## Context
Sysprompt.md was missing the `{PROJECT_MAP_CONTENT}` placeholder, even though `prompt.go` already reads `project-map.md` from the work directory and injects it into the template. Without the placeholder, the injection was silently ignored.

## Changes Made
- Added `[PROJECT MAP]` section with `{PROJECT_MAP_CONTENT}` placeholder at the end of `prompts/sysprompt.md`, after `[PROJECT RULES]`.

## Decisions And Rationale
- Placed after `{AGENTS_CONTENT}` as the last section per user request.

## Implementation Approach
Simple edit to `prompts/sysprompt.md`.

## Files Included
- `prompts/sysprompt.md`: added `[PROJECT MAP]` section.
- `decisions/2026-06-26-1545-add-project-map-placeholder.md`: this decision summary.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
