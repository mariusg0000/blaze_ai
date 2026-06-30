# Session Decision Summary: consolidate specs-manager

Date: 2026-06-30 13:00
Base commit: 822bbd0

## Context
The user redesigned the project documentation system over multiple iterations. Final design: a single `specs-manager` skill that handles both quick project maps and full architecture specs. The old `project-map` skill and the interim `project-specs` skill were removed. `specs.md` at project root replaces `project-map.md` and `specs/README.md` as the single auto-injected file. The `specs/` folder holds individual spec files (only when full analysis is chosen). Sysprompt consolidated from `[PROJECT MAP]` + `[PROJECT SPECS]` into one `[PROJECT CONTEXT]` section.

## Changes Made
- Created `skills/specs-manager.md`: new consolidated skill for map + specs generation
- Deleted `skills/project-map.md` and `skills/project-specs.md`
- Modified `prompts/sysprompt.md`: replaced `[PROJECT MAP]` + `[PROJECT SPECS]` with `[PROJECT CONTEXT]` reading `specs.md`
- Modified `internal/prompt/prompt.go`: reads `specs.md` instead of `project-map.md` + `specs/README.md`, injects as `PROJECT_CONTENT`
- Modified `internal/prompt/doc.go`: updated build order comments
- Modified `skills/skill-manager.md`: builtins list updated (removed project-map, added specs-manager)
- Modified `specs.md`, `specs/02-architecture.md`, `specs/04-prompts.md`, `specs/09-skill-system.md`: all project-map references replaced with specs-manager/specs.md

## Decisions And Rationale
- **specs-manager replaces both project-map and project-specs**: single skill handles short (map only) and full (map + specs) workflows. Two paths based on user choice, not separate skills.
- **specs.md as single injection point**: replaces both project-map.md and specs/README.md. Contains Description + Map + Specs index. One file to read, one variable in sysprompt.
- **specs/ folder only when user chooses full analysis**: no unnecessary folder creation for quick maps.
- **specs-manager skill mentions legacy project-map.md for migration**: intentional backward compatibility for existing workdirs.
- **Decisions and ideas left unchanged**: historical records of the old design kept intact.

## Implementation Approach
Pure file operations — new skill file, deleted old skills, replaced Go code in prompt package, updated sysprompt template, updated stale spec documentation. Zero behavior change in tests; all test fixtures are independent of embedded skills.

## Files Included
- `skills/specs-manager.md`: new builtin skill
- `skills/project-map.md`: deleted
- `skills/project-specs.md`: deleted
- `prompts/sysprompt.md`: consolidated to [PROJECT CONTEXT]
- `internal/prompt/prompt.go`: reads specs.md, injects PROJECT_CONTENT
- `internal/prompt/doc.go`: updated order comment
- `skills/skill-manager.md`: updated builtins list
- `specs.md`: updated builtins list
- `specs/02-architecture.md`: updated skills table, build order, data flow
- `specs/04-prompts.md`: updated sequence, variables, injection map
- `specs/09-skill-system.md`: updated builtins list
- `decisions/2026-06-30-1300-consolidate-specs-manager.md`: this decision summary

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
