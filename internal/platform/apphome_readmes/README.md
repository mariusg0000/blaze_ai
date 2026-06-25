# BlazeAI App Home

This folder stores BlazeAI runtime data outside the project workspace.

- `config/`: machine-local configuration such as providers, API keys, and role assignments.
- `projects/`: per-project runtime state, including session history grouped by working folder.
- `memory/`: one shared `memory.md` file for durable notes the agent may read.
- `memories/`: reusable memory-bank files that can be loaded into prompts when relevant.
- `skills/`: custom skills available on this machine.
- `scripts/`: helper scripts and the lazy Python virtual environment when needed.
- `backups/`: optional safety copies created before risky file changes.

Keep files here human-readable when practical. The app may read them directly into prompts.
