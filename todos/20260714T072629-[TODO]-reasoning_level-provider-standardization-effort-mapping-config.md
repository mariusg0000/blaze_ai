# TODO: Implement reasoning level normalization module

## WHAT MUST BE DONE
Create a Go module (`internal/reasoning/`) that normalizes a **standard set of reasoning level values** into **provider-specific request body parameters**. This module is the single source of truth for reasoning level translation across all providers.

Core requirements:
1. Define standard reasoning levels (abstract values) that users/config interact with
2. Implement a normalizer that maps standard values → provider-specific JSON fields
3. Each provider registers its supported levels and the transformation function
4. The normalizer validates input against allowed levels and returns the correct request body fragment
5. Providers that don't support reasoning levels are explicitly flagged (no silent ignore)

## WHY IT MUST BE DONE
- No OpenAI standard exists for reasoning levels — each provider invented its own format
- Without normalization, reasoning level logic would be scattered across provider code with inconsistent handling
- A dedicated module makes it trivial to add new providers or adjust mappings
- User-facing config uses simple values (`low`, `medium`, `high`); the module handles the ugly translation

## HOW IT MUST BE DONE

### Research from opencode (`/home/marius/opencode/packages/opencode/src/provider/transform.ts`)

opencode solves this with a `variants()` function. Key provider-specific formats:

| Provider | Request field | Values |
|----------|--------------|--------|
| OpenAI | `reasoning_effort` | "none", "minimal", "low", "medium", "high", "xhigh" |
| Anthropic | `thinking.type` + `budgetTokens` | "enabled"/"adaptive" + token count |
| Google/Gemini | `thinkingConfig.thinkingLevel` | "minimal", "low", "medium", "high" |
| xAI/Grok | `reasoningEffort` | "low", "high" |
| DeepSeek | (none — reasoning always on) | — |
| Mistral | `reasoningEffort` | "high" only |
| Bedrock (Anthropic) | `reasoningConfig.type` + `budgetTokens` | "enabled" + token count |
| Bedrock (Nova) | `reasoningConfig.maxReasoningEffort` | "low", "medium", "high" |

### Module design: `internal/reasoning/`

```go
// Standard levels that users and config reference.
const (
    LevelNone     = "none"
    LevelMinimal  = "minimal"
    LevelLow      = "low"
    LevelMedium   = "medium"
    LevelHigh     = "high"
    LevelXHigh    = "xhigh"
    LevelMax      = "max"
)

// ProviderDescriptor defines what a provider supports and how to transform.
type ProviderDescriptor struct {
    SupportedLevels []string                    // allowed standard levels
    Transform       func(level string) map[string]any  // standard → request body fragment
}

// Normalize takes a standard level, validates it, and returns the
// provider-specific request body fields.
func Normalize(provider, modelID, level string) (map[string]any, error)

// SupportedLevels returns the allowed levels for a given provider/model.
func SupportedLevels(provider, modelID string) []string
```

**Provider registration:**
- `openai.go` — Maps to `reasoning_effort` field; detects GPT-5 family for extended levels
- `anthropic.go` — Maps to `thinking` + `budgetTokens`; different for adaptive vs enabled
- `google.go` — Maps to `thinkingConfig.thinkingLevel` or `thinkingConfig.thinkingBudget`
- `xai.go` — Maps to `reasoningEffort` with "low"/"high"
- `deepseek.go` — Returns empty (no reasoning control); flags as unsupported
- `mistral.go` — Maps to `reasoningEffort` for specific models only
- `generic.go` — Fallback: tries `reasoning_effort` (OpenAI-compatible default)

**Integration points:**
- `internal/config/config.go` — Add `ReasoningLevel string` field to model config
- `internal/provider/provider.go` — Call `reasoning.Normalize()` when building streaming request body
- `internal/runtime/` — Runtime override via `/reasoning <level>` command
- `internal/console/slash.go` — Slash command to change reasoning level mid-session
- `internal/console/input.go` — Bind `Ctrl+]` to cycle through supported reasoning levels

**Hotkey: Ctrl+]**
- Cycles through the supported levels for the current model in order: `none → minimal → low → medium → high → xhigh → max → none → ...`
- Only cycles through levels present in `SupportedLevels(provider, modelID)`
- Displays current level after change (e.g. ` reasoning: high `)
- If model has no reasoning support, the keypress is a no-op

**Validation behavior:**
- If level is not in the provider's `SupportedLevels`, return error (per spec: no fallbacks)
- If provider has no descriptor (unknown provider), return error with clear message
- If model is not reasoning-capable, return error stating the model doesn't support it

**Persistence in modes.json:**
- Reasoning level is stored in `modes.json` under the active work mode, keyed by model ID
- Structure: `{ "reasoning_levels": { "model_id": "high" } }`
- When the model changes, the reasoning level loads from the stored value for that model
- `Ctrl+]` or `/reasoning` updates the stored value immediately
- Level persists across sessions and restarts until explicitly changed
- If a model has no stored level, it starts with the provider's default (or `medium` if no default)

**Open questions:**
- Budget tokens for Anthropic: fixed values (16k/32k) or configurable via `reasoning_budget` in config?
- Should the normalizer also handle temperature/top-p adjustments tied to reasoning levels?

## Research: opencode (`/home/marius/opencode/packages/opencode/src/provider/transform.ts`)

opencode uses a `variants()` function that maps abstract effort levels to provider-specific options.

**ReasoningEffort values:** none, minimal, low, medium, high, xhigh

**Provider-specific formats:**

| Provider | Request field | Values |
|----------|--------------|--------|
| OpenAI | `reasoning_effort` | none, minimal, low, medium, high, xhigh |
| Anthropic | `thinking.type` + `budgetTokens` | "enabled"/"adaptive" + token count |
| Google/Gemini | `thinkingConfig.thinkingLevel` | minimal, low, medium, high |
| xAI/Grok | `reasoningEffort` | low, high |
| DeepSeek | (none — reasoning always on) | — |
| Mistral | `reasoningEffort` | high only |
| Bedrock (Anthropic) | `reasoningConfig.type` + `budgetTokens` | "enabled" + token count |
| Bedrock (Nova) | `reasoningConfig.maxReasoningEffort` | low, medium, high |

**Key patterns:**
- OpenAI uses date-based release filtering for "none" and "xhigh" support (e.g., `OPENAI_NONE_EFFORT_RELEASE_DATE = "2025-11-13"`)
- GPT-5 family has complex per-variant effort sets (codex, chat, pro)
- Anthropic adaptive thinking uses `thinking.type: "adaptive"` instead of fixed budget
- DeepSeek and MiniMax M3 have no/limited reasoning control — returned as empty `{}`
- Each model has `supported_reasoning_efforts` and `default_reasoning_effort` in metadata

## Research: codex (`/home/marius/codex/codex-rs/`)

Codex uses the OpenAI Responses API format with a nested `reasoning` object.

**Request body format:**
```json
{
  "model": "o3",
  "reasoning": {
    "effort": "high",
    "summary": "auto",
    "context": "all_turns"
  }
}
```

**ReasoningEffort enum** (`codex-rs/protocol/src/openai_models.rs`):
- None, Minimal, Low, Medium (default), High, XHigh, Max, Ultra, Custom(String)
- `Ultra` is clamped to `Max` before sending: `fn reasoning_effort_for_request(effort) -> effort`
- `Custom(String)` provides forward-compatible passthrough for unknown future values

**Config structure** (`codex-rs/config/src/profile_toml.rs`):
- `model_reasoning_effort: Option<ReasoningEffort>` — per-profile default
- `plan_mode_reasoning_effort: Option<ReasoningEffort>` — plan mode override
- `model_reasoning_summary: Option<ReasoningSummary>` — reasoning summary control

**Model metadata** (`ModelPreset` / `ModelInfo`):
- `default_reasoning_effort: ReasoningEffort` — model's default level
- `supported_reasoning_efforts: Vec<ReasoningEffortPreset>` — allowed levels with descriptions

**Fallback chain:**
1. User config (`model_reasoning_effort` from config.toml)
2. Model metadata default (`model_info.default_reasoning_level`)
3. Neither → `None` (server default)

**Keybindings** (`TuiChatKeymap`):
- `decrease_reasoning_effort` / `increase_reasoning_effort` — cycles through supported levels
- Levels come from `supported_reasoning_efforts` on the active model preset

**BlazeAI adaptation notes:**
- Codex uses Responses API (nested `reasoning` object); BlazeAI uses Chat Completions (flat `reasoning_effort`)
- For Chat Completions: send `reasoning_effort` as a flat string field in request body
- `Ultra` → clamp to `Max` before sending
- `Custom(String)` passthrough for forward compatibility
- Validate against `supported_reasoning_efforts` per model; error if invalid
