# telegram

Stores Telegram bridge instances under `app_home/telegram/<instance>/`.
Each instance folder contains strict runtime-owned files such as `bridge.json` and `state.json`.
Do not invent defaults here at runtime: missing or invalid instance files must stop Telegram startup with a clear error.
