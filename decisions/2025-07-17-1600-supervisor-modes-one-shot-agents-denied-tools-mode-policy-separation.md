# Session Decision Summary: Supervisor Modes and One-Shot Agent Separation

Date: 2025-07-17 16:00

## Context

The initial implementation incorrectly converted persisted modes (quick/default/planning) into in-memory interactive agent definitions. This conflated two distinct concepts: operational modes and delegated sub-agents. A planning supervisor mode needs read-only direct tools while delegating implementation to independently permissioned workers.

## Changes Made

- Removed `internal/runtime/agent_mode_compat.go` — modes are no longer represented as agents.
- Removed `internal/runtime/agent_capabilities.go` — obsolete interactive-agent capability logic.
- Added `internal/runtime/mode_capabilities.go` — applies config/modes.json `denied_tools` to direct runtime tools and limits `run_agent` to explicitly allowed one-shot agents.
- Extended `internal/config/config.go` Mode struct with `DeniedTools` and `Agents` fields.
- Extended `internal/config/modes.go` with `validateModeNames` for empty/duplicate list validation.
- Updated `internal/agents/agents.go` to remove `KindInteractive`; all agents are now `one-shot`.
- Updated `internal/agents/agents_test.go` with one-shot-only fixtures and corrected error expectations.
- Updated `internal/config/modes_test.go` with denied_tools and agents round-trip coverage.
- Updated `internal/runtime/runtime.go` to use `refreshModeCapabilities` instead of `refreshInteractiveTools`.
- Updated `internal/runtime/agent_orchestration.go` with mode-level agent allowlist enforcement at `run_agent` boundary.
- Updated `specs.md` to document modes as the sole interactive abstraction and agents as one-shot children.
- Updated `modes.json` with explicit `denied_tools` for planning and `agents` lists for all modes.

## Decisions And Rationale

- **Modes are the only interactive runtime abstraction**: quick, default, planning live exclusively in `config/modes.json`. They own model selection, direct-tool policy, and allowed sub-agents.
- **All Markdown agents are one-shot**: `KindInteractive` is removed. Agent definitions serve only as delegated child runtimes. This eliminates ambiguity between modes and agents.
- **Mode denied_tools apply only to the main runtime**: planning cannot call shell or write_file directly, but its delegated worker retains full tool access through its own allowlist. This is by design: the supervisor delegates implementation, not execution constraints.
- **Mode agents list controls delegation**: `run_agent` is rejected at the orchestration boundary when the requested agent is not in the current mode's `agents` list. This prevents unlisted agents from being called.
- **Child permissions are independent**: `child_tools = agent_definition.tools`, not `agent_definition.tools ∩ parent_mode.tools`. A planning supervisor delegates to a worker that can execute; the worker's tool allowlist is its own authority.
- **Backup preserved**: existing `modes.json` and obsolete source files backed up under `/home/marius/blazeai/backups/2025-07-17-mode-capabilities/`.
