# Session Decision Summary: exclusive-skill-types

Date: 2026-06-27 11:46
Base commit: 62d352c

## Context
Skill-manager instructions were ambiguous about the relationship between loadable and runnable skills. It presented runnable sections (`[SYNTAX]` + `[CODE]`) as optional extensions that could be added to a normal skill. This caused the LLM to propose mixed-type skills (e.g., `disk-check` with both `[BEHAVIOR]` and `[SYNTAX]`+`[CODE]`), which contradicts the runtime model where runnable skills do not enter the prompt context.

## Changes Made
- Rewrote `Share Format` in `skills/skill-manager.md` to define two exclusive skill types (loadable vs runnable), replacing the ambiguous "optional sections" model.
- Added new `Skill Type Decision` section with explicit criteria for when to choose each type and when to refuse runnable.
- Rewrote `Runnable Skills (v1)` section as a standalone type, not an extension of loadable skills. Explicitly forbids `[BEHAVIOR]` or `[DATA]` in runnable skills.
- Fixed `BEHAVIOR vs DATA` section to say "loadable skill" instead of "skill".
- Removed all language suggesting runnable sections can be mixed with loadable sections.
- Synced app-home copy at `/home/marius/blazeai/skills/skill-manager/skill.md`.
- Added `decisions/2026-06-27-1146-exclusive-skill-types.md`.

## Decisions And Rationale
- **Exclusive types contract**: A skill is either loadable or runnable, never both. This matches the runtime: loadable skills inject context, runnable skills only list as a tool entry. If a use case needs both, the rule says create two separate skills.
- **Skill Type Decision section**: provides a decision matrix the LLM can reference when the user says "make a skill". Previously the model had to infer type from format rules alone.
- **Do not make runnable**: Explicit refusal criteria reduce the chance the LLM makes a skill runnable when it requires conversation, exploration, judgment, or sudo.
- **No parser changes**: The parser already accepts both shapes. The fix is entirely instructional — teach the model to pick one shape per skill.

## Implementation Approach
- Edited `skills/skill-manager.md` in three regions: Skill Format, new Skill Type Decision insertion, Runnable Skills rewrite.
- Copied to app-home after edit.
- Validated with `go test -count=1 ./internal/skills ./internal/prompt`.

## Alternatives Considered
- Changing the parser to enforce exclusive types (forbid [BEHAVIOR] + [SYNTAX] in same skill). Rejected because the runtime currently tolerates mixed content and a future use case might need both. Instruction-level exclusivity is sufficient.
- Merging Skill Type Decision into existing sections. Rejected — a dedicated block is easier for the model to scan and reference.

## Files Included
- `skills/skill-manager.md`: instructional rewrite for exclusive skill types

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
