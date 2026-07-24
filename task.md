---
status: completed
---

# Bootstrap the user-editable default agent

## User outcome

When a new or empty app home has no interactive agent definition, create a
user-editable `~/.blazeai/agents/default.md` from a hardcoded initial template.
Use the validated configured default model, the current native tool registry,
and the currently loaded executor names. Never overwrite an existing file.
Invalid existing definitions must still fail normally. Persist the resulting
active agent through the existing `config/agents.json` flow.

## Current state

- `runtime.NewAgent` currently loads/migrates definitions and then returns
  `no interactive agent definitions found` when no interactive definition is
  present.
- `platform.Bootstrap` already creates `~/.blazeai/agents/` and
  `~/.blazeai/config/` with `0755` permissions.
- `cfg.Roles.Default` is validated before `NewAgent` and is the current initial
  model source.
- The temporary definition-validation registry already contains the native tool
  names and `run_agent`/`agent_done` control names.
- Existing `agents.json` state must prevent automatic recreation after a user
  has intentionally removed all definitions; in that case the existing relevant
  missing-interactive-agent error remains required.

## Supporting evidence

- `.agents/explorer/260724-default-agent-bootstrap.md`

## Resolved design

1. Add `internal/agents/bootstrap.go` with
   `func EnsureDefaultInteractive(agentsDir, model string, toolNames, executorNames []string) (bool, error)`.
2. The function targets exactly `<agentsDir>/default.md`. If that path exists,
   return `(false, nil)` without reading, modifying, or replacing it. If it is
   absent, validate the supplied model, create the parent directory if needed,
   and write the hardcoded template with mode `0600`, returning `(true, nil)`.
3. The generated definition is:
   - `name: default`;
   - `description: General-purpose interactive agent`;
   - `type: interactive`;
   - `model` equal to the supplied validated model;
   - `tools` containing all supplied tool names except `agent_done` and
     `run_agent`, sorted deterministically;
   - `agents` containing all supplied executor names, sorted deterministically,
     omitted when empty;
   - no `directive` field;
   - a fixed English body instructing the agent that it is the primary
     interactive BlazeAI agent and should use its available tools responsibly.
4. Modify `runtime.NewAgent` after legacy migration and the first definition
   load: if no interactive definitions exist and `agents.json` does not exist,
   collect loaded `TypeExecutor` names, call `EnsureDefaultInteractive` with
   `cfg.Roles.Default`, the temporary registry names, and those executor names,
   then reload definitions and continue the existing state initialization.
5. If `agents.json` exists and no interactive definition exists, do not create a
   file; return the existing relevant error. If an existing `default.md` is
   invalid, definition loading returns its error and bootstrap never overwrites
   it. If another interactive definition already exists, do not create
   `default.md`.
6. Do not change `agents.json` schema, reasoning-level behavior, model selection
   precedence, migration semantics, console behavior, or tool permissions beyond
   the generated default's explicit allowlist.

## Exact write paths

- `internal/agents/bootstrap.go`: create
- `internal/agents/bootstrap_test.go`: create
- `internal/runtime/runtime.go`: modify only the no-interactive bootstrap flow
- `internal/runtime/runtime_test.go`: modify only bootstrap/error assertions

No other source, test, specification, decision, or generated path may change.

## Implementation recipe

### `internal/agents/bootstrap.go`

- Document the exported function and fixed template with English WHAT/HOW
  comments.
- Use `os.Stat` to distinguish an existing path from a missing path. Return
  existing non-`IsNotExist` stat errors.
- Validate `model` with `config.ValidateModelFormat` before writing a new file.
- Filter and sort tool/executor names without mutating caller slices. Reject no
  names beyond the existing model validation; the caller already supplies names
  from validated registries/definitions.
- Generate valid front matter accepted by `agents.Load`, followed by the fixed
  body and a trailing newline. Do not include `directive`, `agent_done`, or
  `run_agent` in generated front matter tools.
- Write only when the target was absent; use `0600` and propagate all errors.

### `internal/runtime/runtime.go`

- Preserve the existing migration order and public `NewAgent` signature.
- After the initial `agents.Load`, split definitions into interactive and
  executor sets. Before the current hard error for zero interactive definitions:
  - if `agents.json` exists, return the existing error unchanged;
  - otherwise collect executor names and call the new helper using
    `cfg.Roles.Default` and `registeredToolNames(tempRegistry)`;
  - reload `agents.Load` and recompute both sets.
- Continue through the existing `AgentsConfig` initialization, `LastAgent`
  selection, model persistence, and `newAgent` wiring without additional
  fallback branches.
- Keep the existing error when the second load still has no interactive agent.

### Tests

- `internal/agents/bootstrap_test.go`:
  - `TestEnsureDefaultInteractiveCreatesDefinition`: assert generated name,
    type, description, model, fixed body, sorted tools, sorted executor list,
    omitted control tools, omitted directive, and `0600` mode.
  - `TestEnsureDefaultInteractiveDoesNotOverwrite`: pre-create custom
    `default.md`, assert `(false, nil)` and byte-for-byte preservation.
  - `TestEnsureDefaultInteractiveRejectsInvalidModel`: assert validation error
    and no file.
  - `TestEnsureDefaultInteractivePropagatesUnsafeWriteError`: use a file as the
    agents directory and assert an error.
- `internal/runtime/runtime_test.go`:
  - replace/update the old no-definition failure assertion with
    `TestNewAgentBootstrapsDefaultInteractiveAgent`: isolated HOME with no
    definitions or `agents.json`; assert `NewAgent` succeeds, `default.md`
    exists, its parsed model is `test/test-model`, its type is interactive, its
    body is non-empty, and `agents.json` has `LastAgent: default`.
  - add `TestNewAgentDoesNotBootstrapWhenAgentStateExists`: isolated HOME with
    `agents.json` but no interactive definitions; assert the relevant missing
    interactive-agent error and no `default.md`.
  - retain existing tests for valid existing definitions and state loading.

## Required behavior and invariants

- Generated default is user-editable and never overwritten.
- Bootstrap happens only when there is no interactive definition and no
  `agents.json` state; it is provisioning, not a runtime model/tool fallback.
- Existing invalid/missing configuration errors remain explicit.
- The configured model is never hardcoded and no reasoning-level field is
  introduced.
- All generated tools and executor references are explicit and deterministic.

## Verification commands

1. `go test ./internal/agents`
2. `go test ./internal/runtime`
3. `go test ./...`

Run each after the latest relevant write. A correction reruns every affected
command. Return one concise `PASS`, `FAIL`, or `NOT RUN` result per command.

## Acceptance criteria

- A fresh isolated app home with no interactive definitions receives a valid,
  editable `agents/default.md` seeded from the configured model.
- Existing `default.md` and invalid definitions are never overwritten.
- Existing `agents.json` prevents automatic recreation and preserves explicit
  error behavior.
- All three verification commands pass and only the four declared paths change.

## Stop conditions

Stop if bootstrap requires changing the state schema, adding a fallback for a
missing configured model, changing tool permissions, or modifying paths outside
the allowlist.

## Completion

- Accepted outcome: Fresh app homes now receive a user-editable default
  interactive agent only when no interactive definition or `agents.json` state
  exists; existing definitions and state preserve their prior behavior.
- Changed paths: `internal/agents/bootstrap.go`,
  `internal/agents/bootstrap_test.go`, `internal/runtime/runtime.go`,
  `internal/runtime/runtime_test.go`.
- Verification: `go test ./internal/agents` — PASS; `go test ./internal/runtime`
  — PASS; `go test ./...` — PASS.
- Remaining issues: None.
