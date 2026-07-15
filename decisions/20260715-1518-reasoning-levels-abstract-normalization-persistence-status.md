# Session Decision Summary: reasoning levels with abstract normalization

Date: 2026-07-15 15:18

## Context

User requested provider-agnostic reasoning levels: the user sets only abstract levels (`none`, `min`, `low`, `med`, `high`, `xhigh`, `max`); internal normalization maps them to provider-specific wire formats. No fallbacks. Prior partial implementation mixed API-path names into user-facing config; this was corrected.

## Changes Made

### New files
- `internal/reasoning/` package (levels.go, descriptor.go, openai_chat.go, openai_responses.go, tests) — standard level constants, registry, model-aware normalization, capability checks, OpenAI Chat Completions and Responses/Codex wire format support with max→xhigh clamping.
- `internal/config/testmain_test.go`, `internal/console/testmain_test.go`, `internal/runtime/testmain_test.go`, `internal/telegram/testmain_test.go` — process-wide test HOME isolation to prevent test writes to live modes.json.
- `internal/web/renderer_test.go` — tests for agent tool line HTML with CTX.
- `todos/20260715T131721-[TODO]-reasoning_levels-openai-normalization-modes-persistence-tests.md` — adjusted TODO with clarified architecture.

### Modified files
- `internal/config/modes.go` — added `ReasoningLevels map[string]string`, `ReasoningLevelFor()`, `SetReasoningLevel()`.
- `internal/config/modes_test.go` — added reasoning level round-trip, nil-map, per-model isolation tests.
- `internal/runtime/runtime.go` — newAgent loads stored level; applyModel loads level on switch; `ActiveReasoningLevel()` getter; `SetActiveReasoningLevel()` setter with validation and persistence; AgentActivity carries LastPromptTokens.
- `internal/runtime/runtime_test.go` — tests for stored level loading, getter, setter, invalid level rejection.
- `internal/runtime/agent_orchestration.go` — childHandler tracks lastPromptTokens via OnUsage; passes to AgentActivity.
- `internal/runtime/agent_orchestration_test.go` — tests for childHandler CTX propagation.
- `internal/provider/provider.go` — ReasoningLevel field; Chat Completions request calls reasoning.Normalize.
- `internal/provider/openai_responses.go` — `resolveReasoningEffort()` calls reasoning.Normalize; buildChatGPTRequest/lite accept reasoningLevel parameter.
- `internal/provider/openai_responses_test.go` — updated to pass reasoningLevel.
- `internal/console/console.go` — dynamic reasoning level in status bar; removed hardcoded "medium"; CTX on agent tool_result lines.
- `internal/console/console_test.go` — tests for agent tool_result CTX display.
- `internal/telegram/handler.go` — CTX display on agent tool_result lines.
- `internal/web/handler.go` — CTX display on agent tool_result lines.
- `internal/web/renderer.go` — agentToolLineHTML accepts ctxTokens parameter.

## Decisions And Rationale

1. **User config exposes only abstract levels** — the user never sees `openai_chat` or `openai_responses`; those are internal normalization descriptors.
2. **Per-model persistence via ReasoningLevels map** — stored in modes.json as `{modelID: level}`, backward compatible with omitempty.
3. **max→xhigh clamping inside normalizer** — OpenAI APIs do not accept `max`; the normalizer maps it internally.
4. **Empty level defaults to "medium" in Responses path** — preserves current behavior for models without stored level, while explicit invalid levels still error.
5. **Test isolation via TestMain** — prevents any test from writing to the live app-home modes.json.
6. **Included pre-existing CTX propagation changes** — agent_orchestration, console, telegram, and web CTX display were already in the working tree and are logically complete.
7. **No fallback on explicit invalid level** — per project spec; silent degradation is forbidden.
