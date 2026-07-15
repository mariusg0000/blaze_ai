# Session Decision Summary: persistent child sessions agent_task_md

Date: 2025-07-17 12:45

## Context

Child-agent tasks were stored as conversation messages that could be lost through context compaction. The current task must survive compaction, persist across resume, and be replaced cleanly on each run_agent call.

## Changes Made

- `internal/runtime/agent_orchestration.go` — Child sessions are now persistent under `<main-session>/agents/<id>/` instead of temporary `agent-*` folders. Each run writes `agent_task.md` into the child session folder. Resume with `id` reloads the existing session and replaces `agent_task.md` before the next context build. Child result now includes the session id for later resume. Added `openChildSession` and `validChildID` helpers.
- `internal/prompt/prompt.go` — Added `AgentTaskFile` to `Builder`. On every `BuildRuntimePart`, the task file is read, validated non-empty, and injected as `{AGENT_TASK}` into the system prompt. The template value is allowed to be empty (for main-runtime prompts where it is not set).
- `prompts/sysprompt.agent.md` — Added `[CURRENT TASK]` / `{AGENT_TASK}` section between agent instructions and environment.
- `internal/tools/agent_tools.go` — Added `ID` to both `RunAgentArgs` and `RunAgentTask` for optional persistent child-session identification.
- `internal/prompt/prompt_test.go` — Added `TestBuildRuntimePartLoadsAgentTask` verifying that `agent_task.md` is read into every prompt build and that disk updates take effect on the next build.
- `specs.md` — Updated docs to reflect persistent child sessions, `agent_task.md`, resume with id, and the 80-character fallback.

## Decisions And Rationale

- Task content is a system-prompt artifact, not a conversation message, so compaction cannot remove it.
- Each child session id is validated to reject path traversal, slashes, and traversal components.
- The child session id is auto-generated when not supplied by the main agent, ensuring backward compatibility with existing run_agent calls that omit it.
- No commit or push behavior was changed; child agents still receive only agent_done for completion.
