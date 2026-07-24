# Decision Summary: Replace legacy mode/one-shot system with interactive/executor agent model

Date: 2026-07-24 12:00

## Context

The project needed to replace the legacy mode-based system (where `modes.json`
stored active modes with denied-tools, directives, and executor lists) and the
one-shot child-agent model (`kind: one-shot`) with an OpenCode-like agent model
where Markdown definitions use `type: interactive` or `type: executor`.

The user authorized and approved the complete design: interactive agents replace
active modes for selection, prompts, tools, directives, and model persistence;
executor agents preserve current child execution semantics; legacy modes and
definitions are migrated automatically; reasoning-level persistence is explicitly
deferred.

## Changes Made

- `internal/agents/agents.go`: Replaced `Kind`/`KindOneShot` with `Type`/
  `TypeInteractive`/`TypeExecutor`. Added `Directive` and `ExecutorNames` fields
  to `Definition`. Updated `parse()` to handle `type`, `directive`, and `agents`
  front-matter keys. Rejected the legacy `kind` key with a migration hint.
  Added type-specific validation (interactive requires model, executor must not
  have directive/agents). Added `validateExecutorReferences()` for cross-definition
  integrity.
- `internal/agents/agents_test.go`: Comprehensive tests for interactive and
  executor parsing, type-specific validation, executor reference resolution,
  duplicate references, non-executor references, legacy kind rejection, empty
  tool list rejection, and directive/body support.
- `internal/agents/migration.go` (new): `MigrateLegacyDefinitions()` replaces
  `kind: one-shot` with `type: executor` in-place. `MigrateLegacyModes()`
  converts `modes.json` entries into interactive Markdown definitions without
  overwriting existing files.
- `internal/agents/migration_test.go` (new): Tests for kind-to-type migration,
  mode-to-interactive conversion, no-overwrite guarantee, unsafe name rejection,
  and nil-legacy safety.
- `internal/config/agents.go` (new): `AgentsConfig` with `InteractiveAgentState`
  list and `LastAgent`, atomic save via temp-file+rename, load from disk,
  and validation (unique names, valid models, non-empty LastAgent when states
  exist).
- `internal/config/agents_test.go` (new): Round-trip, atomic-save, validation,
  invalid model, missing file, empty agents, and JSON omit-empty tests.
- `internal/runtime/runtime.go`: Replaced `Modes`/`CurrentMode` with
  `AgentStates`/`CurrentAgent`. Rewrote `NewAgent` to: create base registry,
  run legacy migrations, load definitions, load/initialize `agents.json`, resolve
  persisted interactive agent and model, validate state, add missing state entries,
  and construct agent. Replaced `SetMode`/`NextMode` with `SetAgent`/`NextAgent`.
  Updated `SetModel` to persist per-interactive-agent. Replaced directive injection
  from `[MODE DIRECTIVE]` to `[AGENT DIRECTIVE]`. Added `fileExists()`,
  `registeredToolNames()`, `wellKnownToolNames`, and `stubTool` for validation.
- `internal/runtime/agent_orchestration.go`: Replaced one-shot terminology with
  executor terminology. Changed `oneShotDefinition()` to `executorDefinition()`.
  Updated `modeAllowsAgent()` calls to `interactiveAllowsExecutor()`.
- `internal/runtime/agent_capabilities.go` (new): `refreshAgentCapabilities()`
  filters `BaseTools` by the current interactive agent's `ToolNames`, resolves
  allowed executor definitions for `Builder.Agents`, and auto-includes `run_agent`
  when at least one executor is allowed. `interactiveAllowsExecutor()` gates
  dispatch.
- `internal/runtime/agent_capabilities_test.go` (new): Tests for tool filtering,
  executor resolution, run_agent auto-inclusion, unknown tool rejection, and
  executor allowlist check.
- `internal/runtime/mode_capabilities.go` (deleted): Replaced by
  `agent_capabilities.go`.
- `internal/runtime/runtime_test.go`: Updated all tests to use interactive agent
  definitions and `AgentsConfig` instead of modes. Added tests for persisted
  agent loading, SetAgent, NextAgent cycling, SetModel per-agent persistence,
  executor rejection, ephemeral directive injection, and required interactive
  definition error.
- `internal/prompt/prompt.go`: Changed `[kind]` rendering to `[type: executor]`.
  Updated execution instructions to use "executor" instead of "one-shot".
- `internal/prompt/prompt_test.go`: Tests for type rendering, empty agents
  section, and agent instructions body injection.
- `internal/console/console.go`: Added `/mode [agent]` slash command. Updated
  status bar, Tab shortcut, and startup splash to use agent terminology.
- `internal/console/console_test.go`: Tests for `/mode` set/usage-error/
  executor-rejection, Tab cycling, startup splash agent terminology, and
  prompt label stability.
- `firstrun.go`: Removed `modes.json` creation during first-run setup.
- `firstrun_test.go`: Tests verifying modes.json is not created by first run.
- `internal/telegram/commands_test.go`: Updated test fixtures to create
  interactive agent definitions and `agents.json` state for the runtime.
- `task.md`: Marked task as completed with full rationale and verification
  results.

## Decisions And Rationale

- **`kind: one-shot` → `type: executor`**: The old `kind` field is rejected at
  parse time with a migration hint. Legacy files are migrated in-place before
  loading by `MigrateLegacyDefinitions()`.
- **`denied_tools` → `tools` allowlist**: Interactive definitions use an explicit
  `tools` allowlist instead of the inverted deny-list pattern. This matches the
  OpenCode agent model and is clearer for users.
- **Per-interactive-agent model persistence**: Each interactive agent has its own
  persisted model in `agents.json`, replacing the single global `LastModel` for
  mode-based switching.
- **`agents` list on interactive definitions**: Limits which executor definitions
  each interactive agent can invoke, replacing the per-mode `Agents` field.
- **`directive` on interactive definitions only**: Executor definitions have no
  directive. The directive is injected ephemerally into the latest user message
  via `[AGENT DIRECTIVE]` and never persisted to the session.
- **State in `config/agents.json`**: Atomic save with temp-file+rename pattern.
  Contains only `agents` list (name+model) and `last_agent`. Reasoning-level
  persistence is explicitly deferred.
- **Automatic legacy mode migration**: When `agents.json` does not exist but
  `modes.json` does, legacy modes are converted to interactive Markdown
  definitions. Existing same-name files are never overwritten; collisions are
  errors.
- **No interactive definition is a hard error**: `NewAgent` returns an error
  rather than silently falling back to a global/default model.
- **Reasoning-level persistence deferred**: No reasoning field is added to
  `InteractiveAgentState`. This is explicitly a future step.
