---
status: completed
---

# Interactive and executor agents

## User outcome

Replace the current mode/one-shot split with an OpenCode-like agent model.
Markdown definitions use `type: interactive` or `type: executor`. Interactive
agents are the main-session identity, selectable with Tab or `/mode <agent
name>`. Executor agents are launched as delegated children. Each interactive
agent persists its selected model. Reasoning-level persistence is explicitly a
future step and is not implemented here. Agent descriptions, front-matter
parameters, and Markdown bodies remain; bodies are injected into the system
prompt. Only interactive agents have `directive`, injected ephemerally into the
latest user message.

## Current state and evidence

- `internal/agents/agents.go` currently parses `kind: one-shot` into
  `Definition.Kind`, rejects every other kind, has `ToolNames`, `Instructions`,
  and no directive or executor-list field.
- `modes.json` currently stores `Mode{Name, Model, Directive, DeniedTools,
  Agents}` and `LastMode`; Tab cycles modes, but `/mode <name>` does not exist.
- Existing mode directives are already volatile user-message injection.
- Executor child execution, explicit child tool allowlists, persistent child
  sessions/resume, parallel dispatch, ordered results, and `agent_done` already
  work and must remain working.
- The user's approved decisions for this task are: old `kind` is replaced by
  `type`; `denied_tools` becomes a `tools` allowlist; legacy modes are migrated
  automatically to Markdown interactive agents; legacy `agents` becomes an
  executor allowlist on each interactive definition; state is stored in
  `config/agents.json`; reasoning levels are deferred.

Relevant reports:

- `.agents/explorer/260724-agent-model-inventory.md`
- `.agents/explorer/260724-agent-interaction-flow.md`
- `.agents/explorer/260724-agent-orchestration-detail.md`
- `.agents/explorer/260724-agent-capabilities-detail.md`
- `.agents/explorer/260724-agent-call-sites.md`
- `.agents/explorer/260724-orchestration-requirements.md`

## Resolved data model

### Markdown definition

`internal/agents.Definition` keeps `Name`, `Description`, `Model`, `Timeout`,
`ToolNames`, `Instructions`, and `Path`, renames `Kind` to `Type`, and adds:

- `TypeInteractive = "interactive"`
- `TypeExecutor = "executor"`
- `Directive string` (valid only for interactive definitions)
- `ExecutorNames []string` parsed from front-matter `agents:` (valid only for
  interactive definitions)

Front matter keys are exactly `name`, `description`, `type`, `model`, `timeout`,
`tools`, `agents`, and `directive`. The old `kind` key and `one-shot` value are
not accepted by the final parser. `tools` remains an explicit allowlist and is
required for both types. Interactive definitions require a model; executor
models remain optional and inherit the parent model as today. Interactive
definitions may list executor names; executor definitions may not list agents
or a directive. Every listed tool must exist, every interactive executor name
must resolve to a loaded executor definition, and duplicate names/tools remain
errors. `agent_done` remains runtime-injected for children and is not required
in Markdown. `run_agent` remains forbidden in executor definitions.

The Markdown body remains `Definition.Instructions` and is injected through the
existing `AGENT_INSTRUCTIONS` system-prompt variable for the active interactive
agent and executor child.

### Persistent state

Create `internal/config/agents.go` with:

```go
type InteractiveAgentState struct {
    Name  string `json:"name"`
    Model string `json:"model"`
}

type AgentsConfig struct {
    Agents   []InteractiveAgentState `json:"agents"`
    LastAgent string                  `json:"last_agent,omitempty"`
}
```

Persist it atomically at `<app-home>/config/agents.json`. State contains only
the selected model and active name; no reasoning-level field is added yet.
Validation keeps names unique, models valid, and `LastAgent` non-empty and
referencing an interactive definition once runtime resolves definitions.

### Runtime identity and capability flow

Replace `Agent.Modes`/`CurrentMode` with `Agent.AgentStates` and
`Agent.CurrentAgent *agents.Definition`. Keep `Definitions` as the complete
loaded set. Add `InteractiveDefinitions` only if needed for deterministic Tab
cycling; it must contain only interactive definitions in loader order.

`NewAgent` performs, in order: create the base registry; migrate legacy files;
load definitions; load or initialize `agents.json`; resolve the persisted active
interactive definition and its persisted model; construct the agent; filter
tools and visible executor definitions for that interactive definition; and
inject its body into `Builder.AgentInstructions`. No interactive definition or
state may silently fall back to a mode or global model.

`SetAgent(name)` selects only an interactive definition, applies its persisted
model, updates `CurrentAgent`, `Builder.AgentInstructions`, filtered tools and
visible executor list, writes `LastAgent`, and saves `agents.json`. Unknown or
executor names return an error. `NextAgent()` cycles only interactive
definitions and calls `SetAgent`. Console Tab calls `NextAgent`; `/mode
<agent-name>` calls `SetAgent` and refreshes the status bar.

`SetModel` applies the model, then updates the current interactive agent's state
and saves `agents.json`. `SetModelLocal` remains in-memory only. Favorite-model
cycling continues to call `SetModel`. No reasoning-level behavior is added.

Capability filtering uses the current interactive definition's `tools`
allowlist. The `agents` front-matter list limits visible/callable executor
definitions. If at least one allowed executor exists, `run_agent` is
automatically included in the interactive tool registry; it is not required in
the Markdown `tools` list. Executor children continue to receive only their
definition's `ToolNames` plus `agent_done`, with no recursive delegation.

`RunTurn` injects `[AGENT DIRECTIVE]` plus the current interactive definition's
directive into the latest user message in the outgoing payload only. It never
mutates or persists the session message. Executor definitions have no directive
injection.

### Legacy migration

Before final definition loading, migrate old user files:

1. Replace exact legacy front-matter `kind: one-shot` with
   `type: executor` in existing Markdown definition files. Any other legacy
   kind is an error.
2. If `config/agents.json` is absent and `config/modes.json` exists, load the
   old modes. For every mode, if `<app-home>/agents/<mode-name>.md` is absent,
   generate a `type: interactive` definition with:
   - `name` equal to the mode name;
   - `description: Migrated from mode <mode-name>`;
   - `model` equal to the old mode model;
   - `tools` equal to all base native tools except old `denied_tools`, excluding
     `agent_done` and `run_agent`;
   - `agents` equal to old mode `Agents`;
   - `directive` equal to old mode directive, with embedded newlines converted
     to spaces because the supported front matter is single-line;
   - an empty body.
   Existing same-name Markdown definitions are never overwritten; they must be
   interactive or loading fails with a collision error. Executor references
   and generated definitions are validated after migration.
3. Create `agents.json` state from old mode names/models and `LastMode` renamed
   to `LastAgent`. If there is no legacy state, initialize state from the
   sorted loaded interactive definitions and persist it. If no interactive
   definition exists, return a relevant error instructing the user to create
   one; do not silently use a global/default model.

Keep `config/modes.go` and its structs only as a legacy reader during this
migration; no runtime path may continue using modes after initialization.

## Exact write allowlist

### Stage 1 — definitions, migration, and state (sequential)

- `internal/agents/agents.go`: modify
- `internal/agents/agents_test.go`: modify
- `internal/agents/migration.go`: create
- `internal/agents/migration_test.go`: create
- `internal/config/agents.go`: create
- `internal/config/agents_test.go`: create

### Stage 2 — runtime, prompt, console, startup, and dependent tests (after Stage 1)

- `internal/runtime/runtime.go`: modify
- `internal/runtime/agent_orchestration.go`: modify
- `internal/runtime/mode_capabilities.go`: delete
- `internal/runtime/agent_capabilities.go`: create
- `internal/runtime/runtime_test.go`: modify
- `internal/runtime/agent_orchestration_test.go`: modify
- `internal/runtime/agent_capabilities_test.go`: create
- `internal/prompt/prompt.go`: modify
- `internal/prompt/prompt_test.go`: modify
- `internal/console/console.go`: modify
- `internal/console/console_test.go`: modify
- `firstrun.go`: modify
- `firstrun_test.go`: modify
- `internal/telegram/commands_test.go`: modify (dependent fixture correction)

The legacy config files under `internal/config/modes.go` and its tests are
read-only dependencies in this task. No specs, decisions, Telegram production
files, or unrelated paths may change. The Telegram test path above is the sole
test-only exception required by the full-suite failure.

## Required tests and assertions

Stage 1:

- Parse valid interactive and executor definitions, including `directive`,
  `agents` list, body, tools, model, and timeout.
- Reject old `kind`, unsupported `type`, interactive without model, executor
  with directive/agents, duplicate tools/names, unknown tools, unknown executor
  references, `run_agent` in executor, and invalid timeout/model.
- Round-trip `AgentsConfig` and verify atomic save/load, duplicate state names,
  invalid model, and invalid last agent errors.
- Migrate a legacy executor file in place and assert it becomes `type: executor`.
- Migrate a legacy mode into an interactive Markdown file and assert converted
  tools, executor list, directive, model, description, and empty body; assert an
  existing same-name file is not overwritten and collision errors are returned.

Stage 2:

- Update runtime construction tests to use interactive/executor definitions and
  `AgentsConfig`.
- Assert persisted active agent/model loading, `SetAgent`, `NextAgent` wraparound,
  `/mode <name>`, unknown/non-interactive selection errors, and per-agent model
  persistence.
- Assert interactive tool allowlists and executor visibility, automatic
  `run_agent`, child allowlist plus `agent_done`, and no recursive delegation.
- Assert `RunTurn` injects `[AGENT DIRECTIVE]` only into the outgoing latest user
  message and leaves the session message unchanged; assert executor directives
  are not injected.
- Assert prompt output uses `type: executor`, exposes only the current agent's
  allowed executors, and injects the active agent body into the system prompt.
- Assert console Tab cycles interactive agents only, `/mode` selects by exact
  name, status shows the current agent, and existing model commands persist to
  the selected agent.
- Preserve existing child parallel execution, ordered results, resume, and
  persistent-session tests; update only names/contracts required by the model
  change.
- Update the Telegram command-test agent fixture only so the full suite can
  construct the new runtime agent; do not change Telegram behavior.

## Verification commands

Stage 1 coder:

1. `go test ./internal/agents`
2. `go test ./internal/config`

Stage 2 coder:

1. `go test ./internal/runtime`
2. `go test ./internal/prompt`
3. `go test ./internal/console`
4. `go test ./...`

Each command must run after the latest relevant write and return one concise
`PASS`, `FAIL`, or `NOT RUN` result. A correction reruns every affected check.

## Acceptance criteria

- `type: interactive` and `type: executor` are the only accepted definition
  types; old `kind: one-shot` is migrated before final parsing, not accepted as
  a steady-state syntax.
- Interactive agents replace active modes for selection, prompts, tools,
  directives, and model persistence.
- Executor agents preserve current child execution semantics.
- Legacy modes are migrated automatically without overwriting existing agent
  files; failures are explicit and relevant.
- Reasoning-level persistence is absent and deferred exactly as requested.
- All changed files are inside the allowlist and all verification commands pass.

## Stop conditions

Stop before expanding the allowlist if migration needs a new persisted field,
reasoning-level behavior, executor directive behavior, or a changed child
session lifecycle. Do not add fallbacks, compatibility aliases, global model
state, or unrelated UI/transport changes.

## Completion

- Accepted outcome: interactive/executor Markdown agents replace active runtime
  modes; Tab and `/mode <agent-name>` select interactive agents; model state is
  persisted per interactive agent in `config/agents.json`; legacy definitions
  and modes migrate automatically; interactive directives remain ephemeral;
  reasoning levels remain deferred.
- Changed paths: all Stage 1 and Stage 2 paths listed above, plus the Telegram
  command-test fixture correction in `internal/telegram/commands_test.go`.
- Verification: `go test ./internal/agents` PASS; `go test ./internal/config`
  PASS; `go test ./internal/runtime` PASS; `go test ./internal/prompt` PASS;
  `go test ./internal/console` PASS; `go test .` PASS;
  `go test ./internal/telegram` PASS; `go test ./...` PASS after the final
  fixture correction.
- Remaining issues: None within the approved scope. Reasoning-level
  persistence remains the explicitly deferred next task.
