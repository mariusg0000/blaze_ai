# Session Decision Summary: agent orchestration plan

Date: 2026-07-14 13:20

## Context

User requested analysis and implementation plan for file-defined agents with explicit tool permissions, parallel execution, temporary child sessions, and visible activity display.

## Changes Made

- Created detailed TODO for agent orchestration at `todos/20260714T132042-[TODO]-agent_orchestration-one_shot-tool_permissions-parallel-execution-compaction.md`.

## Decisions And Rationale

- Agent definitions are `.md` files under `{APP_HOME}/agents/` with explicit `name`, `kind` (interactive/one-shot), `model`, and `tools` allowlist.
- Tool access is deny-by-default; tools absent from allowlist are unavailable in the registry, not merely hidden.
- One-shot agents use `agent_done(answer)` as the strict completion protocol; plain assistant text without `agent_done` is incomplete.
- Child sessions are ephemeral: temporary folder created per agent run, deleted entirely after completion, error, cancellation, or timeout.
- No persistent agent IDs, no resume, no history retained for one-shot children.
- Existing work modes (`default`, `quick`, `planning`) migrate to interactive agent definitions; `modes.json` kept temporarily for compatibility.
- `run_agent` is available only to interactive agents that explicitly declare it.
- Auto-compaction is per child with separate compactor; summaries are temporary.
- `ask_main` bidirectional communication is deferred; `agent_done(status=needs_clarification)` with re-launch is the interim approach.
- Parallel execution bounded by configurable concurrency limit.
