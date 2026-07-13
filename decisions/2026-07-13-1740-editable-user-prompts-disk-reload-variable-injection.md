# Session Decision Summary: Editable user prompts with per-build disk reload

Date: 2026-07-13 17:40

## Context

The user wanted all prompt templates exposed to app-home storage so they could be edited without recompilation. Every prompt build must reread templates from disk, and all dynamic variables must work in every template, not only in the universal sysprompt.

## Changes Made

- `main.go` — `prepareBuiltinAssets()` now seeds every embedded prompt template into `<app_home>/prompts/` when missing, then returns `os.DirFS(promptsDir)` as the live disk filesystem. Added `seedMissingPromptFiles()` that skips existing files. Added `filepath` import.
- `main_test.go` — new test: seeding creates all prompt files and preserves user edits.
- `internal/prompt/prompt.go` — `BuildRuntimePart()` renders OS and transport templates through the full shared variable map before injecting them into the universal template. Removed early `injectVariables` calls on OS and transport prompts; they now receive `templateValues` including all runtime sections (helpers, skills, agents, etc.) plus `TRANSPORT_CONTEXT`.
- `internal/prompt/prompts.go` — `PromptsFS` parameter documented as live editable templates directory.

## Decisions And Rationale

- Seed missing files only (never overwrite) so users can edit any template without losing changes on restart.
- Use `os.DirFS(promptsDir)` instead of the embedded `fs.FS` because the embedded filesystem is immutable.
- Render OS and transport templates before the universal template so `{OS_PROMPT}` and `{TRANSPORT_PROMPT}` are resolved when the universal template references them.
- All templates share the same variable map because users writing custom prompts need access to helpers, skills, and agents data.
- Skip `readme.md` and non-`.md` files during seeding because `readme.md` is documentation, not a prompt template.
- Backup saved under `/home/marius/blazeai/backups/editable-prompts/`.
