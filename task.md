status: rejected

# Normalized OpenAI reasoning level with Ctrl+]

## User outcome

Re-introduce a provider-neutral reasoning level for the existing OpenAI-compatible request paths. The canonical level is persisted by full model ID, applied when a model becomes active, emitted in each OpenAI request shape, displayed in the console, and cycled with Ctrl+]. Existing opaque reasoning-content storage, compaction, usage accounting, and Responses-lite continuation context remain unchanged.

## Plan summary

BlazeAI currently accepts only a model ID and always leaves reasoning effort to the provider default. This task adds one strict seven-value setting (`none`, `min`, `low`, `med`, `high`, `xhigh`, `max`) owned by a small dependency-free package. The runtime resolves the setting for the active full model ID, persists changes in `config.json`, and copies the resolved value to the provider client.

The provider converts the canonical value to the OpenAI wire value only when building requests (`min` to `minimal`, `med` to `medium`; `none` omits effort). The console invokes the runtime cycle with Ctrl+] and refreshes its status display. This keeps provider-specific JSON out of user-facing configuration, avoids model capability guessing, and lets unsupported upstream values fail as explicit provider errors rather than silently changing behavior.

## Current state

- The former reasoning-level package, provider effort state, Ctrl+] action, and status segment were removed; no current reasoning-level control exists.
- `session.Message.reasoning_content`, reasoning response accumulation, `StripReasoningFromPayload`, and Responses-lite `reasoning.context: "all_turns"` remain implemented and are out of scope.
- The normal Chat Completions request is assembled inline in `Client.StreamWithPhase`; Responses builders already return `(chatGPTResponsesRequest, error)` and have a nested `chatGPTReasoning.Effort` field that is currently omitted.
- `agents.json` persists a selected model per interactive agent. `modes.json` is migration-only. `config.json` is the appropriate new global per-model lookup map because the setting is keyed by model, must apply when any agent selects that model, and must not change the active `agents.json` schema.
- `inputrc.Unescape("\\C-]")` already decodes to byte `0x1d`; no action is bound. The console status bar currently renders phase, agent, model, work directory, and context.
- OpenCode's provider/model compatibility is metadata-driven: it imports model capability, API package, release-date, and interleaving metadata; generates static provider/model-specific variants; permits config overrides; and merges the selected variant into request options. BlazeAI has no equivalent model metadata source, so OpenCode's implementation is comparative evidence rather than a directly reusable dependency.

## Supporting evidence

- `.agents/explorer/000005-reasoning-facts.md`
- `.agents/explorer/000006-model-capability-facts.md`
- `.agents/explorer/000007-current-reasoning-plan.md`
- `.agents/explorer/000011-opencode-reasoning-mechanism.md`

## Resolved decisions and rationale

1. Scope is the two implemented OpenAI-compatible paths only: standard Chat Completions and ChatGPT OAuth Responses. The repository has no provider-specific Anthropic, Google, xAI, DeepSeek, Mistral, or Bedrock implementations; adding them is outside the smallest direct tranche.
2. Canonical user values are exactly lowercase `none`, `min`, `low`, `med`, `high`, `xhigh`, and `max`. Empty, uppercase, `ultra`, and arbitrary values are explicit errors. `none` is the explicit default for an absent per-model entry and preserves current request omission.
3. Add `Config.ReasoningLevels map[string]string` with JSON key `reasoning_levels,omitempty`, keyed by the complete `provider/model` ID. Stored values are strictly validated. This does not alter `agents.json`, `modes.json`, model selection precedence, or model-ID validation.
4. Add dependency-free `internal/reasoning`; it owns parsing, cyclic progression, and OpenAI effort mapping only. Do not create model-prefix capability tables, supported-level APIs, custom values, clamping, or fallbacks: source has no authoritative capability inventory.
5. `runtime.Agent` owns the resolved in-memory `reasoning.Level`. Creation and every `applyModel` path resolve the selected model's map entry and set both Agent and newly created Provider client. `SetReasoningLevel` saves config before changing Agent/Provider state; a save failure leaves active state unchanged.
6. Standard requests serialize flat `reasoning_effort`; Responses serializes nested `reasoning.effort`. `none` omits effort. Both use the same mapping: `min -> minimal`, `med -> medium`, all other non-none canonical values map directly. Lite Responses retains `context: "all_turns"`; no summary field is added.
7. Register `blazeai-reasoning-next` for Ctrl+] (`inputrc.Unescape("\\C-]")`). On success it refreshes the existing status bar. The splash adds `Ctrl+]: cycle reasoning level`, and status adds `Reasoning: <canonical-level>`. Do not add `/reasoning`, Ctrl+T, reasoning rendering, or an `OnReasoning` callback.

## Rejected or deferred alternatives

- Multi-provider normalization, provider capability detection, default/supported-level lookup, and model-prefix matching are deferred; they require unavailable authoritative model metadata.
- `/reasoning` is deferred; Ctrl+] is the sole requested control surface.
- Persisting in `modes.json` is rejected because it is migration-only. Persisting in `agents.json` is rejected because this setting is model-wide and would alter the active agent-state schema.
- Reintroducing output display, `ShowReasoning`, `ReasoningMaxHeight`, `OnReasoning`, compaction changes, temperature/top-p coupling, budgets, or a fallback/clamp is out of scope.

## Exact write paths

- `internal/reasoning/levels.go`: create
- `internal/reasoning/levels_test.go`: create
- `internal/config/config.go`: modify
- `internal/config/config_test.go`: modify
- `internal/provider/provider.go`: modify
- `internal/provider/provider_test.go`: modify
- `internal/provider/openai_responses.go`: modify
- `internal/provider/openai_responses_test.go`: modify
- `internal/runtime/runtime.go`: modify
- `internal/runtime/runtime_test.go`: modify
- `internal/console/console.go`: modify
- `internal/console/console_test.go`: modify

No other paths may change.

## Sequential delegation units

### Unit 1 — Canonical reasoning primitives

**Goal:** provide the strict canonical value contract shared by configuration, runtime, and provider code.

**Write paths:** `internal/reasoning/levels.go`, `internal/reasoning/levels_test.go`

**Recipe:** create exported `type Level string`; exported constants `None`, `Min`, `Low`, `Med`, `High`, `XHigh`, `Max`; and exactly these functions:
- `Normalize(raw string) (Level, error)`: accept only the seven exact strings.
- `Next(level Level) (Level, error)`: validate then cycle `none -> min -> low -> med -> high -> xhigh -> max -> none`.
- `Effort(level Level) (string, error)`: return `""` for none, `"minimal"` for min, `"medium"` for med, and the direct canonical string for all other valid levels.

Keep the ordered values private and add no provider/model/config dependencies. Tests assert all valid values, rejected empty/uppercase/`ultra`/custom values, every cycle edge including wrap-around, and every exact mapping.

**Verification:** `go test ./internal/reasoning`

### Unit 2 — Per-model durable configuration

**Goal:** persist and validate a canonical level for each full model ID.

**Write paths:** `internal/config/config.go`, `internal/config/config_test.go`

**Dependencies:** Unit 1; current Config validation and save behavior.

**Recipe:** add `ReasoningLevels map[string]string json:"reasoning_levels,omitempty"` to `Config`. During `Validate`, normalize every map value and return an error identifying the offending model key. Add `ReasoningLevelForModel(modelID string) (reasoning.Level, error)` to return stored normalized level or `reasoning.None` when absent. Add `SetReasoningLevelForModel(modelID string, level reasoning.Level) error` to validate, initialize the map, and store the canonical string without saving. Do not alter unrelated defaults, `StripReasoning`, or model-ID validation.

Tests assert absent entry returns none; setter stores under the full model ID and JSON round-trips; invalid stored value fails validation and identifies its key.

**Verification:** `go test ./internal/config`

### Unit 3 — OpenAI request effort propagation

**Goal:** send the resolved canonical setting in both implemented OpenAI request wire shapes.

**Write paths:** `internal/provider/provider.go`, `internal/provider/provider_test.go`, `internal/provider/openai_responses.go`, `internal/provider/openai_responses_test.go`

**Dependencies:** Unit 1; current standard inline request construction and Responses builders.

**Recipe:** add `ReasoningLevel reasoning.Level` to `Client`, initialized by `NewClient` to `reasoning.None`. Add `ReasoningEffort string json:"reasoning_effort,omitempty"` to `chatRequest`; calculate `reasoning.Effort(c.ReasoningLevel)` before standard request marshal and return its error. Pass the same calculated effort into `buildChatGPTRequest` and `buildChatGPTLiteRequest` through an added `reasoning.Level` parameter; populate `chatGPTReasoning.Effort` from it. Preserve lite `Context: "all_turns"` and leave `Summary` empty. Invalid client level returns an error; no replacement level is chosen.

Tests assert standard high serializes flat `reasoning_effort:"high"`, none omits it; Responses med serializes nested `reasoning.effort:"medium"`; lite high includes both effort and `all_turns`; lite none omits effort while retaining context; invalid level returns an error.

**Verification:** `go test ./internal/provider`

### Unit 4 — Runtime selection, model application, and persistence

**Goal:** make the resolved level track every active model and persist a user cycle safely.

**Write paths:** `internal/runtime/runtime.go`, `internal/runtime/runtime_test.go`

**Dependencies:** Units 1–3; `Config` lookup/setter and provider client field.

**Recipe:** add `ReasoningLevel reasoning.Level` to `Agent`. In `newAgent` and `applyModel`, resolve `Config.ReasoningLevelForModel(fullModelID)` before committing replacement state; set both Agent and Provider levels. Add `SetReasoningLevel(raw string) error`: normalize raw input, update the current model's config entry, call existing `Config.Save`, and only then set Agent/Provider fields. Add `NextReasoningLevel() (reasoning.Level, error)`: calculate next via `reasoning.Next`, delegate to setter, return the new level. Preserve level on any error. Do not change `SetModel`, `SetModelLocal`, interactive-agent selection, their existing persistence responsibilities, model precedence, or session/compaction behavior.

Tests assert new agents without entries use none in both Agent and Provider; setting high persists the complete current model key; invalid input and failed config save preserve state; all cycle transitions persist; model and interactive-agent changes load independent persisted values while missing entries use none; `SetModelLocal` resolves level without a new persistence side effect.

**Verification:** `go test ./internal/runtime`

### Unit 5 — Console control and status surface

**Goal:** expose the runtime cycle through Ctrl+] and visibly reflect the active value.

**Write paths:** `internal/console/console.go`, `internal/console/console_test.go`

**Dependencies:** Unit 4; existing readline action/error/status patterns.

**Recipe:** in `runTTY`, register action `blazeai-reasoning-next`; it calls `Agent.NextReasoningLevel`, reports errors using the existing shortcut error path, and calls `updateStatusBar` only after success. Bind it in the emacs map using `inputrc.Unescape("\\C-]")`. Add the exact splash label `Ctrl+]: cycle reasoning level`. In `buildStatusBar`, add `Reasoning: <canonical-level>` while preserving phase, agent, model, work-directory, width, and CTX output behavior. Do not add a slash command or any reasoning-content display.

Tests assert splash contains the exact label; status output includes `Reasoning: none` and a non-default canonical value while retaining existing model and CTX text.

**Verification:** `go test ./internal/console`

## Final integration verification

After all units: `go test ./...`

## Acceptance criteria

- Only declared paths change.
- The seven lowercase canonical levels are strict, cycle with wrap-around, and map exactly to OpenAI effort strings.
- A full model ID has an independently persisted level in `config.json`; missing entry is explicit none and invalid stored entry errors.
- Agent creation, `SetModel`, `SetModelLocal`, and agent switching apply the selected model's resolved level to both Agent and Provider without changing established model persistence or precedence.
- Standard and Responses OpenAI paths emit correct effort fields; none omits effort; lite preserves `all_turns`; invalid level errors rather than falling back.
- Ctrl+] persists and refreshes the visible level; no slash command, output callback/display, capability table, or multi-provider behavior is added.
- All six package commands pass: reasoning, config, provider, runtime, console, and all packages.

## Unresolved questions

None. OpenCode confirms that a broad compatibility system needs an authoritative model metadata source plus provider/model classification and override semantics. BlazeAI has none, so the proposed narrow OpenAI-only task deliberately sends a strict user-selected level and lets the upstream provider return an explicit rejection rather than inventing unsupported compatibility data.

## Stop conditions

Stop for clarification if implementation requires changing `agents.json`, `modes.json` migration behavior, model selection precedence, adding provider-specific paths beyond the two OpenAI paths, inventing capability metadata, adding a fallback/clamp, reintroducing reasoning output, or modifying any undeclared path.

## Rejection reason

Rejected on 2026-07-27. OpenCode analysis showed that reliable reasoning-level compatibility depends on authoritative model metadata, provider/model-specific variants, release-date and family rules, and configuration overrides. BlazeAI does not have that metadata foundation. A partial OpenAI-only implementation would risk claiming compatibility where none exists, while reproducing OpenCode's system would be disproportionate complexity. The reasoning-level feature is abandoned with no implementation.
