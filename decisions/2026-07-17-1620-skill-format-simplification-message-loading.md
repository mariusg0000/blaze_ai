# Decision Summary: Skill Format Simplification and Message-Based Loading

Date: 2026-07-17 16:20

## Context

User requested simplification of the skill system: eliminate the `ActiveList` mechanism, `SKILLS_ACTIVE` prompt injection, and `unload_skill` tool. Replace with a two-section format (`[DESCRIPTION]` + `[BODY]`) where loaded skill bodies are returned as standard tool messages in conversation history.

## Changes Made

**Core implementation (627 lines net removed):**
- `internal/skills/skills.go` — Removed `ActiveList`, `ErrMissingBehavior`, `ErrMissingData`. Parse requires `[DESCRIPTION]` + `[BODY]`. Added `ErrMissingBody`. Removed `SortedNames`.
- `internal/skills/doc.go` — Updated package documentation.
- `internal/skills/skills_test.go` — Rewritten tests for new format.
- `internal/tools/skill_tools.go` — Removed `UnloadSkillTool`. `LoadSkillTool` uses `LoadSkillFunc` returning body directly.
- `internal/tools/skill_tools_test.go` — Rewritten tests.
- `internal/tools/tools.go` — Fixed comment.
- `internal/tools/tools_test.go` — Updated tool count to 9.
- `internal/runtime/runtime.go` — Removed `ActiveList` from Agent struct, removed `UnloadSkillTool` registration, simplified `load_skill` closure.
- `internal/runtime/runtime_test.go` — Removed `ActiveList` references.
- `internal/prompt/prompt.go` — Removed `{SKILLS_ACTIVE}`, `buildActiveSkillsSection`, removed `activeSkills` param from `BuildRuntimePart`. Added `RenderSkillBody`.
- `internal/prompt/prompt_test.go` — Rewritten for new interface.
- `internal/console/console.go` — Removed `OnSkillsChange` callback.
- `internal/console/console_test.go` — Removed skill-change tests.
- `internal/telegram/handler.go` — Removed `OnSkillsChange` callback.
- `internal/telegram/handler_test.go` — Removed skill-change test.
- `internal/telegram/commands_test.go` — Fixed assertion.

**Prompts:**
- `prompts/sysprompt.md` — Removed `{SKILLS_ACTIVE}`, updated skill section.
- `prompts/sysprompt.agent.md` — Same.
- `prompts/readme.md` — Updated.

**Embedded skills (6 files):** Migrated to `[DESCRIPTION]` + `[BODY]`.
**Backup skills (11 files):** Migrated to `[DESCRIPTION]` + `[BODY]`.
**Specs (9 files):** Updated for new format, removed `ActiveList`/`SKILLS_ACTIVE`/`unload_skill` references.
**`README.md`:** Created with project overview.

## Decisions And Rationale

- **Two-section format**: `[DESCRIPTION]` stays in system prompt for discovery. `[BODY]` is loaded once as a tool message, persisted like any other tool result. No reinjection, no active state.
- **No `unload_skill`**: Loaded bodies are ordinary conversation history; no special state to undo.
- **`RenderSkillBody`**: Applied at load time to expand variables like `{SKILL_DIR}` before the body enters conversation.
- **Discovery strictness**: Malformed skills (`[BEHAVIOR]`/`[DATA]` or missing sections) stop discovery with an error. No fallback.
- **`read_file`/`write_file` retained**: `read_file` enables read-only agent boundaries. `write_file` is redundant with shell but kept for now; removal deferred.
