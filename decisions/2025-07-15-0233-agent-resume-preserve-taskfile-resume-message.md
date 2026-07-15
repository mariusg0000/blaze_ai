# Session Decision Summary: Agent resume preserves original task

Date: 2025-07-15 02:33

## Context

User reported that agent resume was replacing `agent_task.md` with the new task, which lost the original task context. The correct behavior is to preserve the original task in the system prompt and deliver the new task as a resume message to the child agent.

## Changes Made

- `internal/runtime/agent_orchestration.go` — `runOneChild` now writes `agent_task.md` only for new sessions; `openChildSession` returns a `resumed` bool; `buildChildInput` produces `[RESUME TASK]` message for resumed children.
- `internal/prompt/prompt.go` — Updated execution instructions to reflect preserved task and resume message behavior.
- `internal/tools/agent_tools.go` — Updated `id` schema description in run_agent to match implemented resume semantics.
- `specs.md` — Updated persistence and protocol rules to document preserved task behavior.
- `internal/runtime/agent_orchestration_test.go` — Added `TestOpenChildSessionMarksExistingSessionAsResumed`, `TestBuildChildInputUsesResumeMessage`, and `TestFormatChildErrorIncludesResumeMetadata`.
- `internal/provider/openai_oauth_test.go` — Minor gofmt whitespace alignment (pre-existing).
- `internal/runtime/runtime_test.go` — Minor gofmt whitespace alignment (pre-existing).

## Decisions And Rationale

- Original task preserved: The system prompt reads `agent_task.md` on every LLM call, so the original task remains visible. The resume task is a separate user message, giving the child clear context about what changed.
- Resume message uses `[RESUME TASK]` tag: Explicit tag avoids ambiguity between the original task and new instructions.
- `openChildSession` returns resumed flag: Clean separation between new and resumed session code paths without duplicating session opening logic.
- Tool schema updated: Parent model needs accurate metadata to construct correct resume calls.
