# Config

This folder stores BlazeAI machine-local configuration files.

- `config.json`: main runtime configuration. Edit carefully because invalid values should stop the app, not trigger fallbacks.
- `modes.json`: work modes and the last active mode. It is separate from `config.json` so frequent mode edits do not touch provider secrets.

`config.json` overview:

- `providers`: provider definitions. Each item needs `name`, `endpoint`, and `api_key`.
- `favorite_models`: quick-pick list of model IDs in `provider/model_name` form.
- `roles`: model assignments. `default` is required; `vision` and `summarization` are optional.
- `compaction`: context compaction thresholds such as trigger, target, summary budget, file cap, local token estimate coefficient, and hard-cap backoff.
- `stripReasoning`: whether reasoning parts are stripped before provider calls and how many newest reasoning blocks stay visible.
- `last_model`: legacy fallback for rare cases without an active mode. Normal model persistence should happen through `modes.json`.
- `helperSetup`: whether optional helper-install suggestions were dismissed and which helpers were declined.

`modes.json` overview:

- `modes`: named work modes. Each mode has `name`, `model`, and optional `directive`.
- `last_mode`: the mode restored as active on the next app start.

Keep values explicit, valid, and human-readable. Secrets stay in these local files, not in prompts.
