# TODO: Implement simple agent orchestration

## WHAT MUST BE DONE
Implement a simple agent system in which agent definitions are Markdown files under `{APP_HOME}/agents/` and the existing interactive work modes (`default`, `quick`, and `planning`) can be represented as interactive agent definitions.

### Agent definitions

- Add an `internal/agents/` package responsible for discovering, parsing, validating, and resolving agent definitions.
- Discover only `.md` files directly under `{APP_HOME}/agents/`.
- Add the `agents` directory to application-home bootstrap.
- Define Markdown metadata containing at least:
  - `name`;
  - `kind`, with `interactive` or `one-shot`;
  - optional `model`;
  - explicit `tools` allowlist.
- Treat the Markdown body as the agent's behavioral instructions.
- Reject duplicate names, invalid metadata, unknown kinds, malformed models, and unknown tools with explicit errors.
- Do not silently skip invalid agent definitions.

### Models

- Resolve an explicitly defined agent model exactly as configured.
- If a one-shot agent has no model, inherit the currently active model of its interactive parent.
- Capture the effective model at launch time.
- Reject missing or invalid configured models; never fall back silently.
- Keep interactive-agent model switching compatible with the existing runtime model and mode behavior during migration.

### Tool permissions

- Make tool access deny-by-default for every agent.
- Build a filtered tool registry for each runtime from the definition's explicit allowlist.
- Ensure a tool absent from the allowlist is not merely hidden from the prompt but unavailable in the registry.
- Validate that every declared tool exists.
- Preserve the existing tool implementations and reuse them through filtered registries.
- Add the internal `agent_done` tool automatically to one-shot child agents; it is not required in the user-defined allowlist.
- Prevent one-shot agents from recursively launching other agents in the first implementation unless explicitly designed and validated later.
- Keep write and execution tools explicit; do not inherit the parent's tools automatically.

### Interactive agents and mode migration

- Introduce a common agent-definition model for interactive agents and one-shot agents.
- Keep the current `modes.json` behavior temporarily for compatibility.
- Convert existing modes into equivalent interactive definitions in memory or through a controlled migration path.
- Preserve `Tab` cycling, active model selection, directives, and session resume behavior.
- Preserve existing `/model` and mode-related behavior until the new agent definitions fully replace the old mode storage.
- Make the interactive agent's definition control the main runtime's allowed tools.
- Replace `CurrentMode` with an agent-oriented concept only after compatibility coverage exists.

### One-shot execution

- Add a `run_agent` tool available only to interactive agents that explicitly declare it.
- Support launching one child agent with a task and context.
- Support launching multiple child agents in parallel from one orchestration request.
- Create one temporary child session folder per child under the parent session, with temporary `session.json`, prompt data, and summaries as needed.
- Give each child its own provider client, prompt builder, session, compactor, context, and filtered tool registry.
- Pass the requested task and context to the child without automatically copying the full parent transcript.
- Keep the child runtime independent until completion.
- Use a bounded concurrency limit and a per-agent timeout.
- Propagate parent cancellation to all running children.
- Preserve result ordering according to the requested task order when returning multiple results.

### Strict completion protocol

- Add the `agent_done` tool for one-shot children.
- Require the child to call `agent_done` with a non-empty final answer.
- Treat plain assistant text without `agent_done` as incomplete rather than successful completion.
- Stop the child tool loop after `agent_done`.
- Reject or ignore further tool calls after completion.
- Return the `agent_done` answer as the `run_agent` tool result to the interactive parent.
- Return explicit errors for timeout, cancellation, invalid completion, provider failure, and child tool failure.

### Temporary context and cleanup

- Treat child sessions as ephemeral tool execution state, not resumable sessions.
- Delete the entire child session folder after the child returns a result, error, cancellation, or timeout.
- Ensure cleanup runs on every path, including panics or early returns where practical.
- Do not retain child session files, summaries, prompt files, transcripts, event logs, or metadata after cleanup.
- Do not expose child session identifiers or introduce persistent agent IDs.
- Use only the agent name for user-facing activity display.
- Keep the parent tool result as the only persistent record of the child outcome.
- Define and test behavior when parent persistence fails: report the error explicitly while still removing ephemeral child data.

### Auto-compaction

- Reuse the existing compaction implementation for each child runtime with a separate compaction manager.
- Compact the child context independently from the parent context.
- Preserve raw child session data only until the child finishes or is cancelled.
- Keep summaries temporary and delete them with the child folder.
- Apply compaction before another child LLM call when provider usage or local estimation exceeds configured limits.
- Ensure the final answer returned through `agent_done` is bounded and suitable for insertion into the parent context.
- Do not inject the complete child transcript into the parent session.

### Activity display

- Extend the runtime-to-transport activity contract with agent-scoped events without introducing persistent agent IDs.
- Include at least the agent name, event kind, tool name when relevant, status, and text or phase.
- Support events for started, provider phase, tool started, tool finished, completed, failed, cancelled, and timed out states.
- Update the console to show readable agent-name prefixes, for example `[code-review]`.
- Prevent interleaved output from becoming ambiguous when children run in parallel.
- Adapt web SSE output to display child activity separately from the main assistant stream.
- Provide a compact Telegram status representation for multiple active children.
- Keep the first UI implementation simple; do not build a full live dashboard or persistent activity history.

### Validation

- Add parser and validation tests for valid, invalid, duplicate, missing-model, unknown-tool, and malformed definitions.
- Add model inheritance tests for explicit and inherited models.
- Add filtered-registry tests proving disallowed tools cannot execute.
- Add interactive-agent compatibility tests for existing modes and model switching.
- Add one-shot tests proving that `agent_done` is required and returns the final answer.
- Add parallel execution tests using mocked providers with different completion order.
- Add cancellation and timeout tests proving all children stop and temporary folders are removed.
- Add cleanup tests for success, failure, cancellation, timeout, and parent persistence failure.
- Add compaction tests proving child summaries are local and temporary.
- Add console, web, and Telegram activity tests for multiple child agents.
- Run `go test ./...` and `go build ./...` after implementation.

## WHY IT MUST BE DONE
BlazeAI needs reusable child agents that can be called like tools, execute independently with explicitly limited capabilities, and return a clear result to the main interactive agent. The current runtime supports one agent with a single session and sequential tool execution, while the existing work modes separately control the main model and directive; a unified definition model can reduce duplication without making one-shot agents persistent.

Child sessions must be temporary because these agents are not resumable conversations and must leave no files after completion or interruption. A strict `agent_done` protocol is needed so the runtime can distinguish a completed task from an unfinished assistant response. Per-child compaction is needed to support long child tool loops without contaminating the parent's context, while explicit tool allowlists are required to prevent accidental access to tools that the child was not authorized to use.

Parallel execution also requires agent-scoped activity events because the existing callbacks do not identify which runtime produced output. The implementation must remain simple, avoid persistent child IDs and over-engineered orchestration, and preserve explicit failure behavior required by the project specification.

## HOW IT MUST BE DONE
Implement incrementally, starting with agent definitions and filtered tool registries before adding parallel execution. Keep the current `modes.json` path working during the transition and migrate only after interactive-agent behavior has equivalent test coverage.

Use a separate child runtime assembled from the existing provider, prompt, session, compaction, and tool components. The child must receive only its task, explicit context, agent instructions, and permitted tools. The parent model remains responsible for orchestration; `run_agent` blocks until the child calls `agent_done` or fails, times out, or is cancelled.

Create temporary child state inside the parent session using a unique filesystem temporary directory, but do not expose that directory name as an agent identifier. Register cleanup immediately after creation and remove the complete directory on every terminal path. The parent stores only the bounded `run_agent` result in its normal session history.

Use a bounded parallel execution group with shared cancellation and per-child timeout. Return multiple results in request order, while emitting activity events as children actually progress. Add the smallest event extension needed by console, web, and Telegram transports; use agent names for display and do not build a persistent event store.

Keep `agent_done` as an internal protocol tool that is automatically available only to one-shot children. Require a non-empty answer, mark any child that exits without the tool as incomplete, and make all failure states explicit. Reuse the current compaction manager per child, write summaries only into the temporary child folder, and delete them with the rest of the child state.

Before implementation, confirm the exact front-matter syntax and whether `shell` is allowed for read-only agents. During implementation, document all new packages, types, methods, and non-trivial orchestration logic according to `AGENTS.md`, update the project specs and project map in the same patch, and do not commit unless explicitly requested.