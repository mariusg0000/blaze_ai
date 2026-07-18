# Scripts

This folder stores task scripts created by BlazeAI outside user projects.

- Store scripts for each task under `tasks/<task-slug>/`.
- Prefer the simplest robust shell or Python implementation that reduces tool calls and tokens.
- `write_file` creates missing parent directories, so a separate `mkdir` call is unnecessary.
- `venv/` is reserved for the lazily-created BlazeAI Python virtual environment.
- Run Python task scripts and install their libraries only through that venv unless the user explicitly requests work on the system Python environment.
