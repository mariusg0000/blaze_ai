# Session Decision Summary: Agent System Prompt and Case-Insensitive Project File Lookup

Date: 2025-07-15 05:15

## Context

The session covered three related changes requested across multiple turns:
1. Child agents used the full main runtime system prompt, including transport, communication protocol, agents catalog, and secondary model sections irrelevant to one-shot agents.
2. Agent-definition instructions from Markdown body were injected as normal user messages (`[AGENT INSTRUCTIONS]`), making them vulnerable to context compaction summarization.
3. `specs.md` and `AGENTS.md` lookup was case-sensitive.

## Changes Made

- `prompts/sysprompt.agent.md` — New simplified universal prompt for one-shot agents. Includes identity, environment, safety (no elevated commands), OS prompt, output style, skills, host helpers, `{AGENTS_CONTENT}`, `{PROJECT_CONTENT}`, and execution rules with `agent_done`. Omits transport, communication protocol, secondary model consultation, agents catalog, and project rules section header.
- `internal/prompt/prompt.go` — Added `SystemPromptName` and `AgentInstructions` fields to Builder. Builder selects prompt file by name. Transport section skipped when agent profile active. `{AGENT_INSTRUCTIONS}` injected as allowed-empty template variable. Added `readProjectFileOptional` for case-insensitive `specs.md` and `AGENTS.md` lookup with ambiguous-duplicate rejection. Main runtime uses `sysprompt.md`; child agents use `sysprompt.agent.md`.
- `internal/runtime/runtime.go` — Main runtime explicitly sets `SystemPromptName: "sysprompt.md"`.
- `internal/runtime/agent_orchestration.go` — Child agent builder uses `sysprompt.agent.md` profile. `definition.Instructions` assigned to `Builder.AgentInstructions` (injected into system prompt on every rebuild). `Builder.Agents` set to nil (no agents catalog for children). Instructions removed from `buildChildInput`.
- `internal/prompt/prompt_test.go` — Added `TestReadProjectFilesCaseInsensitive` regression test for mixed-case `SpEcS.MD` and `aGeNtS.mD` lookup.

## Decisions And Rationale

- **Agent instructions in system prompt, not user messages** — Prevents loss during compaction/summarization. The instructions persist at every context rebuild as part of the system role.
- **Included `AGENTS.md` and `specs.md` in agent prompt** — `AGENTS.md` contains project coding rules, documentation conventions, and testing requirements that agents need. `specs.md` provides technology stack and architecture context. Both are authoritative inputs.
- **Case-insensitive project file lookup with ambiguity rejection** — Users and platforms create files with varying casing (`agents.md`, `AGENTS.md`, `Agents.MD`). EqualFold matching handles this; rejecting duplicates prevents silent wrong-file reads.
- **No elevated commands in agent prompt** — Cross-platform safety rule; agents should never require sudo or equivalent.
- **Transport skipped for agent profile** — Transport prompts are REPL/Telegram/web-specific and irrelevant to child agents that never interact with a transport directly.
