[DESCRIPTION]
Load when the user wants to configure BlazeAI models, providers, API keys, favorite models, role assignments, or work modes. Use for any config.json changes including creating, editing, or deleting modes.

[DETAILS]
# Customize Me

## Config Location
- Runtime configuration lives at {APP_HOME}/config/config.json.
- The config file is the single source of truth for providers, models, and roles.

## Config Structure
```json
{
  "providers": [
    {
      "name": "provider_name",
      "endpoint": "https://api.example.com/v1",
      "api_key": "sk-..."
    }
  ],
  "favorite_models": [
    "provider_name/model_name"
  ],
  "roles": {
    "default": "provider_name/model_name",
    "vision": "provider_name/model_name",
    "summarization": "provider_name/model_name"
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
  }
}
```

## Provider Definition
- Each provider has exactly three fields: `name`, `endpoint`, `api_key`.
- Only OpenAI-compatible providers are supported.
- No fallback providers. No automatic provider switching on failure.

## Model Roles
- `default`: required. Used for normal agent interaction and summarization (initial implementation).
- `vision`: optional. Intended for vision tasks.
- `summarization`: optional. Intended for summarization tasks (future use).

## How To Edit
1. Read the current config with the `shell` tool.
2. Modify the JSON with the `shell` tool (use replace_block or direct file editing).
3. **Mode changes are hot-reloaded**: new/edited modes are available immediately via Tab cycling. No restart needed.
4. Provider and role changes require a session restart.
5. The `/model` command changes the current model (NOT the mode). `/model` does NOT accept mode names; it only accepts `provider/model_name`.

## Fetching Models from Providers

When the user asks to browse models (e.g. "show deepseek models"), follow this process.

### Algorithm
1. Check if a helper script already exists at `{APP_HOME}/scripts/fetch_models`. Reuse it if present.
2. If not, create it on the fly using available shell tools. Python is last resort.
3. The script reads config.json to find the requested provider's endpoint and API key.
4. Calls `<endpoint>/models` with the key in the Authorization header.
5. Parses the JSON response (`data[].id`), filters by the search string (case-insensitive).
6. Outputs one `provider/model_id` per line.

### Creation guidelines
- Write the script to `{APP_HOME}/scripts/fetch_models` with OS-appropriate extension (.sh, .ps1, .py).
- Accept two arguments: `<provider_name>` and `[filter]`.
- API keys must be read from disk — never hardcoded in the script.
- Make it executable (`chmod +x` on Unix).

### Usage
Call per provider: `fetch_models <provider_name> <filter>`.

### Presenting results
1. If user specified a provider: query just that provider.
2. If not: query providers sequentially until matches are found.
3. Show results as a numbered list. Ask the user to pick.
4. Use the selected ID directly — it is already in `provider/model` format.

## Work Modes (config.json)
Modes are part of the runtime config at {APP_HOME}/config/config.json. Each mode
binds a model and an optional directive that is injected into the last message
sent to the LLM (volatile — not stored in session history).

### Structure
```json
"modes": [
  {
    "name": "default",
    "model": "provider/model_name",
    "directive": ""
  },
  {
    "name": "planning",
    "model": "openai/gpt-4o",
    "directive": "You are in planning mode. Use only read-only tools and propose a plan."
  }
],
"last_mode": "default"
```

### Rules
- `name`: unique, non-empty.
- `model`: must exist in favorite_models and reference a configured provider.
- `directive`: free text. Empty string = no directive injected.
- At least one mode must exist (the `default` mode is pre-created on first run).
- `last_mode`: persists the active mode between sessions; must match an existing mode name.

### Operations you can perform
- Create a new mode: append an entry to `modes` and persist config. Validate with the rules above.
- Edit a mode's directive or model: find by `name`, update, persist.
- Delete a mode: remove from `modes`. If it was `last_mode`, set `last_mode` to the first remaining mode. Never delete the last remaining mode.
- Switch active mode at runtime: the user presses Tab to cycle modes. Newly created modes are hot-reloaded and appear in the cycle immediately. Do NOT suggest `/model modename` — that command does not switch modes.
- After any edit, validate config integrity (unique names, valid models, provider references).

### Directive behavior
The directive is appended to the last message of the payload sent to the LLM on every LLM call while the mode is active. It is not stored in session.json. Use it to constrain agent behavior for the current task (e.g. read-only, quick/cheap, verbose, etc.). Keep directives short and imperative. Always write directives in English, even when the user communicates in another language — the directive is injected into the LLM prompt and must be understood by the model.
