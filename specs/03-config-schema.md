# Config Schema

## Source Files

- `internal/config/config.go` — Config struct tree, Load/Save/Validate, first-run detection, helpers
- `internal/config/modes.go` — ModesConfig, LoadModes/SaveModes, validation, migration
- `firstrun.go` — first-run interactive setup

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
  "favorite_models": ["openai/gpt-4o", "openai/gpt-4o-mini"],
  "roles": {
    "default": "openai/gpt-4o",
    "vision": "openai/gpt-4o",
    "summarization": "openai/gpt-4o-mini",
    "advisor": "openai/o3"
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
  "showReasoning": false
}
```

### Go Type

```go
type Config struct {
    Providers      []Provider     `json:"providers"`
    FavoriteModels []string       `json:"favorite_models"`
    Roles          Roles          `json:"roles"`
    Compaction     Compaction     `json:"compaction"`
    StripReasoning StripReasoning `json:"stripReasoning"`
    LastModel      string         `json:"last_model,omitempty"`
    HelperSetup    HelperSetup    `json:"helperSetup,omitempty"`
    ShowReasoning  bool           `json:"showReasoning"`
}
```

### Provider

```go
type Provider struct {
    Name     string `json:"name"`     // unique identifier, e.g. "openai"
    Endpoint string `json:"endpoint"` // base URL, e.g. "https://api.openai.com/v1"
    APIKey   string `json:"api_key"`  // secret key, stored directly in config.json
}
```

Validation rules:
- `Name` must be non-empty and unique (no duplicate names)
- `Endpoint` must be non-empty
- `APIKey` must be non-empty

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

- `last_model` (string, optional) — persists the last selected model across sessions. Format: `provider/model_name`. Used as fallback when no active mode is set.
- `showReasoning` (bool, default false) — toggle streaming of reasoning/thinking tokens to the user

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
      "directive": "Think step by step before executing."
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
    Name      string `json:"name"`              // unique mode identifier
    Model     string `json:"model"`             // provider/model_name
    Directive string `json:"directive,omitempty"` // injected into last LLM message (volatile)
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
  │    ├─ All model values: provider/model_name format
  │    ├─ All model values: provider references valid
  │    ├─ Providers: non-empty fields, unique names
  │    └─ FavoriteModels: format + provider check
  └─ Return Config or error
```

No silent fallbacks on validation failure. The runtime stops with the specific validation error.

## Migration

`MigrateFromConfig()` extracts legacy mode definitions embedded in `config.json` (from an earlier version where modes lived inside the config struct) into the separate `modes.json`. Called on every `NewAgent()` startup.

After migration, `config.json` should no longer contain inline mode data. Migration is idempotent — runs every startup but only writes modes.json if it does not yet exist.
