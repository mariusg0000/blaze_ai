# Project-Scoped Skills and Memories

Allow skills and memories to be scoped to a specific project (working folder), not only global under `app_home`.

## Discovery

At every prompt build, in addition to global `app_home/skills/` and `app_home/memories/`, scan the current project folder for:

```
{workdir}/.blazeai/skills/    → project-scoped skills
{workdir}/.blazeai/memories/  → project-scoped memories
```

## Behavior

- Project-scoped skills appear alongside global skills in the "Available skills" section, prefixed with `[project]` to distinguish them.
- Project-scoped memories appear alongside global memories, similarly marked.
- When a mode or working folder changes, project-scoped items are automatically unloaded. No stale project context leaks into a different project.
- Collision rule: if a skill or memory name exists both globally and in the project, the project version wins (same as custom-over-builtin today).

## Benefits

- Keeps project-specific tooling (build commands, deploy scripts, project conventions) close to the code, not in the global `app_home`.
- Team members can share `.blazeai/` in version control for consistent project workflows.
- Reduces global skill/memory namespace pollution as the number of projects grows.
- Complements `project-map.md` — the map describes structure, scoped skills provide project-local actions.

## Risks

- `.blazeai/` must be gitignore-aware to avoid accidentally committing secrets or API keys.
- Skill collision resolution between global and project needs clear precedence rules.
- Prompt size may increase if many projects have large skill sets — compaction must be aware of project-scoped content.
