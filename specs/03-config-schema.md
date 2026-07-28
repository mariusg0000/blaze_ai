# Config Schema

## Sources

| File | Anchor | Role |
|------|--------|------|
| `internal/config/config.go` | `Config`, `ModelDefinition`, `Validate` | Config tree, model catalog, load/save/validation, first-run detection |
| `internal/config/modes.go` | `ModesConfig`, `LoadModes` | Work-mode persistence, validation, migration |
| `firstrun.go` | `firstRunModelDefinition`, `firstRun` | First-run provider/model setup and catalog construction |
| `internal/config/config_test.go` | `TestValidateModelCatalogValidEntries` | Catalog protocol/provider/variant validation contract |

## File Locations

| File | Path | Purpose |
|------|------|---------|
| `config.json` | `app_home/config/config.json` | Providers, roles, models, compaction, reasoning, preferences |
| `modes.json` | `app_home/config/modes.json` | Work modes (name/model/directive), persisted active mode |

Separation rationale: `modes.json` contains frequently-edited mode data, isolated from the sensitive provider/API key data in `config.json`. `modes.json` is saved atomically via temp-file write to reduce corruption risk.

## Config (config.json)

### JSON Structure

```json
{
  "providers": [
    {
      "name": "openai",
      "endpoint": "https://api.openai.com/v1",
      "api_key": "sk-..."
    }
  ],
  "models": {
    "openai/gpt-4o": {
      "protocol": "openai-chat",
      "capabilities": {"tools": true, "reasoning": false},
      "openai_chat": {"include_stream_usage": true, "include_reasoning_content": true}
    }
  },
  "favorite_models": ["openai/gpt-4o"],
  "roles": {
    "default": "openai/gpt-4o",
    "vision": "openai/gpt-4o",
    "summarization": "openai/gpt-4o",
    "advisor": "openai/gpt-4o"
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
  "last_model": "openai/gpt-4o",
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
    Models         map[string]ModelDefinition `json:"models"`
    FavoriteModels []string       `json:"favorite_models"`
    Roles          Roles          `json:"roles"`
    Compaction     Compaction     `json:"compaction"`
    StripReasoning StripReasoning `json:"stripReasoning"`
    LastModel      string         `json:"last_model,omitempty"`
    HelperSetup    HelperSetup    `json:"helperSetup,omitempty"`
    DebugPrompt    bool           `json:"debugPrompt,omitempty"`
}
```

### Model Catalog

`Config.Models` is an explicit catalog keyed by the plain `provider/model_name` identifier. Each `ModelDefinition` records `Protocol`, `Capabilities` (`tools`, `reasoning`), and exactly one protocol variant: `OpenAIChat` (`include_stream_usage`, `include_reasoning_content`) or `Responses` (`lite`). The supported protocol constants are `openai-chat` and `openai-responses`.

Validation requires every catalog key to have valid model syntax and an existing provider. `openai-chat` requires a non-OAuth provider and an `openai_chat` variant; `openai-responses` requires an OAuth provider and a `responses` variant. Each configured role, favourite, and non-empty `last_model` must also resolve to a catalog entry. Unknown protocols and contradictory variants are rejected; the provider client does not infer protocol from model names.

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

## Validation Sequence

```
Config.Load(path)
  ├─ Read file → parse JSON
  ├─ Validate():
  │    ├─ Roles.Default non-empty
   │    ├─ Model catalog: format, provider, protocol, auth, and variant consistency
   │    ├─ All role/favorite/last_model values: format + provider + catalog reference
   │    ├─ Providers: non-empty fields, unique names
   │    └─ Return Config or the first specific validation error
  └─ Return Config or error
```

No silent fallbacks on validation failure. The runtime stops with the specific validation error.

## Migration

`MigrateFromConfig()` extracts legacy mode definitions embedded in `config.json` (from an earlier version where modes lived inside the config struct) into the separate `modes.json`. Called on every `NewAgent()` startup.

After migration, `config.json` should no longer contain inline mode data. Migration is idempotent — runs every startup but only writes modes.json if it does not yet exist.
