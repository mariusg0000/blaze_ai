[DESCRIPTION]
Load for BlazeAI configuration: providers, API keys, models, roles, compaction, reasoning stripping, interactive and executor agents (the active mode system), Telegram bridges, and host helpers.

[BODY]
# Config Manager

This skill edits durable BlazeAI configuration and agent definitions. It must:

- Inspect the complete current file before changing it.
- Preserve unrelated valid settings.
- Write valid JSON or Markdown.
- Keep secrets out of output.
- Stop on missing required values or validation errors.
- Never invent API keys, provider endpoints, model IDs, tool names, agent names, Telegram IDs, or paths.

## Configuration Sources and Ownership

- `{APP_HOME}` is `<user-home>/blazeai` (`~/blazeai` on Unix, `%USERPROFILE%\blazeai` on Windows).
- Runtime-created subdirectories: `skills/`, `scripts/`, `backups/`, `projects/`, `config/`, `telegram/`, `agents/`.
- Manually editable sources:
  - `{APP_HOME}/config/config.json` — providers, favorites, roles, compaction, reasoning stripping, helper state, diagnostic prompt capture.
  - `{APP_HOME}/agents/*.md` — interactive and executor definitions.
  - `{APP_HOME}/telegram/<instance>/bridge.json` — Telegram credentials, authorization, and workdir.
- Runtime-managed state:
  - `{APP_HOME}/config/agents.json` — interactive agent model state and `last_agent`.
  - `{APP_HOME}/telegram/<instance>/state.json` — Telegram selected model and pending selection state.
- Legacy-only input:
  - `{APP_HOME}/config/modes.json` — one-time migration source, not current mode configuration.
- Persist JSON/state and generated agent files with private `0600` file mode.
- Manual `config.json` or `agents/*.md` edits require process restart. Runtime `/model`, `/mode`, favorite commands, and Tab act immediately and persist through the runtime-owned files.

## Global config.json

Location: `{APP_HOME}/config/config.json`

Complete template with placeholder values:

```json
{
  "providers": [
    {
      "name": "provider-name",
      "endpoint": "https://provider.example/v1",
      "api_key": "REPLACE_WITH_SECRET"
    }
  ],
  "favorite_models": [
    "provider-name/model-name"
  ],
  "roles": {
    "default": "provider-name/model-name",
    "vision": "provider-name/model-name",
    "summarization": "provider-name/model-name",
    "advisor": "provider-name/model-name"
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
  "helperSetup": {
    "dismissed": false,
    "declined": []
  },
  "debugPrompt": false
}
```

Optional role keys (`vision`, `summarization`, `advisor`) may be omitted rather than set to invalid empty model IDs. Do not put `last_model` in the editable template; it is vestigial first-run output that active model selection does not use.

### Providers

- Each provider requires a unique non-empty `name`, a non-empty endpoint, and must be OpenAI-compatible.
- API-key providers require `api_key`.
- ChatGPT OAuth uses `auth_type: "oauth"` plus OAuth credentials and should be configured through `/auth openai`.
- No provider fallback. No automatic provider switching on failure.

### Model IDs and Roles

- Model IDs use exact `provider/model_name` format with no `|` suffix.
- Every referenced provider must exist in the configured providers list.
- `default` role is required.
- `vision`, `summarization`, and `advisor` are optional.
- Requesting an unset role fails explicitly with no fallback.

### Favorites and Model Selection

- `/model` opens live provider/model selection.
- `/model <provider/model>` assigns the current interactive agent's model.
- `/model +` adds the current model to favorites.
- `/model -` removes the current model from favorites.
- `Ctrl+\` cycles favorites.
- Zero or one favorites produce no cycle change.

### Compaction and Reasoning Payload

Displayed defaults:

| Key | Default |
|-----|---------|
| `maxContextTokens` | 100000 |
| `minContextTokens` | 50000 |
| `summaryMaxTokens` | 2000 |
| `maxSummaryFiles` | 10 |
| `tokenCoefficient` | 3.5 |
| `maxBackoffOffsetTokens` | 25000 |

- Compaction thresholds govern context summarization.
- `stripReasoning.enable` removes older reasoning payload text.
- `stripReasoning.preserveLast` keeps the newest N reasoning-bearing messages.
- This does not configure request-time reasoning effort or console reasoning display.

### Helper and Diagnostic Settings

- `helperSetup.dismissed` suppresses all optional helper suggestions.
- `helperSetup.declined` suppresses named helper suggestions by name.
- `debugPrompt: true` writes `prompt.json` in the session folder for diagnosis and defaults to `false`.

### Editing and Reload

1. Read the complete current file.
2. Modify the JSON with a text editor or `replace_block`.
3. Validate JSON syntax before saving.
4. Preserve credentials and unrelated fields.
5. Restart after manual edits for changes to take effect.
6. Runtime `/model`, `/mode`, and favorite commands update live state without requiring a restart.

## Agent Definitions and Active Modes

An active BlazeAI "mode" is now an interactive agent defined in Markdown. `modes.json` is not the active configuration source.

### Location and Discovery

- Definitions are direct, non-recursive `.md` files under `{APP_HOME}/agents/`.
- Files are sorted by path at load time.
- Frontmatter `name` values must be globally unique across all definitions.
- Invalid files, duplicate names, bad tool references, bad executor references, or missing interactive definitions stop startup with an explicit error.
- Definitions load only at process startup. There is no hot reload.

### Definition Format

```markdown
---
name: agent-name
description: What this agent does
type: interactive
model: provider-name/model-name
timeout: 15m
directive: A short per-turn directive
tools:
  - read_file
  - shell
agents:
  - worker
---
Markdown instructions for the agent.
```

Accepted keys only: `name`, `description`, `type`, `model`, `timeout`, `directive`, `tools`, `agents`. `kind:` is legacy and must not be written.

- `name` and `description` are required.
- `type` is exactly `interactive` or `executor`.
- `model` is required for interactive agents and optional for executors.
- An executor without `model` inherits the parent interactive model.
- `timeout`, when present, is a positive Go duration.
- `tools` is required, non-empty, unique, and every name must exist in the runtime registry.
- `directive` and `agents` are interactive-only.
- `agents` contains unique names of existing executor definitions.
- Executors cannot reference other agents or use `run_agent`.
- When an interactive agent allows at least one executor, runtime exposes `run_agent` automatically.
- The Markdown body is the agent's instruction text.
- An interactive directive is appended ephemerally to the latest user message for the provider call and is not persisted into session history.

### Interactive Agent Example

```markdown
---
name: planning
description: Strategic planning and architecture review
type: interactive
model: provider-name/model-name
timeout: 15m
directive: Focus on long-term structure. Do not make code changes directly.
tools:
  - read_file
  - shell
  - write_file
agents:
  - worker
---
You are a planning agent. Analyze the current project state, identify goals, and propose structured plans. Delegate implementation tasks to the worker executor.
```

### Executor Agent Example

```markdown
---
name: worker
description: Executes implementation tasks delegated by interactive agents
type: executor
tools:
  - read_file
  - write_file
  - shell
  - replace_block
---
You are a worker executor. Follow the instructions given by the calling interactive agent precisely. Complete the requested task using the available tools.
```

Tool names in examples are placeholders. Replace them with names from the actual runtime registry when available. Never claim an unverified tool is universally installed.

### Selecting a Mode and Assigning Its Model

- `/mode <interactive-name>` selects by exact name.
- Tab cycles loaded interactive agents with wrap-around.
- Executor definitions are not selectable as modes.
- `/model <provider/model>` changes the selected interactive agent's active model and writes that state to `agents.json`.
- The definition's `model` seeds state only when the interactive agent has no existing state entry.
- `last_agent` controls next-start selection and is updated by `/mode` and Tab.
- `agents.json` is normally runtime-managed and should not be used to define tools, directives, instructions, or executor permissions.

### Creating, Editing, Renaming, and Deleting Safely

1. Create or edit the Markdown definitions, validate all names, tools, and references, then restart. New interactive state entries are added automatically.
2. Renaming only a filename is safe because frontmatter `name` is the identity.
3. Renaming an interactive `name` requires updating or removing the old `agents.json` state entry and updating `last_agent` when it points to the old name; otherwise startup fails.
4. Deleting an interactive requires removing its state entry, selecting an existing `last_agent`, and leaving at least one interactive definition.
5. Renaming or deleting an executor requires updating every interactive `agents:` reference first; otherwise startup fails.
6. Do not delete all interactive definitions while `agents.json` exists; the default is not recreated in that condition.
7. Preserve unrelated runtime state and never silently repair an ambiguous rename or choose a replacement agent or model without user confirmation.

### Fresh-Install Default Agent

When zero interactive definitions exist and `agents.json` is absent, startup provisions `agents/default.md` from the validated default role and available runtime tools. It is user-editable and never overwritten. Existing `agents.json` suppresses re-provisioning even if empty.

## Legacy modes.json Migration Only

**Do not create or edit `modes.json` to configure current modes.** This section documents legacy migration for existing installations only.

- Migration runs only when `agents.json` is absent and `modes.json` exists.
- Each legacy mode becomes `<mode.name>.md` with `type: interactive`.
- `model` and `directive` transfer.
- Allowed tools are derived by subtracting `denied_tools` and control tools from the runtime tool set.
- Legacy `agents` become executor references.
- Existing destination files are never overwritten.
- Unsafe names and invalid content fail explicitly.
- Legacy `kind: one-shot` definitions are migrated to `type: executor`, but all new definitions must use `type`.
- Legacy `modes` embedded directly in `config.json` have no active production migration caller and must not be claimed as automatically migrated.

## Telegram Bridge Guide

Use this section when the user needs Telegram bridge instance creation, editing, inspection, repair, startup, or verification.

### Required Values

- `instance` name for `{APP_HOME}/telegram/<instance>/`
- Telegram bot token
- Allowed chat ID
- Absolute existing `workdir`
- Selected model in `provider/model_name` format

If any required value is missing, stop and ask. Do not invent defaults.

### Required Reads

- Read `{APP_HOME}/telegram/README.md` before touching files under `{APP_HOME}/telegram/`.
- Read `{APP_HOME}/config/README.md` and `{APP_HOME}/config/config.json` before validating models.

### Bot and Chat Setup

- Create the bot with BotFather: `/newbot`, choose display name, choose unique username ending in `bot`, capture returned token.
- Treat the token as a secret.
- To discover the allowed chat ID, ask the user to send a message to the bot, then call `https://api.telegram.org/bot<token>/getUpdates` and read `message.chat.id`.
- Keep the numeric sign exactly as returned. Do not quote `allowed_chat_id` in JSON.

### Instance Files

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

`state.json` is runtime-managed and should not be freely rewritten:

```json
{
  "selected_model": "provider/model_name"
}
```

Rules:
- `selected_model` is required.
- It must reference a provider that already exists in `{APP_HOME}/config/config.json`.

### Workflow

1. Identify the instance name.
2. Read the required README and config files.
3. Gather or confirm bot token, chat ID, workdir, and selected model.
4. Validate all required values before writing any file.
5. If editing an existing instance, read current `bridge.json` and `state.json` first.
6. Write valid minimal JSON only.
7. Re-read the written files or otherwise validate the saved content.

### Startup and Services

- Start one instance with `blazeai --telegram <instance>`.
- Startup must fail if any required Telegram file or field is missing or invalid.

#### Linux systemd

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

### Restart

Service `blazeai-telegram@<instance>` has `Restart=always`. Best restart method **without sudo**:

```sh
kill $(pgrep -f "blazeai.*--telegram <instance>")
```

systemd respawns instantly. Verify with `pgrep -f "blazeai.*--telegram <instance>"`.

If `sudo` is used, chain all commands in one call to prompt once:

```sh
sudo sh -c 'systemctl restart blazeai-telegram@<instance> && systemctl status blazeai-telegram@<instance> --no-pager'
```

### Verification

- Send a normal text message from the allowed chat.
- Confirm the bot responds.
- Confirm `/help` works.
- Confirm `/model` shows the current instance model.
- If another chat messages the bot, explain that the instance is expected to ignore it.

### Stop Conditions

- Stop and ask if one of `instance`, bot token, allowed chat ID, workdir, or selected model is missing.
- Stop and ask if the requested model's provider does not exist.
- Stop and ask if `workdir` does not exist.
- Stop and ask before deleting an instance or replacing a token for an existing instance.

## Host Helper Setup Guide

Use this section when host tools are missing or the user asks about helper installation.

### Scope

- Configure helper utilities only when the user asks or when a missing helper would materially improve current work.
- Never install anything without explicit user approval.
- Never run `sudo` without explicit user approval per command.
- After installation, verify that the helper is usable before the next prompt build.

### Core Cross-Platform Helpers

| Helper | Purpose | Typical Install Command |
|--------|---------|-------------------------|
| rg | fast recursive content search | `apt install ripgrep` / `brew install ripgrep` / `winget install BurntSushi.ripgrep` |
| fd | fast file and directory discovery | `apt install fd-find` / `brew install fd` / `winget install sharkdp.fd` |
| jq | JSON inspection and transformation | `apt install jq` / `brew install jq` / `winget install jqlang.jq` |
| git | VCS operations | `apt install git` / `brew install git` / `winget install Git.Git` |
| curl | HTTP/API checks, downloads | `apt install curl` / `brew install curl` / built-in on modern Windows |
| pandoc | document conversion | `apt install pandoc` / `brew install pandoc` / `winget install JohnMacFarlane.Pandoc` |
| sqlite3 | lightweight SQL queries | `apt install sqlite3` / `brew install sqlite3` / `winget install SQLite.SQLite` |

### Detection

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

### Installation

#### Linux

1. Detect package manager first: `which apt || which dnf || which pacman || which zypper || which apk`
2. Ask which helper(s) to install and confirm.
3. If `sudo` is required, ask separately. Never batch `sudo` commands without approval.
4. Example: `sudo apt update && sudo apt install -y ripgrep fd-find jq pandoc sqlite3`

#### macOS

1. Check for Homebrew: `command -v brew`
2. If brew exists and user approves: `brew install ripgrep fd jq pandoc sqlite3`
3. If brew is missing, suggest installing it first and ask the user.

#### Windows

1. Check for package manager: `winget --version` or `scoop` or `choco`
2. If winget exists and user approves, install the requested helpers.
3. If no package manager exists, explain that the user needs winget, scoop, or choco first.

### Verification After Install

- Re-run detection.
- If the helper still does not resolve, report failure and continue with available alternatives.
- Do not loop or retry without user instruction.

### Dismissing the Helper Reminder

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

### Python Environment

- Python is not a host helper. It is a restricted runtime.
- If Python is truly necessary and `{APP_HOME}/scripts/venv/` does not exist yet, ask before creating it.
- Create it lazily with `python3 -m venv {APP_HOME}/scripts/venv`.
- All later Python usage must go through that venv.

## Final Validation Checklist

Before completing any configuration edit:

- [ ] Valid complete JSON and private `0600` file permissions.
- [ ] Required `default` role set and all model references point to existing providers.
- [ ] Unique agent names, valid types, valid tool references, valid executor references, and at least one interactive agent defined.
- [ ] `agents.json` consistent with agent definitions only when a rename or delete requires state repair.
- [ ] Telegram `workdir` is absolute and exists; chat ID is authorized.
- [ ] Restart performed after manual config or definition edits.
- [ ] No fallback, guessed secret, guessed model, guessed tool, guessed agent, or overwrite of unrelated settings.
