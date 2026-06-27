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
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
```

- Enable and start with `systemctl enable --now blazeai-telegram@<instance>`.
- Check logs with `journalctl -u blazeai-telegram@<instance> -f`.

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
