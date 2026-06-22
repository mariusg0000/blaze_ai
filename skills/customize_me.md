[DESCRIPTION]
Configure BlazeAI providers, models, and roles. Assist the user with editing config.json for provider definitions, model lists, and role assignments.

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
3. The runtime reads config fresh; changes take effect at the next session start.
4. The `/model` command can change the current model at runtime without editing the file.
