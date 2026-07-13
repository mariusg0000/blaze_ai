# Session Decision Summary: Skill kind markers — [RUNNABLE] / [NON-RUNNABLE] exclusive types

Date: 2026-07-13 13:55

## Context

LLM would confuse runnable skills with loadable skills and try to `load_skill` on runnable-only skill names. The prompt had no clear separation between the two categories.

## Changes Made

- `internal/skills/skills.go` — Added `ErrMixedSkillTypes`, parse validation rejects skills combining `[BEHAVIOR]`/`[DATA]` with `[SYNTAX]`/`[CODE]`
- `internal/skills/skills_test.go` — Added `TestParseRejectsMixedSkillTypes`
- `internal/prompt/prompt.go` — `{SKILLS_AVAILABLE}` header: `[NON-RUNNABLE SKILLS — use load_skill only]`; `{RUNNABLE_SKILLS_SECTION}` header: `[RUNNABLE SKILLS — use run_skill only; never use load_skill]`
- `internal/prompt/prompt_test.go` — Updated assertions for new markers
- `internal/tools/skill_tools.go` — `LoadSkillTool` resolver type changed to `ResolveSkillFunc`; `load_skill` description and schema declare `[NON-RUNNABLE]` only; runtime rejects runnable skills with "use run_skill" guidance; `run_skill` description: `execute [RUNNABLE] skill; do not use load_skill`
- `internal/tools/skill_tools_test.go` — Added `TestLoadSkillExecuteRejectsRunnable`; updated `run_skill` description test
- `internal/runtime/runtime.go` — `load_skill` registered with `runnableSkillResolver` for type-checking
- `specs/09-skill-system.md` — Documented exclusive skill types, updated prompt examples

## Decisions And Rationale

- Exclusive types: a skill must be either loadable (`[BEHAVIOR]`/`[DATA]`) or runnable (`[SYNTAX]`/`[CODE]`), not both. This eliminates the primary source of LLM confusion.
- Prompt markers: inject explicit `[NON-RUNNABLE]` and `[RUNNABLE]` headers that tell the LLM which tool to use for each category.
- Tool descriptions: repeat the restriction in both `load_skill` and `run_skill` schemas so even without reading the full prompt the LLM sees the rule.
- Runtime protection: `load_skill.Execute` checks `IsRunnable()` and refuses runnable-only skills with a clear error pointing to `run_skill`.
- `RunSkillTool` resolver already has the parsed skill struct; `LoadSkillTool` resolver upgraded from `ResolveFunc` to `ResolveSkillFunc` to enable the check.
