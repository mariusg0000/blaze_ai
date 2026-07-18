# Skills

This folder stores custom global BlazeAI skills available on this machine.

- Each skill lives in a subfolder: `<name>/skill.md`.
- Required format: non-empty `[DESCRIPTION]` and `[BODY]` sections.
- Keep skills concise and focused. Project-scoped skills live under `{APP_HOME}/projects/<project>/skills/`.
- The names `skill-manager`, `config-manager`, and `audit-manager` are reserved for immutable builtin skills. Do not create custom skills with these names.
- If a conflicting custom skill exists, BlazeAI ignores it and uses the embedded builtin.
- Bare names load builtin or global skills; project-scoped skills use the `project/` prefix.
