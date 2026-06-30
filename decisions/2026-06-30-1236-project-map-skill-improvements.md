# Session Decision Summary: project map skill improvements

Date: 2026-06-30 12:36
Base commit: dfc82d9

## Context
The user asked for analysis of the builtin `project-map` skill and its auto-injection into `[PROJECT MAP]` in the sysprompt. The analysis found three gaps: the skill didn't mention auto-injection or its downstream prompt cost, had no size guidance, and lacked staleness detection. The `[PROJECT MAP]` section in `sysprompt.md` was bare with no usage instructions for the model.

## Changes Made
Updated two files with textual improvements based on the analysis:

- `skills/project-map.md`: added auto-injection awareness in description and purpose, added size target (20-40 lines, max 60), added output rule for size, added `## Usage` section, added `## Staleness` section.
- `prompts/sysprompt.md`: expanded the `[PROJECT MAP]` section with usage instructions for the model and a note when the map does not exist.

## Decisions And Rationale
No code changes were needed — the auto-injection already works in `internal/prompt/prompt.go`. The gaps were only in documentation/instructions. The model needs to know: (1) its output has downstream prompt cost, (2) how big the map should be, (3) when to regenerate, and (4) how to use an existing map. All four points are now covered.

## Implementation Approach
Direct text edits to the skill and sysprompt files. No runtime logic changes. Validated with `go test ./...` (all pass).

## Files Included
- `skills/project-map.md`: skill improvements
- `prompts/sysprompt.md`: [PROJECT MAP] section improvements
- `decisions/2026-06-30-1236-project-map-skill-improvements.md`: this decision summary

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
