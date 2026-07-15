# Session Decision Summary: boss mode child display resume protocol

Date: 2025-07-17 13:15

## Context

Multiple improvements to child-agent orchestration, mode configuration, and prompt hygiene were requested in one session.

## Changes Made

- `/home/marius/blazeai/config/modes.json` — Added boss mode with read-only directive, denied write tools, and coder agent access. Set as last_mode.
- `internal/runtime/agent_orchestration.go` — Short 5-character hex child IDs via shortChildID(). Display format changed to [agent_<id>]. Model name included in started event. openChildSession fixed to use childID for folder path. Result includes childID for resume.
- `internal/tools/agent_tools.go` — run_agent description explicitly mentions resume-with-id protocol.
- `internal/prompt/prompt.go` — AGENTS.md instructions updated with explicit resume protocol: preserve child session id, pass it back on resume, never invent ids.
- `prompts/sysprompt.agent.md` — Removed AGENTS.md/AGENTS_CONTENT section entirely; child agents only receive specs.md via PROJECT_CONTENT.

## Decisions And Rationale

- Child-agent IDs limited to 5 hex characters for compact display; folder and display are separate concerns.
- Boss mode denied write_file, replace_block, task_write only; shell remains available for read-only commands and agent delegation.
- AGENTS.md removed from child sysprompt because it contains commit workflow, interaction rules, and project conventions irrelevant to one-shot child execution.
- Resume protocol documented both in tool description and in main-agent prompt instructions so the LLM knows to preserve and reuse child session IDs.
