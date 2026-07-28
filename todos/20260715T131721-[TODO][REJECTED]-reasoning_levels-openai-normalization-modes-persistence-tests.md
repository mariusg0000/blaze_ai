# TODO: Implement reasoning levels with incremental rollout

## WHAT MUST BE DONE
Add a single, provider-agnostic `reasoning_level` setting per model. The user config must expose only abstract reasoning levels:
- `none`
- `min`
- `low`
- `med`
- `high`
- `xhigh`
- `max`

The user must never see or depend on internal API-path names such as `openai_chat` or `openai_responses`. That is a pure internal normalization detail.

The first implementation targets OpenAI-compatible APIs only:
- Chat Completions wire shape
- Responses/Codex wire shape

Reasoning levels must persist per model in `modes.json`. The active reasoning level must be shown in the status bar. Tests must cover normalization, validation, persistence, and end-to-end model-switch behavior.

Deferred to separate work items:
- `/reasoning` slash command and `Ctrl+]` hotkey.
- Additional providers beyond OpenAI scope.
- `ReasoningMaxHeight` cleanup (tracked in `20260714T062240-[TODO]-remove_reasoning_max_height`).

## WHY IT MUST BE DONE
- Providers invent different request-body formats for reasoning effort.
- A standard abstract level prevents reasoning logic from spreading across provider code.
- The project spec requires explicit errors, not silent fallbacks.
- Users need a simple, stable setting that stays readable regardless of which backend API shape is used.

## USER-FACING RULE
The user config must feel like this:

```json
{
  "model": "openrouter/openai/o3",
  "reasoning_level": "max"
}
```

The user only chooses a level. Provider-specific transformation is fully internal.

## HOW IT MUST BE DONE

### 1. Config model

#### Option A: stored inside `modes.json` per model

```json
{
  "reasoning_levels": {
    "openrouter/openai/o3": "max",
    "chatgpt/o3": "high"
  }
}
```

#### Option B: stored as a top-level active setting

```json
{
  "last_model": "openrouter/openai/o3",
  "reasoning_level": "max"
}
```

#### Recommended approach
- If per-model persistence is needed immediately: Option A.
- If only the active model matters now: Option B.
- In both cases, the user-visible field name must be `reasoning_level`, not `reasoning_effort`.

### 2. Internal normalizer

Create a pure Go package with no provider/client dependencies:

- Standard level constants only.
- A registry that maps internal API shape to normalization logic.
- The public API must be model-aware and user-config-facing:

```text
Normalize(modelID, level string) (any, error)
Supported(modelID string) []string
Default(modelID string) string
IsReasoningCapable(modelID string) bool
```

#### Key rules
- `Normalize` receives the user-facing level only.
- Internally, it must:
  1. resolve the model’s provider/API shape
  2. validate the level against that model’s supported set
  3. return the wire field the provider expects
- If the model/provider is unknown or unsupported, return an explicit error.
- If the model does not support the requested level, return an explicit error.
- No silent fallback.

### 3. Provider descriptors

#### First scope: OpenAI only

**OpenAI Chat Completions**
- Wire field: `reasoning_effort`
- Typical wire values: `none`, `minimal`, `low`, `medium`, `high`, `xhigh`
- `max` must be resolved internally to the correct wire value if the provider does not accept `max` directly.

**OpenAI Responses/Codex**
- Wire field: `reasoning.effort`
- Same abstract levels allowed from user side.
- `max` must be resolved internally if needed.

### 4. Persistence rules

- Persist under `modes.json`, not `config.json`.
- When the active model changes, load the stored reasoning level for that model.
- If no level is stored for the model, use `Default()` only for display.
- If the user explicitly requests an unsupported level, fail immediately.
- Runtime level changes must persist immediately.

### 5. Status bar

- Show the active reasoning level next to the model.
- If the model is not reasoning-capable, omit the reasoning segment.
- Immediate refresh on model/level change.

### 6. Runtime/provider integration

- `internal/provider/provider.go` must not import internal API-path names from user-facing config.
- `internal/runtime` must resolve the level from user config/model state and pass it down cleanly.
- The provider layer must translate the normalized result to the correct wire shape.

### 7. Tests

- `internal/reasoning/...` — standard level validation, supported-model lookup, normalization result per model/API path.
- `internal/config/...` — persistence round-trip for `reasoning_level`.
- Model switch tests — level switches with the model.
- Invalid/unsupported level tests — explicit errors, not fallbacks.
- `go test ./internal/reasoning/... ./internal/config/... ./internal/runtime/...` must pass.

## Explicitly deferred items

- `/reasoning` slash command.
- `Ctrl+]` keybinding.
- Multi-provider reasoning normalization beyond OpenAI.
- `ReasoningMaxHeight` cleanup.

### Rejection reason

Rejected on 2026-07-27. The implementation was intentionally abandoned because correct reasoning-level behavior differs by provider and model. OpenCode resolves this with model metadata, provider-specific compatibility rules, release-date and model-family classification, generated variants, and configuration overrides. BlazeAI has no authoritative model metadata or compatibility registry. Implementing only a generic OpenAI mapping would make the feature appear universal while allowing incompatible model requests, and reproducing the full compatibility system would be disproportionate complexity for the current product scope. Existing provider errors remain the explicit behavior instead of adding a partial or misleading abstraction.

## Validation expectations

- User config contains only abstract reasoning levels.
- Internal normalizer contains API-path logic.
- Runtime/provider code does not expose `openai_chat`/`openai_responses` to user configuration.
- `go build ./...` succeeds.
- `go test` passes for the affected packages.
- Invalid or unsupported level produces an explicit error.
