# Session Decision Summary: Agent Orchestration

Date: 2025-07-17 14:27

## Context

The runtime lacked Markdown-based agent definitions, one-shot child execution, explicit tool permissioning, and any visible progress during child-agent runs. The user requested a complete orchestration system with strict protocols, chronological interleaved activity, and compatibility with the existing quick/default/planning modes.

## Changes Made

- Added `internal/agents/agents.go` — strict Markdown agent discovery, parsing, and validation.
- Added `internal/runtime/agent_orchestration.go` — ephemeral one-shot execution with bounded parallelism, model inheritance, temporary cleanup, ordered results, and scoped live tool activity.
- Added `internal/runtime/agent_capabilities.go` — interactive agent tool filtering on mode switch.
- Added `internal/runtime/agent_mode_compat.go` — converts modes.json entries into in-memory interactive agent definitions.
- Added `internal/tools/agent_tools.go` — run_agent and agent_done tool adapters with required purpose field and 150-character task fallback.
- Added `internal/agents/agents_test.go` and `internal/tools/agent_tools_test.go` — validation and display tests.
- Extended `internal/tools/tools.go` with Filter(), Remove(), and Clone() registry methods.
- Extended `internal/prompt/prompt.go` with AGENTS section rendering (descriptions, kinds, models, permissions, run/completion instructions).
- Extended `internal/runtime/runtime.go` with definition loading, BaseTools retention, capability refresh, and mode compat wiring.
- Extended `internal/config/config.go` with ValidateModelFormat helper.
- Extended `internal/platform/platform.go` with agents/ bootstrap.
- Extended console, web, and Telegram handlers with OnAgentActivity for chronological child tool visibility.
- Updated `prompts/sysprompt.md` with [AGENTS] section.
- Updated `specs.md` with agent orchestration documentation.

## Decisions And Rationale

- **Required description field**: Every agent definition must include a non-empty description to ensure the AGENTS prompt section always contains routing information.
- **Explicit tool allowlist**: `shell` is not implicit; every tool including `shell` must be declared in the tools list. `agent_done` is added automatically for one-shot agents; `run_agent` is prohibited for one-shot children.
- **Mode compatibility as in-memory definitions**: quick/default/planning are represented as interactive agent definitions during migration, preserving existing persistence, Tab cycling, and directive behavior.
- **Deny-by-default run_agent**: The tool is registered only when a matching Markdown interactive agent or compatibility mode explicitly declares it; legacy modes receive it only through their allowlist.
- **Chronological interleaved display**: Child tool-call and tool-result events are emitted immediately through the shared transport handler. Events appear in arrival order without timestamps or reordering. Final run_agent result preserves original task order.
- **Transient display IDs**: Each child receives a display-only ID like explore_01; these are never persisted or used as session identifiers.
- **Required purpose field**: run_agent follows the shell/replace_block convention with a three-sentence purpose; when absent, falls back to task truncated to 150 characters.
- **Child sessions are temporary**: Created under the parent session folder and removed by deferred cleanup regardless of success or failure.
- **No agent_done in parent**: The parent runtime stops the child turn immediately after agent_done succeeds, preventing any provider follow-up.
