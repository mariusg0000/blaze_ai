# BlazeAI App Home

This folder stores BlazeAI runtime data outside the project workspace.

- `config/`: machine-local configuration such as providers, API/OAuth credentials, and role assignments. Provider integration is configured from the primary console; ChatGPT OAuth follows the Codex PKCE and token-exchange flow.
- `projects/`: per-project runtime state, including session history grouped by working folder.
- `skills/`: custom skill modules. Each skill lives in a subfolder with `skill.md`.
- `scripts/`: helper scripts and the lazy Python virtual environment when needed.
- `backups/`: optional safety copies created before risky file changes.

Keep files here human-readable when practical. The app may read them directly into prompts.
