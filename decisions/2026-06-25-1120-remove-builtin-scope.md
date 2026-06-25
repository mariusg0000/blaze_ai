# Session Decision Summary: remove-builtin-scope

Date: 2026-06-25 11:20
Base commit: e13a595

## Context
The user clarified that builtin skills should not be a runtime scope. Embedded skills are templates only — seeded once into `app_home/skills/` at startup. At runtime, only two scopes exist: global and project. The user wanted the ability to edit seeded skills and restore originals by deleting the folder and restarting.

## Changes Made
- `internal/skills/skills.go`: Removed `ScopeBuiltin` constant. Removed `discoverFromFS`, `DiscoverFromFS`, `discoverBuiltinFromDir`, `DiscoverFromDirs` (all builtin-related discovery). Added `SeedBuiltins(templatesFS, appHomeSkillsDir)` that copies embedded .md files into `app_home/skills/<name>/skill.md` only if missing. Renamed `DiscoverFromDirs` to `DiscoverGlobalFromDir` (test helper, single dir). `DiscoverAll` now takes only `workDir` and derives global dir from `platform.AppHome()`. `Resolve` checks only `global/name` and `project/name` candidates.
- `internal/prompt/prompt.go`: Removed `BuiltinSkillsFS` field from `Builder`. `buildSkillsSection` calls `skills.DiscoverAll(b.WorkDir)` directly.
- `internal/runtime/runtime.go`: Removed `builtinSkillsFS` parameter from `NewAgent`. Skill resolver uses `skills.DiscoverAll(agent.WorkDir)`.
- `main.go`: Resolves embedded templates FS, calls `skills.SeedBuiltins()` before creating agent. Added `skills` import.
- `skills/customize_me.md`: Added "Customizing Builtin Skills" section documenting the seed/edit/restore workflow.
- `skills/skill-manager.md`: DATA section updated — two runtime scopes, seeding behavior, restore via delete+restart.
- `prompts/sysprompt.md`: Updated to two scopes only (global and project). Simplified scoped ID guidance.

## Decisions And Rationale
- Builtin scope removed entirely because the user's model is: templates → seed → edit → restore. No need for a `builtin/` runtime prefix when all skills are either global or project.
- Seeding is idempotent (checks `os.Stat` before writing) so existing user edits are never overwritten.
- Restore is manual (delete folder + restart) rather than automatic — the user preferred explicit control.

## Implementation Approach
- `SeedBuiltins` iterates embedded .md files via `fs.ReadDir`, checks `os.Stat` for each target, creates `MkdirAll` + `WriteFile` if missing.
- `DiscoverAll` calls `platform.AppHome()` internally to locate the global skills dir, keeping the call sites (prompt builder, resolver) simple.
- `discoverFromSubdirs` simplified — only two scope cases: project and default (global).

## Files Included
- `internal/skills/skills.go`: Core logic — removed builtin, added seeding, simplified discovery
- `internal/prompt/prompt.go`: Removed BuiltinSkillsFS field
- `internal/runtime/runtime.go`: Simplified NewAgent signature
- `main.go`: Seed call + skills import
- `skills/customize_me.md`: Seed/restore documentation
- `skills/skill-manager.md`: Updated scoping data
- `prompts/sysprompt.md`: Two-scope model
- `decisions/2026-06-25-1120-remove-builtin-scope.md`: This summary

## Commit Linkage
This summary is committed together with the implementation changes.
