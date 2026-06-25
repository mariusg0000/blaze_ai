# Session Decision Summary: scoped-skill-ids

Date: 2026-06-25 11:00
Base commit: 8a6b62d

## Context
The user reviewed the skills+memories unification implementation against the plan in `New.txt` and identified discrepancies: skill IDs used bare names for builtin/global scopes, `ScopeBuiltin` was undefined, and same-name collisions silently let global override builtin instead of reporting ambiguity.

## Changes Made
- `internal/skills/skills.go`: All skills now use canonical scoped IDs (`builtin/name`, `global/name`, `project/name`) as map keys. `ScopeBuiltin` constant added. `discoverFromFS`, `discoverFromSubdirs`, `discoverBuiltinFromDir`, `DiscoverFromDirs` all use prefix keys. `Resolve()` handles qualified lookups (builtin/x, global/x, project/x) and unqualified lookups that check all three scopes — ambiguous matches error with candidate list instead of silently overriding.
- `prompts/sysprompt.md`: Updated to reference `builtin/name`, `global/name`, `project/name` canonical IDs. Skill loading guidance now mentions all three scoped prefixes.
- `skills/skill-manager.md`: DATA section updated — `skill.collision` replaced with `skill.canonical_ids`, `skill.resolution`, and `skill.naming` rules matching the new behavior.
- `New.txt`: Reference plan document included for traceability.

Additionally, the user's personal skills at `/home/marius/blazeai/skills/` were converted: 5 former memory files migrated to skill folders with `[DATA]` sections, existing skill `[DETAILS]` sections converted to `[BEHAVIOR]`, cross-references updated from "memory" to "skill", and 3 split behavior/data pairs merged into single skills (gree_ac_control, music_player, node_server).

## Decisions And Rationale
- All three scopes use prefixed canonical IDs internally for consistency, matching the plan's recommendation: "Use canonical IDs: builtin/name, global/name, project/name."
- Ambiguous unqualified names now error instead of silently preferring one scope — this matches the plan's "no fallback silențios" rule.
- `ScopeBuiltin` was missing from the initial implementation; added to enable builtin skill discovery to set the correct scope field.

## Implementation Approach
- Map keys changed from bare names to prefixed IDs at discovery time, not at resolution time. This keeps the skill map self-describing and avoids needing separate scope tracking during lookup.
- `Resolve()` checks three candidate scopes (`builtin/name`, `global/name`, `project/name`) for unqualified names, collects all matches, and errors if ambiguous.
- Prompt display uses the scoped IDs directly (e.g., `builtin/skill-manager.md`, `global/project_hub.md`), giving the LLM full visibility into which scope each skill belongs to.

## Alternatives Considered
- Keeping global as bare-name override (like before) was rejected because the plan explicitly requires ambiguity errors, not silent preference.
- Omitting `builtin/` prefix for display (plan mentions this option) was left for a future refinement; currently all three scopes show their prefix.

## Files Included
- `internal/skills/skills.go`: Core skill model and discovery with scoped IDs
- `prompts/sysprompt.md`: LLM guidance for scoped skill IDs
- `skills/skill-manager.md`: Updated skill authoring data with scoping rules
- `New.txt`: Reference plan document that drove the implementation fix
- `decisions/2026-06-25-1100-scoped-skill-ids.md`: This summary

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
