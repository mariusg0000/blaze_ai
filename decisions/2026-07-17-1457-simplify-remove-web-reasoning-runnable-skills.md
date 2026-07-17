# Decision Summary: Remove web transport, reasoning system, and runnable skills

Date: 2026-07-17 14:57

## Context

User requested a major simplification of the codebase before further optimization work. The goal was to strip the application down to its essential core: console and Telegram transports only, plain model IDs with provider-default behavior, and prompt-based skills only. No fallbacks or migration logic was permitted.

## Changes Made

### Web transport removal
- Deleted `internal/web/` package (7 files: server, handler, commands, page, renderer, tests)
- Deleted `prompts/transport.web.md`
- Removed `-web` CLI flag, `runWeb()`, web import from `main.go`
- Removed all web references from prompts, specs, and test fixtures

### Reasoning system removal
- Deleted `internal/reasoning/` package (9 files: descriptors, levels, model_spec, normalizer, OpenAI transforms, tests)
- Removed `Client.ReasoningLevel`, suffix parsing, and explicit effort/summary from provider
- Removed `ActiveReasoningLevel()`, `SetActiveReasoningLevel()` from runtime
- Removed console reasoning status, Ctrl+] cycle, Ctrl+T toggle, `OnReasoning` display
- Removed `ShowReasoning`, `ReasoningMaxHeight` from config
- Model IDs containing `|` are now explicitly rejected
- Retained: opaque `OnReasoning` handler compatibility, session `reasoning_content`, `StripReasoningFromPayload`, Responses Lite `all_turns` context

### Runnable skills removal
- Removed `IsRunnable()`, `HasPromptContent()`, `ErrMixedSkillTypes`, code-fence parsing from skill parser
- Deleted `RunSkillTool`, `RunSkillArgs`, `runnableSkillResolver` from tools and runtime
- Removed `{RUNNABLE_SKILLS_SECTION}` from prompt templates and builder
- Removed `run_skill` icon mappings from console and Telegram
- Simplified `LoadSkillTool` description to ordinary skills only
- Rewrote `skills/skill-manager.md` as loadable-only

### Prompt and spec updates
- All sysprompt templates: removed `{RUNNABLE_SKILLS_SECTION}`
- Transport prompts: removed reasoning visibility bullets
- `specs.md` and 12 spec files: removed web, reasoning controls, runnable skills
- `prompts/readme.md`: removed `transport.web.md` reference

### Test cleanup
- Removed `transport.web.md` fixtures from all test suites
- Removed reasoning-cycle, reasoning-level, and run_skill tests/assertions
- Updated prompt test fixtures to match current templates

## Decisions And Rationale

- **Tag before deletion**: `pre-simplification-transport-reasoning` created for revert safety.
- **No automatic suffix stripping**: Old `model|level` configs fail with explicit error per no-fallback rule.
- **Opaque reasoning compatibility retained**: Provider response capture, `reasoning_content` serialization, and `StripReasoningFromPayload` remain because the API may emit them independently of user controls.
- **Responses Lite `all_turns` preserved**: Required by backend contract even without explicit effort.
- **`OnReasoning` kept as no-op**: Handler interface requires it; console no longer displays reasoning.
- **No web migration**: Web transport deleted outright; it was opt-in and not in active scope.
- **Runnable skills fully removed**: Shell tool provides equivalent capability; prompt skills simplify the model's tool surface.

## Validation

- `go test ./...` — PASS
- `go build ./...` — PASS
- `git diff --check` — PASS
- Active grep for removed tokens — PASS
