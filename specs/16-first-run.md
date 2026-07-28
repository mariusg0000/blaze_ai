# First-Run Setup

## Sources

| File | Anchor | Role |
|------|--------|------|
| `firstrun.go` | `firstRun`, `firstRunModelDefinition` | Interactive setup and catalog construction |
| `firstrun_test.go` | `TestFirstRunModelDefinition`, `TestFirstRunFullFlow` | Catalog definition and saved-config contracts |
| `main.go` | `main` | Detection, setup trigger, startup sequence |
| `internal/skills/config-manager.md` | whole file | Builtin skill for LLM-assisted reconfiguration |
| `internal/config/config.go` | `ModelDefinition`, `Config.Models` | Catalog schema persisted by first-run |
| `internal/provider/openai_responses.go` | `IsResponsesLiteModel` | Exported Responses-lite classification predicate |

## Overview

First-run setup triggers when the config is missing or `default` role is
unassigned. It is an interactive console wizard that guides the user through
provider selection, authentication, model retrieval, and role assignment.

The setup lives in the root package (`package main`) because it calls
`os.Stdout`/`os.Stdin` directly. Test helpers accept explicit `io.Writer` and
`*bufio.Reader` for isolation.

## Trigger Condition

Checked at startup in `main.go`:

```go
if cfg == nil || config.NeedsFirstRun() {
    cfg, err = runFirstRun()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Setup failed: %v\n", err)
        os.Exit(1)
    }
}
```

`NeedsFirstRun()` returns true when:
- Config file does not exist (`config.json` missing)
- `Ranges.Default` is empty (no default role assigned)

## Setup Flow

```
runFirstRun()
  └─ firstRun(os.Stdout, bufio.NewReader(os.Stdin))

firstRun(out, reader)
  ├─ Print welcome banner
  │
  ├─ Step 1: selectProvider(out, reader)
  │    ├─ Display curated list (max 15)
  │    ├─ Option: ChatGPT OAuth browser login
  │    ├─ Option: custom provider
  │    └─ Return selected Provider{Name, Endpoint}
  │
  ├─ Step 2: provider authentication
  │    ├─ API-key provider: prompt for API key and reject empty input
  │    └─ ChatGPT OAuth: print authorization URL and wait for localhost callback
  │
  ├─ Step 3: Model retrieval
  │    ├─ API-key provider: client.ListModels() → retrieve from /v1/models
  │    ├─ ChatGPT OAuth: use the explicit Codex entitlement catalog
  │    ├─ Preserve the OAuth entitlement order; sort API models alphabetically
  │    └─ Error if retrieval fails or returns empty list
  │
  ├─ Step 4: selectModel(out, reader, models, providerName)
  │    ├─ Display sorted model list with numbers
  │    ├─ User selects by number
  │    └─ Return "provider/model_name"
  │
  ├─ Step 5: Build config
  │    ├─ config.Default() → pre-filled compaction thresholds
  │    ├─ Set single provider
   │    ├─ Set FavoriteModels = [modelID]
   │    ├─ Set Roles.Default = modelID
   │    ├─ Build explicit Config.Models[modelID] metadata
   │    │    ├─ API-key model → openai-chat + Chat variant/capabilities
   │    │    └─ OAuth model → openai-responses + Responses variant/capabilities
   │    └─ Set LastModel = modelID
  │
  ├─ Step 6: assignOptionalRoles(out, reader, models, providerName, cfg)
  │    ├─ For each role (vision, summarization, advisor):
  │    │    ├─ "Assign <role> role? (y/N): "
  │    │    ├─ If no → skip
  │    │    ├─ Display model list
  │    │    ├─ User selects by number
  │    │    └─ Set role + add to FavoriteModels
  │    └─ Invalid selection → skip with message, not error
  │
  ├─ cfg.Save() → writes config.json (0600)
  ├─ config.DefaultMode(modelID).Save() → writes modes.json
  └─ Print "Config saved. Default model: <model>"
```

## Provider List

Curated list of 15 providers matching the spec maximum:

| # | Name | Endpoint |
|---|------|----------|
| 1 | openrouter | `https://openrouter.ai/api/v1` |
| 2 | deepseek | `https://api.deepseek.com/v1` |
| 3 | openai | `https://api.openai.com/v1` |
| 4 | groq | `https://api.groq.com/openai/v1` |
| 5 | anthropic | `https://api.anthropic.com/v1` |
| 6 | together | `https://api.together.xyz/v1` |
| 7 | mistral | `https://api.mistral.ai/v1` |
| 8 | perplexity | `https://api.perplexity.ai` |
| 9 | fireworks | `https://api.fireworks.ai/inference/v1` |
| 10 | cohere | `https://api.cohere.ai/v1` |
| 11 | xai | `https://api.x.ai/v1` |
| 12 | hyperbolic | `https://api.hyperbolic.xyz/v1` |
| 13 | infermatic | `https://api.infermatic.ai/v1` |
| 14 | opencode-go | `https://opencode.ai/zen/go/v1` |
| 15 | lmstudio | `http://localhost:1234/v1` |

Option 16: ChatGPT OAuth — browser authentication for the ChatGPT/Codex provider.
Option 17: Custom — prompts for name, endpoint, API key manually.

## Model Retrieval

API-key providers use `provider.NewClientRaw(endpoint, apiKey)` and call
`ListModels()` against the provider's `/v1/models` endpoint. ChatGPT OAuth
uses the authenticated Codex `/models?client_version=...` endpoint and reads
the account-scoped `models[].slug` values returned by ChatGPT. First-run
preserves the provider order and assigns the first returned model as the
default role for the OAuth provider.

API models are sorted alphabetically for consistent display. The OAuth result
keeps its account-provided order and assigns its first model as the default.
The result is formatted as `provider_name/model_id`.

First-run writes the selected model's catalog definition explicitly. OAuth
Responses `lite` is selected through `provider.IsResponsesLiteModel(modelName)`;
protocol choice is supplied by the setup path and is not inferred later from a
model-name prefix. Optional role selections copy the default model definition
for the newly referenced catalog key.

## Role Assignment

### Default (Required)

Always assigned from the first model selection. Cannot be skipped.

### Optional Roles

- `vision` — for image-capable models
- `summarization` — for context compaction summarization (separate model)
- `advisor` — for `ask_a_friend` secondary calls

Each is prompted with `(y/N)` and defaults to no. Invalid selections (wrong
number, non-numeric) display a skip message and continue — they do not fail the
setup.

All selected optional models are appended to `FavoriteModels`.

## Output

Config files written:
- `app_home/config/config.json` — providers, credentials, roles, models, compaction settings
- `app_home/config/modes.json` — default work mode

Permissions:
- `config.json`: 0600 (owner read/write only — contains provider credentials)
- `modes.json`: 0644

## Reconfiguration with config-manager

After first-run, the builtin `config-manager` skill provides LLM-assisted
reconfiguration for:
- Adding/modifying providers and credentials
- Changing role assignments
- Editing modes
- Updating model selections

The skill uses replace_block on `config.json` and `modes.json` directly.
It is a builtin skill discovered at every startup.

## Testability

`firstRun(out, reader)` accepts test interfaces directly:
- `out io.Writer` — capture output for assertion
- `reader *bufio.Reader` — feed predetermined input sequences

Tests verify:
- Provider selection by number
- Custom provider input
- Model selection
- Optional role assignment (accept/decline)
- Invalid input handling (out of range, non-numeric)
- Complete config output structure
