# Normalized reasoning level with Ctrl+]

## Status

postponed — awaiting user request to proceed

## User outcome

Re-introduce a provider-neutral reasoning level for the current OpenAI-compatible
request paths. The active level is normalized to exactly
`none`, `min`, `low`, `med`, `high`, `xhigh`, or `max`, cycles in that order with
wrap-around when the user presses Ctrl+], is visible in the console status
surface, is persisted per full model ID, and is sent as the appropriate OpenAI
wire effort field. The existing opaque reasoning-content storage and compaction
behavior remain unchanged.

This plan covers standard OpenAI-compatible Chat Completions and the existing
ChatGPT OAuth Responses/Codex path only. It does not add Anthropic, Google, xAI,
DeepSeek, Mistral, Bedrock, `/reasoning`, Ctrl+T output toggling, or reasoning
display rendering.

## Current state

- The former `internal/reasoning/` package and all active reasoning-level state,
  provider effort propagation, Ctrl+] binding, and status display were removed
  by the 2026-07-17 simplification decision.
- `internal/provider.chatRequest` has no effort field. The Responses request has
  `chatGPTReasoning.Effort`, but no caller sets it; the lite path only keeps its
  existing `Context: "all_turns"` value.
- `runtime.Agent` owns mutable model/session state but has no reasoning level.
  `Config` persists to `config.json`; `modes.json` is legacy migration input,
  while active per-agent model state is in `agents.json`.
- Readline already decodes `\C-]` to byte `0x1d`; it is not bound to an action.
  Existing shortcut actions mutate Agent state, persist when appropriate, and
  refresh the status bar.
- Model IDs containing `|` remain invalid. Reasoning content in session
  messages, provider response parsing, usage accounting, and
  `StripReasoningFromPayload` are independent and must remain intact.

## Supporting evidence

- `.agents/explorer/000001-reasoning-records.md`
- `.agents/explorer/000002-reasoning-config.md`
- `.agents/explorer/000003-ctrl-input.md`
- `.agents/explorer/000004-runtime-controls.md`
- `.agents/explorer/000005-reasoning-facts.md`
- `.agents/explorer/000006-model-capability-facts.md`

The two open reasoning TODOs and the earlier reasoning decisions establish the
seven-level vocabulary, Ctrl+] cycle order, `min -> minimal`, `med -> medium`,
and direct `max -> max` wire mappings. Their `modes.json` reference is stale;
the active configuration file is selected instead.

## Resolved design and rationale

1. **Canonical levels:** use only the seven exact lowercase values
   `none/min/low/med/high/xhigh/max`. Reject empty, unknown, uppercase, `ultra`,
   and arbitrary custom values with an explicit error. `none` is the explicit
   default preserving current requests that omit effort.
2. **Normalizer boundary:** create a small dependency-free
   `internal/reasoning` package containing the canonical level type, strict
   parsing, cyclic successor, and the OpenAI effort mapping. Do not recreate
   the deleted model-prefix capability table: current source has no reliable
   table, and all active providers use OpenAI-compatible request structures.
   Unsupported upstream model behavior must surface as the provider's explicit
   request error; no silent clamping or fallback is allowed.
3. **Persistence:** add `Config.ReasoningLevels map[string]string` with JSON
   key `reasoning_levels`, keyed by the complete `provider/model` ID. This is
   per-model as described by the TODOs, uses active `config.json`, avoids the
   legacy `modes.json`, and avoids changing `agents.json` schema. Missing map
   entries explicitly resolve to `none`; stored invalid values fail validation.
4. **Runtime state:** add `Agent.ReasoningLevel reasoning.Level`. Model
   application resolves the selected model's persisted level, updates both the
   Agent and the newly created Provider client, and therefore also applies to
   `SetModelLocal`. `SetReasoningLevel` strictly parses, persists, then updates
   the in-memory Agent and Provider. `NextReasoningLevel` uses the seven-value
   cycle and delegates persistence to `SetReasoningLevel`.
5. **Provider wire mapping:** add `Client.ReasoningLevel reasoning.Level`.
   Standard chat requests set `reasoning_effort` from the normalized level;
   `none` omits it through `omitempty`. Responses requests set nested
   `reasoning.effort` for non-`none` levels and preserve the lite path's
   `context: "all_turns"`. The same canonical-to-wire mapping is used in both
   paths. No summary field or reasoning-content callback is reintroduced.
6. **Console:** register `blazeai-reasoning-next` on `\C-]`, call
   `Agent.NextReasoningLevel`, report persistence errors through the existing
   shortcut error path, and refresh the status bar after success. Add
   `Reasoning: <level>` to the existing status bar and
   `Ctrl+]: cycle reasoning level` to the startup shortcut list. Do not add a
   slash command or restore Ctrl+T.
7. **Documentation:** update the three directly affected subsystem specs with
   the exact config, runtime, provider, status, and shortcut behavior. Do not
   modify stale historical decisions or unrelated specs.

## Rejected or deferred alternatives

- Do not implement the full multi-provider table from the older TODO: the
  current provider package has no provider-specific Anthropic/Google/etc.
  request paths, and adding them would expand this request beyond the smallest
  working tranche.
- Do not persist in `modes.json` or add a new state file; `modes.json` is
  migration-only and `config.json` already owns durable user configuration.
- Do not add model-prefix support detection, `SupportedLevels`, `Default`, or
  `Custom`/`Ultra` passthrough. The source contains no authoritative capability
  table; `max` is the supported top value and `ultra` is rejected.
- Do not re-add `OnReasoning`, `ShowReasoning`, `ReasoningMaxHeight`, reasoning
  output rendering, or compaction changes. Those are separate from request
  effort selection and were intentionally removed.
- Do not add `/reasoning`; Ctrl+] is the requested control surface for this
  tranche.

## Exact write paths

### Coder source and tests

- `internal/reasoning/levels.go`: create
- `internal/reasoning/levels_test.go`: create
- `internal/config/config.go`: modify only reasoning-level config field,
  default/validation, and per-model lookup/update helpers
- `internal/config/config_test.go`: modify only reasoning-level config tests
- `internal/provider/provider.go`: modify only Client/request effort state and
  standard Chat Completions serialization
- `internal/provider/provider_test.go`: modify only standard request effort
  serialization assertions
- `internal/provider/openai_responses.go`: modify only Responses request
  effort propagation
- `internal/provider/openai_responses_test.go`: modify only Responses effort
  assertions while retaining lite context assertions
- `internal/runtime/runtime.go`: modify only Agent reasoning state,
  initialization/model application, setter, and cycle methods
- `internal/runtime/runtime_test.go`: add reasoning initialization, persistence,
  model-switch, validation, and cycle assertions
- `internal/console/console.go`: modify only status-bar text, startup shortcut
  text, and Ctrl+] action registration
- `internal/console/console_test.go`: modify only splash/status assertions

### Operator documentation

- `specs/03-config-schema.md`: document `reasoning_levels`, canonical values,
  per-model persistence, strict validation, and explicit `none` default
- `specs/13-console-ui.md`: document Ctrl+] and status-bar display
- `specs/15-runtime-core.md`: document Agent/provider reasoning state and
  model-switch resolution

No other source, test, specification, decision, TODO, idea, generated, or
configuration path may change.

## Implementation-critical symbols and recipe

### `internal/reasoning/levels.go`

Add documented exported `type Level string` and constants
`None`, `Min`, `Low`, `Med`, `High`, `XHigh`, and `Max` with the exact values
listed above. Add:

- `Normalize(raw string) (Level, error)`: accept only the seven exact values;
  return the canonical Level or an explicit invalid-level error.
- `Next(level Level) (Level, error)`: validate the current value, then return
  the next value in `none -> min -> low -> med -> high -> xhigh -> max -> none`.
- `Effort(level Level) (string, error)`: validate the level; return `""` for
  `none`, `"minimal"` for `min`, `"low"` for `low`, `"medium"` for `med`,
  `"high"` for `high`, `"xhigh"` for `xhigh`, and `"max"` for `max`.

Keep the ordered list private, do not expose mutable slices, and add no model,
provider, config, or API-client dependency.

### `internal/config/config.go`

Add `ReasoningLevels map[string]string json:"reasoning_levels,omitempty"` to
`Config`. During existing config validation, normalize every stored value and
return an error identifying its model key when invalid. Add documented methods:

- `ReasoningLevelForModel(modelID string) (reasoning.Level, error)`: return the
  stored normalized value, or `reasoning.None` when the model has no entry.
- `SetReasoningLevelForModel(modelID string, level reasoning.Level) error`:
  validate with `reasoning.Normalize`, initialize the map if nil, and store the
  canonical string. This method does not save; the runtime owns the existing
  `Config.Save()` side effect.

Do not alter model ID validation, provider validation, defaults unrelated to
reasoning, or the `StripReasoning` config.

### `internal/runtime/runtime.go`

Add `ReasoningLevel reasoning.Level` to `Agent`. In the existing initial client
construction and `applyModel` path, resolve the new model's level through
`Config.ReasoningLevelForModel`, set the Agent field and the new Provider client
field, and return resolution errors instead of substituting another level.

Add:

- `SetReasoningLevel(raw string) error`: normalize the input; update the
  per-model config entry; save with existing `Config.Save`; if saving fails,
  return the error without leaving the active Agent/Provider level changed;
  after success assign the canonical level to both Agent and Provider.
- `NextReasoningLevel() (reasoning.Level, error)`: calculate the next level from
  the active value using `reasoning.Next`, call `SetReasoningLevel`, and return
  the new value. Preserve the current level on any error.

Keep `NewAgent`, `SetModel`, `SetModelLocal`, model precedence, session state,
compaction, and handler interfaces otherwise unchanged.

### `internal/provider/provider.go` and `openai_responses.go`

Add `ReasoningLevel reasoning.Level` to `Client`, initialized to
`reasoning.None` by `NewClient`. Add
`ReasoningEffort string json:"reasoning_effort,omitempty"` to `chatRequest` and
populate it from `reasoning.Effort(c.ReasoningLevel)` on standard requests.
Propagate the same effort into `chatGPTReasoning.Effort` in both existing
Responses builders; leave `Summary` empty and preserve lite `Context`.
Invalid client levels must return an error from request construction/streaming,
not be silently replaced.

### `internal/console/console.go`

In the existing `runTTY` keymap registration, register the exact action name
`blazeai-reasoning-next`; bind `inputrc.Unescape("\\C-]")` in the emacs map.
The action calls `Agent.NextReasoningLevel`, uses the same existing error
reporting pattern as the other shortcut actions, and calls `updateStatusBar`
only after success. Add the exact splash label `Ctrl+]: cycle reasoning level`.
Add the exact status segment `Reasoning: <canonical-level>` while preserving
the existing width, phase, agent, model, workdir, and context behavior.

## Tests and exact assertions

### `internal/reasoning/levels_test.go`

- `TestNormalize`: all seven exact values round-trip to their constants;
  uppercase, empty, `ultra`, and custom values return errors.
- `TestNextCyclesAllLevels`: assert every adjacent transition and
  `max -> none`.
- `TestEffortMapping`: assert the exact seven mappings, including `none -> ""`
  and `max -> "max"`.

### `internal/config/config_test.go`

- Assert an absent model entry resolves to `reasoning.None`.
- Assert `SetReasoningLevelForModel` stores the canonical string under the full
  model ID and JSON round-trips it.
- Assert an invalid stored value fails config validation with the model key in
  the error.

### Provider tests

- Standard request with `high` serializes `"reasoning_effort":"high"`; with
  `none`, the field is absent.
- Responses request with `med` serializes nested `"reasoning":{"effort":"medium"}`.
- Lite Responses request with `high` preserves `context:"all_turns"` and adds
  `effort:"high"`; with `none`, effort remains omitted and context remains.
- Invalid client level returns an error rather than producing a request.

### `internal/runtime/runtime_test.go`

- New Agent with no stored entry starts at `none` and its Provider matches.
- `SetReasoningLevel("high")` updates Agent and Provider and persists the full
  model key in `config.json`.
- Invalid level and failed save return errors without changing active state.
- `NextReasoningLevel` follows the complete seven-value wrap-around and
  persists each change.
- Switching models loads each model's independent stored level; a model with
  no entry explicitly uses `none`. `SetModelLocal` applies the same lookup
  without introducing a new persistence side effect.

### `internal/console/console_test.go`

- Startup splash contains the exact Ctrl+] label.
- Status-bar rendering contains `Reasoning: none` and a non-default canonical
  level without removing the existing model and context text.

Retain all existing reasoning-content capture, session serialization,
compaction, usage, model, agent, and shortcut decoding tests unchanged except
where request-builder signatures require mechanical fixture updates.

## Required verification commands

1. `go test ./internal/reasoning`
2. `go test ./internal/config`
3. `go test ./internal/provider`
4. `go test ./internal/runtime`
5. `go test ./internal/console`
6. `go test ./...`

Run every command after the latest relevant write. A correction reruns every
check it could invalidate. Return one concise `PASS`, `FAIL`, or `NOT RUN`
entry per command.

## Acceptance criteria

- Only the declared source, test, and three documentation paths change.
- All seven canonical levels normalize strictly and cycle with wrap-around.
- Ctrl+] is bound in the existing readline action system, updates the active
  level, persists it per full model ID, refreshes the status bar, and surfaces
  persistence errors.
- Both current OpenAI-compatible request paths emit the exact effort mapping;
  `none` omits effort, Responses lite retains `all_turns`, and no fallback or
  silent clamping occurs.
- Model switches load independent persisted levels; missing entries use the
  explicit `none` default; invalid persisted values fail explicitly.
- Existing reasoning-content persistence/stripping and all unrelated model,
  agent, session, and console behavior remain intact.
- All six verification commands pass.

## Unresolved questions

None. Provider scope, persistence location, level set, default, status format,
and deferred command/output behavior are resolved above for this tranche.

## Stop conditions

Stop and return for clarification if implementation requires changing
`agents.json`, `modes.json` migration semantics, adding provider-specific code
outside the two current OpenAI paths, inventing model capability tables,
re-adding reasoning output callbacks, adding a fallback/clamp, or changing any
path outside the allowlists.
