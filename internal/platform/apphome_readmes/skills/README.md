# Skills

This folder stores custom BlazeAI skills available only on this machine.

- Each skill lives in a subfolder: `<name>/skill.md`.
- Required format: `[DESCRIPTION]` plus at least one of `[BEHAVIOR]` or `[DATA]`.
- BEHAVIOR = procedural guidance; DATA = persistent facts in key=value format.
- Keep skills concise and focused. Project-scoped skills live under `{APP_HOME}/projects/<project>/skills/`.
- Skills override builtin skills by name. Project-scoped skills use `project/` prefix when loading.
