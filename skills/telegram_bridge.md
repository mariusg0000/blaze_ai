[DESCRIPTION]
Load when the user wants to create, edit, inspect, list, or repair BlazeAI Telegram bridge instances. Use for `app_home/telegram/<instance>/bridge.json`, `state.json`, allowed chat setup, workdir binding, and per-instance model validation.

[BEHAVIOR]
# Telegram Bridge

## Purpose
- Manage Telegram bridge instances without manual JSON authoring.
- Guide the full Telegram setup path from bot creation to runtime verification.
- Create or update `bridge.json` and `state.json` under `{APP_HOME}/telegram/<instance>/`.
- Keep Telegram model selection local to the instance. Never write Telegram model changes into `{APP_HOME}/config/config.json` or `{APP_HOME}/config/modes.json`.

## What The User Must Provide
- `instance` name for `{APP_HOME}/telegram/<instance>/`
- Telegram bot token
- allowed chat id
- absolute existing `workdir`
- selected model in `provider/model_name` format

If any required value is missing, stop and ask. Do not invent defaults.

## Required Folder Context
- Application home contains `telegram`, `config`, `projects`, `skills`, `scripts`, `backups`.
- Before inspecting or modifying files under `{APP_HOME}/telegram/`, read `{APP_HOME}/telegram/README.md` first.
- Before validating models against global providers, read `{APP_HOME}/config/README.md` and then `{APP_HOME}/config/config.json`.

## Telegram Bot Creation
- Preferred workflow: create the bot with BotFather in Telegram.
- Tell the user to open a chat with `@BotFather`.
- Use `/newbot`.
- Provide the display name BotFather asks for.
- Provide a unique bot username ending in `bot`.
- Capture the token BotFather returns.
- Treat the token as a secret. Do not paste it into unrelated files or messages.

### Optional BotFather Tasks
- `/setdescription` to document what the bot is for.
- `/setuserpic` if the user wants an avatar.
- `/setcommands` if the user wants Telegram's command hint list to match the runtime:
  - `help - show supported commands`
  - `model - show or change the instance model`
  - `clear - clear the current conversation`
  - `new - clear the current conversation`
  - `exit - close the current session cleanly`

Do not claim these optional tasks are required for BlazeAI runtime startup.

## Allowed Chat ID Discovery
- The bridge accepts exactly one configured chat id per instance.
- Never guess the chat id from the bot username or token.

### Preferred Discovery Flow
1. Ask the user to send any message to the bot from the target chat.
2. Call Telegram Bot API `getUpdates` with the bot token.
3. Read the numeric `message.chat.id` from the returned update.
4. Use that exact integer as `allowed_chat_id`.

### Example API Check
```text
https://api.telegram.org/bot<token>/getUpdates
```

### Interpretation Rules
- Private chats usually have a positive chat id.
- Group or supergroup chats may have negative chat ids.
- Keep the numeric sign exactly as returned.
- `allowed_chat_id` must stay numeric in JSON, not quoted.

### If `getUpdates` Is Empty
- Confirm the user has sent a fresh message to the bot.
- If privacy mode or group routing may interfere, ask the user to message the bot directly first.
- If still empty, stop and ask instead of inventing a value.

## Model Selection
- Read `{APP_HOME}/config/config.json` before choosing `selected_model`.
- `selected_model` must be `provider/model_name`.
- The provider must already exist in global config.
- Prefer a model already present in `favorite_models` when the user has not chosen a specific one, but still ask the user to confirm the exact model. Do not auto-pick silently.

## Instance Layout
- Instance directory: `{APP_HOME}/telegram/<instance>/`
- Static config: `{APP_HOME}/telegram/<instance>/bridge.json`
- Mutable state: `{APP_HOME}/telegram/<instance>/state.json`

### bridge.json
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
- Never infer a chat id from previous files or messages.
- Never store comments or extra metadata in the JSON.

### state.json
```json
{
  "selected_model": "provider/model_name"
}
```

Rules:
- `selected_model` is required.
- It must be exactly `provider/model_name`.
- The provider part must exist in `{APP_HOME}/config/config.json`.
- Do not write extra fields unless the runtime adds them later.

## Provisioning Workflow
1. Identify the instance name.
2. Read `{APP_HOME}/telegram/README.md`.
3. Read `{APP_HOME}/config/README.md` and `{APP_HOME}/config/config.json`.
4. Gather or confirm the bot token, allowed chat id, workdir, and selected model.
5. If the user does not yet have a bot token, guide them through the BotFather flow first.
6. If the user does not yet know the chat id, guide them through `getUpdates` discovery first.
7. If editing an existing instance, read the current `bridge.json` and `state.json` before changing them.
8. Validate all required values before writing any file.
9. Create `{APP_HOME}/telegram/<instance>/` only after all required values are known.
10. Write both JSON files with valid syntax and the exact field names above.
11. Re-read the written files or otherwise validate the saved JSON content.
12. Tell the user how to start the bridge and how to verify it.

## Startup
- Start one instance with:
```text
blazeai --telegram <instance>
```
- The runtime loads:
  - `{APP_HOME}/config/config.json`
  - `{APP_HOME}/telegram/<instance>/bridge.json`
  - `{APP_HOME}/telegram/<instance>/state.json`
- Startup must fail if any required Telegram file or field is missing or invalid.
- Do not describe any fallback startup path.

## Verification
- After startup, send a normal text message from the allowed chat.
- Confirm the bot responds in that chat.
- Confirm `/help` works.
- Confirm `/model` shows the current instance model.
- If the user changes the model with `/model provider/model_name`, confirm the new value is persisted only in `state.json`.
- If another chat messages the bot, explain that the instance is expected to ignore it.

## Listing Instances
- To list configured bridge instances, inspect subdirectories under `{APP_HOME}/telegram/`.
- Report only directories that contain at least one of `bridge.json` or `state.json`.
- If a directory is incomplete, call that out explicitly instead of treating it as valid.

## Editing Rules
- Preserve unchanged fields when editing an existing instance.
- Touch only the requested instance unless the user explicitly asks for broader cleanup.
- If the instance folder exists but one required file is missing, treat it as an invalid instance and repair it only with explicit user intent.
- If the user asks to rename an instance, create the new folder contents explicitly; do not silently repurpose another instance.

## Maintenance
- Token rotation: replace only `bot_token` in `bridge.json` after the user provides a new token.
- Model changes requested outside chat: update only `state.json`.
- Workdir changes: validate the new directory exists before writing `bridge.json`.
- Incomplete instance repair: read both files, identify what is missing, then repair only with explicit user intent.
- Deletion is destructive. Stop and ask before removing an instance folder.

## Validation
- `selected_model` provider must exist in global config.
- `workdir` must exist and be a directory.
- `allowed_chat_id` must remain numeric in JSON, not a quoted string.
- JSON must stay minimal and valid.
- No fallback values. Missing required input means stop and ask.
- When using `getUpdates`, use the most recent relevant message from the intended chat.
- Keep secrets scoped to the intended Telegram instance files and necessary runtime commands.

## Stop Conditions
- Stop and ask if the user has not provided one of: instance name, bot token, allowed chat id, workdir, selected model.
- Stop and ask if the requested model's provider does not exist in global config.
- Stop and ask if `workdir` does not exist.
- Stop and ask before deleting an instance or replacing a bot token for an existing instance.
- Stop and ask if the bot token appears invalid or `getUpdates` cannot confirm the target chat id.
