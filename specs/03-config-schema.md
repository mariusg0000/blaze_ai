# Config Schema

## Sources

| File | Anchor | Role |
|------|--------|------|
| `internal/config/config.go` | `Config`, `ModelAdapterCatalog`, `ResolveModelAdapter`, `Validate`, `LoadFrom` | Config tree, separate adapter catalog, resolution, load/save/validation, migration, first-run detection |
| `internal/config/builtin_model_adapters.json` | top-level `providers` and `adapters` | Embedded provider contracts and builtin model overrides |
| `internal/config/modes.go` | `ModesConfig`, `LoadModes` | Work-mode persistence, validation, migration |
| `firstrun.go` | `firstRunModelDefinition`, `firstRun` | First-run provider/model setup and catalog construction |
| `internal/config/config_test.go` | `TestValidateModelCatalogValidEntries` | Catalog protocol/provider/variant validation contract |

## File Locations

| File | Path | Purpose |
|------|------|---------|
| `config.json` | `app_home/config/config.json` | Providers, roles, compaction, reasoning, preferences; no active model catalog |
| `model_adapters.json` | `app_home/config/model_adapters.json` | User model adapter catalog under top-level `adapters` |
| `modes.json` | `app_home/config/modes.json` | Work modes (name/model/directive), persisted active mode |

Separation rationale: `modes.json` contains frequently-edited mode data, isolated from the sensitive provider/API key data in `config.json`. `modes.json` is saved atomically via temp-file write to reduce corruption risk.

## Config (config.json)

### JSON Structure

```json
{
  "providers": [
    {
       "name": "openrouter",
       "endpoint": "https://openrouter.ai/api/v1",
      "api_key": "sk-..."
    }
  ],
  "favorite_models": ["openrouter/gpt-4o"],
  "roles": {
    "default": "openrouter/gpt-4o",
    "vision": "openrouter/gpt-4o",
    "summarization": "openrouter/gpt-4o",
    "advisor": "openrouter/gpt-4o"
  },
  "compaction": {
    "maxContextTokens": 100000,
    "minContextTokens": 50000,
    "summaryMaxTokens": 2000,
    "maxSummaryFiles": 10,
    "tokenCoefficient": 3.5,
    "maxBackoffOffsetTokens": 25000
  },
  "stripReasoning": {
    "enable": true,
    "preserveLast": 5
  },
  "last_model": "openrouter/gpt-4o",
  "helperSetup": {
    "dismissed": false,
    "declined": []
  },
  "debugPrompt": false
}
```

### Go Type

```go
type Config struct {
    Providers      []Provider     `json:"providers"`
    AdapterCatalog ModelAdapterCatalog `json:"-"`
    LegacyModels   map[string]ModelDefinition `json:"models,omitempty"` // migration-only
    FavoriteModels []string       `json:"favorite_models"`
    Roles          Roles          `json:"roles"`
    Compaction     Compaction     `json:"compaction"`
    StripReasoning StripReasoning `json:"stripReasoning"`
    LastModel      string         `json:"last_model,omitempty"`
    HelperSetup    HelperSetup    `json:"helperSetup,omitempty"`
    DebugPrompt    bool           `json:"debugPrompt,omitempty"`
}
```

### Model Adapter Catalog (`model_adapters.json`)

`Config.AdapterCatalog` is loaded from the separate file and serialized as `{ "adapters": { ... } }`. Each `ModelDefinition` records `Protocol`, `Capabilities` (`tools`, `reasoning`), and exactly one protocol variant: `OpenAIChat` (`include_stream_usage`, `include_reasoning_content`) or `Responses` (`lite`). Supported protocols are `openai-chat` and `openai-responses`; user entries may be exact IDs or terminal model-prefix wildcards.

User resolution is exact match, then longest matching literal-prefix wildcard. If no user entry matches, the embedded catalog is checked by exact/longest-prefix model override and then exact configured provider identity. Builtins are keyed by provider name, never endpoint or alias: `openrouter` uses Chat without usage/reasoning history; `opencode-go` and `opencode-zen` use Chat with both; `google-gemini` uses Chat without both; and `openai-chatgpt-oauth` uses Responses with the `gpt-5.6-*` Lite override. A missing match is an explicit error.

Validation requires user adapter patterns to use `provider/model` syntax, with at most one terminal `*`, and an existing configured provider. `openai-chat` requires a non-OAuth provider and `openai_chat`; `openai-responses` requires an OAuth provider and `responses`; contradictory variants and unknown protocols are rejected. Roles, favourites, and non-empty `last_model` must resolve through the user/builtin catalog; protocol and capabilities are never inferred from model names.

### Provider

```go
type Provider struct {
    Name     string            `json:"name"`                  // unique identifier, e.g. "openai"
    Endpoint string            `json:"endpoint"`              // base URL
    APIKey   string            `json:"api_key,omitempty"`      // API-key credential
    AuthType string            `json:"auth_type,omitempty"`    // "oauth" for OAuth providers
    OAuth    *OAuthCredential  `json:"oauth,omitempty"`        // refreshable OAuth credential
}

type OAuthCredential struct {
    IDToken      string `json:"id_token,omitempty"`
    AccessToken  string `json:"access_token,omitempty"`
    RefreshToken string `json:"refresh_token"`
    APIKey       string `json:"api_key,omitempty"`
    ExpiresAt    int64  `json:"expires_at,omitempty"`
    AccountID    string `json:"account_id,omitempty"`
}
```

Validation rules:
- `Name` must be non-empty and unique (no duplicate names)
- `Endpoint` must be non-empty
- API-key providers must have a non-empty `APIKey`
- OAuth providers must have `AuthType: "oauth"` and a non-empty refresh token
- ChatGPT OAuth login persists the identity token, access token, refresh token, account ID, and the token-exchange API key

### Roles

```go
type Roles struct {
    Default       string `json:"default"`                 // required
    Vision        string `json:"vision,omitempty"`        // optional
    Summarization string `json:"summarization,omitempty"` // optional
    Advisor       string `json:"advisor,omitempty"`       // optional
}
```

- All model values must be in `provider/model_name` format
- All model values must reference an existing provider
- All model values must reference an existing model catalog entry
- `Default` is required — runtime fails if empty
- Other roles are optional (empty string = not configured)

### Compaction

```go
type Compaction struct {
    MaxContextTokens       int     `json:"maxContextTokens"`       // 100000
    MinContextTokens       int     `json:"minContextTokens"`       // 50000
    SummaryMaxTokens       int     `json:"summaryMaxTokens"`       // 2000
    MaxSummaryFiles        int     `json:"maxSummaryFiles"`        // 10
    TokenCoefficient       float64 `json:"tokenCoefficient"`       // 3.5
    MaxBackoffOffsetTokens int     `json:"maxBackoffOffsetTokens"` // 25000
}
```

Defaults provided by `DefaultCompaction()`:

| Field | Default | Description |
|-------|---------|-------------|
| `maxContextTokens` | 100000 | Trigger point for compaction |
| `minContextTokens` | 50000 | Target size after pruning |
| `summaryMaxTokens` | 2000 | Token budget for summarizer |
| `maxSummaryFiles` | 10 | Max summary chunks per session |
| `tokenCoefficient` | 3.5 | Char-to-token divisor for local estimator |
| `maxBackoffOffsetTokens` | 25000 | Max offset above base (hard cap = maxContextTokens + this) |

### StripReasoning

```go
type StripReasoning struct {
    Enable       bool `json:"enable"`       // true — strip old reasoning from LLM payload
    PreserveLast int  `json:"preserveLast"` // 5 — keep newest N reasoning parts
}
```

Defaults: `Enable: true`, `PreserveLast: 5`.

### HelperSetup

```go
type HelperSetup struct {
    Dismissed bool     `json:"dismissed"`          // suppress all helper install prompts
    Declined  []string `json:"declined,omitempty"` // helpers explicitly declined by user
}
```

Distinct from live detection: this stores UX preferences only. Actual binary presence is detected at runtime via `exec.LookPath`.

### Other Fields

- `debugPrompt` (boolean, optional, default `false`) — writes `prompt.json` before each LLM call only when enabled; intended for debugging and session audits.
- `last_model` (string, optional) — persists the last selected model across sessions. Format: `provider/model_name`. Used as fallback when no active mode is set.
- Model IDs are plain `provider/model_name` strings; providers use their default reasoning behavior.

## Modes (modes.json)

### JSON Structure

```json
{
  "modes": [
    {
      "name": "default",
      "model": "openai/gpt-4o",
      "directive": "Be concise and direct. Prefer shell execution."
    },
    {
      "name": "planning",
      "model": "openai/o3",
      "directive": "Think step by step before executing.",
      "denied_tools": ["shell", "replace_block", "write_file"],
      "agents": ["coder", "worker"]
    }
  ],
  "last_mode": "default"
}
```

### Go Type

```go
type ModesConfig struct {
    Modes    []Mode `json:"modes"`              // work mode definitions
    LastMode string `json:"last_mode,omitempty"` // persisted active mode name
}

type Mode struct {
    Name        string   `json:"name"`                  // unique mode identifier
    Model       string   `json:"model"`                 // provider/model_name
    Directive   string   `json:"directive,omitempty"`   // injected into last LLM message (volatile)
    DeniedTools []string `json:"denied_tools,omitempty"` // tools unavailable to main runtime
    Agents      []string `json:"agents,omitempty"`       // one-shot agents the runtime may call
}
```

### Validation

Basic (structural, no provider data needed):
- No empty mode names
- No duplicate mode names
- No empty mode models
- Malformed provider/model_name rejected

Full (with provider data from config):
- All mode models reference existing providers
- `last_mode` references an existing mode name
- `denied_tools` references only known tool names (validated at runtime by `mode_capabilities.go`)
- `agents` references only known one-shot agent definitions (validated at runtime by `mode_capabilities.go`)

### Save Behavior

- Atomic save via temp file + rename
- Corruption/missing file fallback: creates `DefaultMode(modelID)` with a single "default" mode
- If `last_mode` is dangling (mode deleted), runtime falls back to first mode
- `modes.json` is saved to disk on every mode switch, model change in a mode, or mode creation/deletion

### Default Mode

```go
func DefaultMode(modelID string) *ModesConfig {
    return &ModesConfig{
        Modes:    []Mode{{Name: "default", Model: modelID}},
        LastMode: "default",
    }
}
```

Created when:
- `modes.json` does not exist (first start)
- `modes.json` is empty or corrupted
- All modes have been deleted
- Migration from legacy config-embedded modes produced no modes

## First-Run Detection

`NeedsFirstRun()` returns true if:
- `config.json` does not exist (`ErrConfigMissing`)
- `config.json` exists but `Roles.Default` is empty (`ErrDefaultRoleUnassigned`)

If `config.json` exists but is malformed (invalid JSON), `NeedsFirstRun()` returns an error — the runtime stops with a clear message rather than entering the first-run flow with partial data.

First-run retrieves the provider's model list, assigns explicit adapter metadata for selected role models, and saves `config.json` plus `model_adapters.json`. When a selected model resolves through a builtin contract, no redundant user adapter is written; otherwise an exact user adapter is created. OAuth models use Responses metadata and determine the Lite variant from the model name only in this first-run construction path.

## Validation Sequence

```
Config.Load(path)
  ├─ Read file → parse JSON
  ├─ Validate():
  │    ├─ Roles.Default non-empty
   │    ├─ Model adapter catalog: pattern, provider, protocol, auth, and variant consistency
   │    ├─ All role/favorite/last_model values: format + provider + resolved adapter
   │    ├─ Providers: non-empty fields, unique names
   │    └─ Return Config or the first specific validation error
  └─ Return Config or error
```

No silent fallbacks on validation failure. The runtime stops with the specific validation error.

## Model Adapter Migration

When `config.json` has a non-empty legacy `models` object and `model_adapters.json` is absent, `LoadFrom` performs one exact migration into the separate adapter catalog and persists `config.json` without `models`. If both sources are present with legacy entries, loading stops with a conflict; if neither provides adapters required by a non-builtin model, loading stops with a missing-catalog error. `SaveTo` refuses to persist legacy models.

`ReloadModelAdapters()` reloads the strict on-disk configuration and replaces only the in-memory adapter catalog. A failed reload leaves the current catalog unchanged.
