# Session Decision Summary: Helpers Cleanup

Date: 2026-06-29 06:22
Base commit: 3638442

## Context

User requested:
1. Remove contextual helpers (go, node, npm, etc.) — they're obvious when working in that project, no need to list as helpers
2. Update helper descriptions to be clear and cross-platform (no references to grep/find)
3. Add preamble saying to prefer helpers over classic shell equivalents

## Changes Made

- `internal/helpers/helpers.go`: Removed 9 contextual helper entries from `Known` list (go, node, npm, pnpm, yarn, bun, cargo, rustc, docker). Updated descriptions for remaining 7 (rg, fd, jq, git, curl, pandoc, sqlite3) to be clearer and self-contained without Unix-specific references.
- `internal/helpers/helpers_test.go`: Removed `TestAvailableHelpersContextualGo`, `TestAvailableHelpersContextualNode`, `TestProjectRelevant`. Cleaned up unused imports (`os`, `path/filepath`).
- `prompts/sysprompt.md`: Replaced "Use these helpers with shell tool." with full preamble in the `[HOST ENVIRONMENT HELPERS]` section — editable without recompiling.

## Preamble Text (sysprompt.md:99)

"Already verified — no need to check availability. Prefer these helpers over their classic shell-only equivalents. When a helper covers a task domain, always choose it over traditional commands."

## Implementation Approach

- Preamble placed in sysprompt.md template, not hardcoded in Go (per user preference)
- buildHostHelpersSection() left unchanged (no new preamble code)
- Remaining dead code (KindContextual, ProjectFiles, ProjectRelevant) left in place — compiles fine, not scope creep

## Files Included
- `internal/helpers/helpers.go`: entries removed, descriptions updated
- `internal/helpers/helpers_test.go`: tests removed, imports cleaned
- `prompts/sysprompt.md`: preamble added

## Commit Linkage
This summary is committed together with the implementation changes.
