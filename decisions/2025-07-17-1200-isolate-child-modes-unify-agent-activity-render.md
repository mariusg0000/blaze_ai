# Session Decision Summary: isolate child modes unify agent activity rendering

Date: 2025-07-17 12:00

## Context

Child agents were still inheriting the parent mode configuration through `NewAgent`, which meant a `boss` mode could inject read-only directives and denied-tool policies into one-shot children. The fix was requested only for the child runtime, not for main-runtime mode behavior.

## Changes Made

- `internal/runtime/runtime.go` — extracted shared construction into `newAgent` and added `newChildAgent`, which creates a one-shot runtime with only the requested model and no mode state.
- `internal/runtime/agent_orchestration.go` — switched `runOneChild` to `newChildAgent` so children no longer load or apply parent modes.
- `internal/console/console.go` — replaced generic `OnSystem` child rendering with scoped agent tool activity, including a fresh line after `run_agent` and unified tool-line formatting.
- `internal/tools/agent_tools.go` — normalized `run_agent` fallback display to 80 characters.
- `internal/tools/shell.go` — normalized shell fallback display to 80 characters.
- `internal/tools/agent_tools_test.go` — updated `run_agent` fallback test.
- `internal/tools/tools_test.go` — updated shell fallback test.
- `internal/web/handler.go` — replaced generic system-style child rendering with agent-scoped tool/result HTML.
- `internal/web/renderer.go` — added `agentToolLineHTML` helper.
- `decisions/2025-07-17-1200-isolate-child-modes-unify-agent-activity-render.md` — added.

## Decisions And Rationale

- Child agents must not be constructed with `NewAgent`, which loads modes and applies mode policy for the main runtime. A dedicated child constructor preserves the spec that children inherit only the chosen model and explicit tool allowlist.
- The same pass included unified child activity rendering in both console and web, because child tool events were still styled as generic system messages.
- Fallback display lengths for `run_agent` and shell were normalized to 80 characters as previously requested for child activity consistency.
- No mode files, mode schema, or main mode behavior were changed, because the requirement was only to remove mode inheritance from agents.
