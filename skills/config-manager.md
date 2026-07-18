[DESCRIPTION]
Load for BlazeAI auto-configuration: providers, API keys, favorite models, role assignments, work modes, Telegram bridge, and host helpers.

[BODY]
# Config Manager

## Config Location
- **Application home (`{APP_HOME}`)** contains `backups`, `config`, `projects`, `scripts`, `skills`.
- Each top-level folder has a `README.md` that documents its structure, use, and rules.
- Before inspecting or modifying any other file in one of those folders, read that folder's `README.md` first.
- Runtime configuration lives at `{APP_HOME}/config/config.json` — providers, models, roles, compaction, reasoning.
- Work modes live separately at `{APP_HOME}/config/modes.json` — mode definitions and last active mode.
- Provider credentials are stored in config.json. API-key providers use `api_key`; ChatGPT OAuth uses `auth_type: "oauth"` and stores the identity, access, refresh, account, and token-exchange API-key credentials. Modes reference provider/model names but never contain credentials.

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
- API-key providers have `name`, `endpoint`, and `api_key`.
- OAuth providers have `name`, `endpoint`, `auth_type: "oauth"`, and OAuth credentials.
- Only OpenAI-compatible providers are supported.
- Configure or authenticate providers from the primary console. Use `/auth openai` for ChatGPT browser OAuth.
- No fallback providers. No automatic provider switching on failure.

## Model Roles
- `default`: required. Used for normal agent interaction and core runtime work.
- `vision`: optional. Intended for vision tasks.
- `summarization`: optional. Intended for summarization and compact review tasks. Used by `ask_a_friend(role="summarization")` for per-session review reports.
- `advisor`: optional. Intended for one-shot delegated analysis to a stronger model. Used by `ask_a_friend(role="advisor")` for cross-session synthesis and the `audit-manager` meta-review step.

### Role Configuration Rules
- Each role must reference a valid `provider_name/model_name` that exists in the configured providers.
- The same model can serve multiple roles (e.g., `summarization` and `advisor` can both point to the default model).
- If `summarization` or `advisor` is unset, `ask_a_friend` calls targeting that role will fail with `model role is not configured: <role>`. No fallback is attempted.
- The `audit-manager` workflow depends on both `summarization` and `advisor`. If either is missing, the workflow stops at the first `ask_a_friend` call that needs it.

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

# Telegram Bridge Guide

Use this document when the user needs Telegram bridge instance creation, editing, inspection, repair, startup, or verification.

## Required Values
- `instance` name for `{APP_HOME}/telegram/<instance>/`
- Telegram bot token
- allowed chat id
- absolute existing `workdir`
- selected model in `provider/model_name` format

If any required value is missing, stop and ask. Do not invent defaults.

## Required Reads
- Read `{APP_HOME}/telegram/README.md` before touching files under `{APP_HOME}/telegram/`.
- Read `{APP_HOME}/config/README.md` and `{APP_HOME}/config/config.json` before validating models.

## Bot And Chat Setup
- Create the bot with BotFather: `/newbot`, choose display name, choose unique username ending in `bot`, capture returned token.
- Treat the token as a secret.
- To discover the allowed chat id, ask the user to send a message to the bot, then call `https://api.telegram.org/bot<token>/getUpdates` and read `message.chat.id`.
- Keep the numeric sign exactly as returned. Do not quote `allowed_chat_id` in JSON.

## Instance Files
- Instance directory: `{APP_HOME}/telegram/<instance>/`
- Static config: `{APP_HOME}/telegram/<instance>/bridge.json`
- Mutable state: `{APP_HOME}/telegram/<instance>/state.json`
- Session storage: `{APP_HOME}/telegram/<instance>/session/`

`bridge.json`:

```json
{
  "bot_token": "<telegram-bot-token>",
  "allowed_chat_id": 123456789,
  "workdir": "/absolute/project/path"
}
```

Rules:
- `bot_token` is required.
- `allowed_chat_id` is required and must be a non-zero integer.
- `workdir` is required and must be an absolute existing directory.
- Never default `workdir` to the current directory.

`state.json`:

```json
{
  "selected_model": "provider/model_name"
}
```

Rules:
- `selected_model` is required.
- It must reference a provider that already exists in `{APP_HOME}/config/config.json`.

## Workflow
1. Identify the instance name.
2. Read the required README and config files.
3. Gather or confirm bot token, chat id, workdir, and selected model.
4. Validate all required values before writing any file.
5. If editing an existing instance, read current `bridge.json` and `state.json` first.
6. Write valid minimal JSON only.
7. Re-read the written files or otherwise validate the saved content.

## Startup And Services
- Start one instance with `blazeai --telegram <instance>`.
- Startup must fail if any required Telegram file or field is missing or invalid.

### Linux systemd
- Use one service per instance.
- Run the service as the same user that owns `{APP_HOME}`.
- Prefer an explicit absolute binary path and an explicit instance argument.

```ini
[Unit]
Description=BlazeAI Telegram bridge (%i)
After=network-online.target

[Service]
Type=simple
User=blazeai
WorkingDirectory=/home/blazeai
ExecStart=/opt/blazeai/blazeai --telegram %i
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
```

- Enable and start with `systemctl enable --now blazeai-telegram@<instance>`.
- Check logs with `journalctl -u blazeai-telegram@<instance> -f`.
- Keep `Restart=always` so the service comes back even if the process exits after a signal.
- The bridge retries transient `getUpdates` transport errors in-process, so resets like `connection reset by peer` should not require a manual restart.

## Restart

Service `blazeai-telegram@<instance>` has `Restart=always`. Best restart method **without sudo**:

```sh
kill $(pgrep -f "blazeai.*--telegram <instance>")
```

systemd respawns instantly. Verify with `pgrep -f "blazeai.*--telegram <instance>"`.

If `sudo` is used, chain all commands in one call to prompt once:

```sh
sudo sh -c 'systemctl restart blazeai-telegram@<instance> && systemctl status blazeai-telegram@<instance> --no-pager'
```

## Verification
- Send a normal text message from the allowed chat.
- Confirm the bot responds.
- Confirm `/help` works.
- Confirm `/model` shows the current instance model.
- If another chat messages the bot, explain that the instance is expected to ignore it.

## Stop Conditions
- Stop and ask if one of `instance`, bot token, allowed chat id, workdir, or selected model is missing.
- Stop and ask if the requested model's provider does not exist.
- Stop and ask if `workdir` does not exist.
- Stop and ask before deleting an instance or replacing a token for an existing instance.

# Helper Setup Guide

Use this document when host tools are missing or the user asks about helper installation.

## Scope
- Configure helper utilities only when the user asks or when a missing helper would materially improve current work.
- Never install anything without explicit user approval.
- Never run `sudo` without explicit user approval per command.
- After installation, verify that the helper is usable before the next prompt build.

## Core Cross-Platform Helpers

| Helper | Purpose | Typical Install Command |
|--------|---------|-------------------------|
| rg | fast recursive content search | `apt install ripgrep` / `brew install ripgrep` / `winget install BurntSushi.ripgrep` |
| fd | fast file and directory discovery | `apt install fd-find` / `brew install fd` / `winget install sharkdp.fd` |
| jq | JSON inspection and transformation | `apt install jq` / `brew install jq` / `winget install jqlang.jq` |
| git | VCS operations | `apt install git` / `brew install git` / `winget install Git.Git` |
| curl | HTTP/API checks, downloads | `apt install curl` / `brew install curl` / built-in on modern Windows |
| pandoc | document conversion | `apt install pandoc` / `brew install pandoc` / `winget install JohnMacFarlane.Pandoc` |
| sqlite3 | lightweight SQL queries | `apt install sqlite3` / `brew install sqlite3` / `winget install SQLite.SQLite` |

## Detection

Before suggesting installation, verify what is already available.

```sh
# Linux / macOS
command -v rg && command -v fd && command -v jq && command -v git && command -v curl && command -v pandoc && command -v sqlite3

# Windows PowerShell
Get-Command rg -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
Get-Command fd -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
Get-Command jq -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
Get-Command git -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
Get-Command curl.exe -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
Get-Command pandoc -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
Get-Command sqlite3 -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
```

## Installation

### Linux
1. Detect package manager first: `which apt || which dnf || which pacman || which zypper || which apk`
2. Ask which helper(s) to install and confirm.
3. If `sudo` is required, ask separately. Never batch `sudo` commands without approval.
4. Example: `sudo apt update && sudo apt install -y ripgrep fd-find jq pandoc sqlite3`

### macOS
1. Check for Homebrew: `command -v brew`
2. If brew exists and user approves: `brew install ripgrep fd jq pandoc sqlite3`
3. If brew is missing, suggest installing it first and ask the user.

### Windows
1. Check for package manager: `winget --version` or `scoop` or `choco`
2. If winget exists and user approves, install the requested helpers.
3. If no package manager exists, explain that the user needs winget, scoop, or choco first.

## Verification After Install
- Re-run detection.
- If the helper still does not resolve, report failure and continue with available alternatives.
- Do not loop or retry without user instruction.

## Dismissing The Helper Reminder
- After all core helpers are installed and verified, set `helperSetup.dismissed` to `true` in `{APP_HOME}/config/config.json`.
- If the user wants to stop the reminder without installing the remaining helpers, also set `helperSetup.dismissed` to `true`.
- If the user wants to skip specific helpers permanently, add them to `helperSetup.declined`.

Example:

```json
"helperSetup": {
  "dismissed": true,
  "declined": ["fd"]
}
```

## Python Environment
- Python is not a host helper. It is a restricted runtime.
- If Python is truly necessary and `{APP_HOME}/scripts/venv/` does not exist yet, ask before creating it.
- Create it lazily with `python3 -m venv {APP_HOME}/scripts/venv`.
- All later Python usage must go through that venv.

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
For Telegram bridge instance creation, editing, systemd setup, or verification, follow the inline Telegram Bridge Guide above.
For missing helper detection, approval-safe install guidance, or helper reminder dismissal, follow the inline Helper Setup Guide above.
