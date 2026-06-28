[DESCRIPTION]
Load for BlazeAI auto-configuration: providers, API keys, favorite models, role assignments, work modes, Telegram bridge, and host helpers.

[BEHAVIOR]
# Customize Me

## Config Location
- **Application home (`{APP_HOME}`)** contains `backups`, `config`, `projects`, `scripts`, `skills`.
- Each top-level folder has a `README.md` that documents its structure, use, and rules.
- Before inspecting or modifying any other file in one of those folders, read that folder's `README.md` first.
- Runtime configuration lives at `{APP_HOME}/config/config.json` — providers, models, roles, compaction, reasoning.
- Work modes live separately at `{APP_HOME}/config/modes.json` — mode definitions and last active mode.
- API keys are stored in config.json. Modes reference provider/model names but never contain keys.

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
    "summarization": "provider_name/model_name",
    "advisor": "provider_name/model_name"
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
- `default`: required. Used for normal agent interaction and core runtime work.
- `vision`: optional. Intended for vision tasks.
- `summarization`: optional. Intended for summarization and compact review tasks. Used by `ask_a_friend(role="summarization")` for per-session learning reports.
- `advisor`: optional. Intended for one-shot delegated analysis to a stronger model. Used by `ask_a_friend(role="advisor")` for cross-session synthesis and the `session-learning-review` meta-review step.

### Role Configuration Rules
- Each role must reference a valid `provider_name/model_name` that exists in the configured providers.
- The same model can serve multiple roles (e.g., `summarization` and `advisor` can both point to the default model).
- If `summarization` or `advisor` is unset, `ask_a_friend` calls targeting that role will fail with `model role is not configured: <role>`. No fallback is attempted.
- The `session-learning-review` workflow depends on both `summarization` and `advisor`. If either is missing, the workflow stops at the first `ask_a_friend` call that needs it.

### Recommended Configuration
```json
"roles": {
  "default": "provider/main-model",
  "vision": "provider/vision-model",
  "summarization": "provider/summarizer-model",
  "advisor": "provider/advisor-model"
}
```

If a user does not have separate summarization or advisor models, reuse the default model:
```json
"roles": {
  "default": "provider/main-model",
  "summarization": "provider/main-model",
  "advisor": "provider/main-model"
}
```

After editing roles, the runtime holds config in memory. Restart BlazeAI (or the Telegram bridge) for changes to take effect.

## How To Edit
1. Read the current file with the `shell` tool.
2. Modify the JSON with the `shell` tool (use replace_block or direct file editing).
3. **config.json**: provider and role changes require a session restart.
4. **modes.json**: all changes (new, edit, delete) require restarting BlazeAI. The modes are loaded once at startup and never reloaded. Inform the user to exit and restart with `-c` to continue the current session.
5. Always validate JSON syntax and mode rules before saving.
6. The `/model` command changes the current model (NOT the mode). `/model` does NOT accept mode names; it only accepts `provider/model_name`.

## Linked Docs
- For Telegram bridge configuration, startup, or repair guidance, read `{SKILL_DIR}/docs/telegram.md` before acting.
- For host helper detection or install guidance, read `{SKILL_DIR}/docs/helpers.md` before acting.
- Keep the linked docs as the source of truth for those workflows. Do not duplicate their detailed steps here.

## Fetching Models from Providers

When the user asks to browse models, follow this process.

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
- Read API keys from disk. Never hardcode them in the script.
- Make it executable (`chmod +x` on Unix).

### Usage
Call per provider: `fetch_models <provider_name> <filter>`.

### Presenting results
1. If user specified a provider: query just that provider.
2. If not: query providers sequentially until matches are found.
3. Show results as a numbered list. Ask the user to pick.
4. Use the selected ID directly — it is already in `provider/model` format.

## Work Modes (modes.json)
Modes are stored in `{APP_HOME}/config/modes.json`, separate from config.json so frequently edited mode data stays isolated from critical provider and API key configuration. Each mode binds a model and an optional directive injected into the last message sent to the LLM. The directive is volatile and is not stored in session history.

### Structure
`modes.json` is a standalone JSON file. It is not embedded in `config.json`:
```json
{
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
}
```

### Rules
- `modes`: array of mode objects, at least one entry required.
- `name`: unique, non-empty.
- `model`: must exist in favorite_models and reference a configured provider.
- `directive`: free text. Empty string = no directive injected.
- `last_mode`: persists the active mode between sessions; must match an existing mode name.
- At least one mode must exist (the `default` mode is pre-created on first run).

### Operations you can perform
- Create a new mode: append an entry to the `modes` array in modes.json and persist. Validate with the rules above.
- Edit a mode's directive or model: find by `name`, update in modes.json, persist.
- Delete a mode: remove from `modes` array in modes.json. If it was `last_mode`, set `last_mode` to the first remaining mode. Never delete the last remaining mode.
- Switch active mode at runtime: the user presses Tab to cycle through modes loaded at startup. Do NOT suggest `/model modename` — that command does not switch modes.
- After any edit to modes.json, validate integrity (unique names, valid models, provider references).

After creating or editing a mode, remind the user that mode changes take effect only after restarting BlazeAI. Suggest restarting with the `-c` flag to continue the current session.

### Directive behavior
The directive is appended to the last message of the payload sent to the LLM on every call while the mode is active. It is not stored in `session.json`. Use it to constrain agent behavior for the current task. Keep directives short and imperative.

Write the directive in English only. Never include translations, dual-language content, separator labels like `[MODE DIRECTIVE]`, or non-English text. The directive is for the LLM, not the user. Even if the user speaks another language, the directive must be a single block of English text.

For skill creation, editing, scoping, or restoration, load the `skill-manager` skill. Customize the skill-manager itself via `skill-manager` too.
For Telegram bridge instance creation, editing, systemd setup, or verification, read `{SKILL_DIR}/docs/telegram.md` and follow it.
For missing helper detection, approval-safe install guidance, or helper reminder dismissal, read `{SKILL_DIR}/docs/helpers.md` and follow it.
