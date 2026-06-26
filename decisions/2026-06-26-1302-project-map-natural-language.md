# Session Decision Summary: project-map-natural-language

Date: 2026-06-26 13:02
Base commit: 02074db

## Context
Session started with review of last 15 decisions and 6 ideas. User evaluated a `diff helper` for system-level diff (like `sd` or `rg`) and rejected it as premature — shell tool already covers `diff` / `git diff` cross-platform.

From the ideas, only `project-map` was deemed worth implementing now. It is a skill (not runtime code), fits the existing `AGENTS.md` auto-inject pattern, and gives the LLM project structure context without manual exploration.

A secondary thread in the session: user decided that compact-language conversion of skill and prompt .md files was undesirable. `prompts/sysprompt.md` and `skills/skill-manager.md` were reverted to their natural-language prose versions from commit `82e525d`.

## Changes Made
- **New builtin skill `project-map`**: guides the LLM to scan the working directory and generate `project-map.md`. Written in natural-language prose.
- **Auto-inject in prompt**: `internal/prompt/prompt.go` reads optional `{WORK_DIR}/project-map.md` and injects it via a new `{PROJECT_MAP_CONTENT}` placeholder, placed before `AGENTS.md` in the runtime prompt.
- **Prompt template updated**: `prompts/sysprompt.md` now has a `## Project Map` section with the `{PROJECT_MAP_CONTENT}` placeholder.
- **Natural-language reversion**: `prompts/sysprompt.md` and `skills/skill-manager.md` restored to the prose versions from `82e525d`.
- **skill-manager builtins updated** to include `project-map`.

## Decisions And Rationale
- `project-map` is a skill, not runtime code — zero new Go dependencies or behavior beyond one placeholder injection.
- Auto-injection follows the existing `AGENTS.md` pattern: read from workdir, wrap in delimiter, inject via placeholder.
- No project-map.md → section renders `NULL` (same behavior as all other optional injectable sections).
- Compact-language was rolled back from prompt and skill files because the user preferred the natural-language prose format for those surfaces. The reversion was done via `git checkout 82e525d -- prompts/sysprompt.md skills/skill-manager.md`, then project-map changes were applied on top.
- Compact-language in Go code (tool descriptions, helper descriptions) remains untouched — this scope was only about .md files.

## Implementation Approach
- `internal/prompt/prompt.go:308-315`: reads `project-map.md` via existing `readFileOptional`, wraps with `---\nproject-map.md:\n\n...\n---` delimiters.
- `internal/prompt/prompt.go:337`: `PROJECT_MAP_CONTENT` added to the `injectTemplateVariables` map alongside the other injected placeholders.
- `prompts/sysprompt.md:132-133`: new `## Project Map` section with `{PROJECT_MAP_CONTENT}`.
- `skills/project-map.md`: builtin skill with `[DESCRIPTION]` triggers and `[BEHAVIOR]` workflow for generating the map.
- `skills/skill-manager.md:1`: builtin list includes `project-map`.

## Alternatives Considered
- A dedicated `diff` helper was considered and rejected. Shell tool already provides `diff -u`, `git diff`, `fc`. No cross-platform pain point reported.
- memory-bank — contradicts spec (DATA in skills is the memory mechanism).
- context-router, periodic-skill-review — require secondary LLM calls, conflict with speed priority.
- task-focused-summarization — identifying "active task" is fragile without a secondary LLM.

## Files Included
- `skills/project-map.md`: new builtin skill for generating project structure maps
- `skills/skill-manager.md`: reverted to natural language; added project-map to builtin list
- `prompts/sysprompt.md`: reverted to natural language; added `## Project Map` section
- `internal/prompt/prompt.go`: reads optional project-map.md from workdir, injects via `PROJECT_MAP_CONTENT`
- `internal/prompt/doc.go`: updated comment to include project-map.md in source order
- `AGENTS.md`: unrelated/pre-existing change included to leave the repository clean
- `skills/customize_me.md`: unrelated/pre-existing change included to leave the repository clean
- `skills/setup_helpers.md`: unrelated/pre-existing change included to leave the repository clean

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
