# First-Run Setup

## Source Files

| File | Role |
|------|------|
| `firstrun.go` | Interactive setup: provider selection, API key, model retrieval, role assignment |
| `firstrun_test.go` | Unit tests for all interactive steps |
| `main.go` | Detection, setup trigger, startup sequence |
| `internal/skills/config-manager.md` | Builtin skill for LLM-assisted reconfiguration |

## Overview

First-run setup triggers when the config is missing or `default` role is
unassigned. It is an interactive console wizard that guides the user through
provider selection, API key entry, model retrieval, and role assignment.

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
  │    ├─ Option: custom provider
  │    └─ Return selected Provider{Name, Endpoint}
  │
  ├─ Step 2: API key entry
  │    ├─ Prompt: "Enter API key for <provider>"
  │    ├─ Read line from stdin
  │    ├─ Trim whitespace
  │    └─ Error if empty
  │
  ├─ Step 3: Model retrieval
  │    ├─ provider.NewClientRaw(endpoint, apiKey)
  │    ├─ client.ListModels() → retrieve from /v1/models
  │    ├─ Sort alphabetically
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

Option 16: Custom — prompts for name, endpoint, API key manually.

## Model Retrieval

Uses `provider.NewClientRaw(endpoint, apiKey)` to create a temporary client
(session-independent) and calls `ListModels()` which hits the provider's
`/v1/models` endpoint.

Models are sorted alphabetically for consistent display. The user selects by
number, and the result is formatted as `provider_name/model_id`.

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
- `app_home/config/config.json` — providers, roles, models, compaction settings
- `app_home/config/modes.json` — default work mode

Permissions:
- `config.json`: 0600 (owner read/write only — contains API keys)
- `modes.json`: 0644

## Reconfiguration with config-manager

After first-run, the builtin `config-manager` skill provides LLM-assisted
reconfiguration for:
- Adding/modifying providers and API keys
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
