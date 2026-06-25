# Session Decision Summary: apphome readmes ideas

Date: 2026-06-25 09:57
Base commit: 574152f

## Context
The user requested concise README files for key app-home folders so both humans and the LLM can understand their purpose with low token cost. These README files had to be copied into app home automatically at startup only when missing. The user also requested a new `ideas/` folder, a written idea for a future `project-map` skill, migration of the existing root `IDEAS.md` content into individual idea files, and removal of the old aggregate file.

## Changes Made
- Added embedded README templates for the app-home root plus `backups`, `scripts`, and `skills`.
- Updated platform bootstrap to create those README files in app home only when they do not already exist.
- Added bootstrap tests for README creation and preservation of user-edited README content.
- Updated prompt wording and prompt tests to reflect the new README behavior.
- Created `ideas/` with individual Markdown files for `project-map`, `memory-bank`, `context-router`, and `task-focused-summarization`.
- Removed the old root `IDEAS.md` after splitting its content into separate files.

## Decisions And Rationale
- Embedded README templates were chosen so startup can materialize them from the compiled binary without depending on repository files at runtime.
- Bootstrap writes only missing files because the user explicitly wanted copying when absent, not silent overwrites of human-maintained documentation.
- The README texts stay short and practical so they remain useful both for humans and for prompt injection if loaded later.
- The old monolithic `IDEAS.md` was split into individual files to make ideas easier to browse, extend, and discuss independently.
- `prompts/sysprompt.md` contains broader pre-existing edits relative to `HEAD`; the commit includes the current file state together with the requested README wording update to leave the repository clean.

## Implementation Approach
- Added `internal/platform/apphome_readmes.go` with an embedded filesystem and a small helper that maps embedded README assets to their app-home destinations.
- Hooked that helper into `platform.Bootstrap()` after directory creation.
- Added focused tests in `internal/platform/platform_test.go` for generated README presence and no-overwrite behavior.
- Updated `internal/prompt/prompt_test.go` and `prompts/sysprompt.md` to keep runtime prompt wording consistent with the new app-home documentation behavior.
- Added the requested idea files under `ideas/` and removed the legacy root file.

## Files Included
- `internal/platform/apphome_readmes.go`: embeds and copies shipped app-home README assets.
- `internal/platform/apphome_readmes/README.md`: root app-home README template.
- `internal/platform/apphome_readmes/backups/README.md`: backups folder README template.
- `internal/platform/apphome_readmes/scripts/README.md`: scripts folder README template.
- `internal/platform/apphome_readmes/skills/README.md`: skills folder README template.
- `internal/platform/platform.go`: bootstrap now copies missing README files.
- `internal/platform/platform_test.go`: coverage for README creation and preservation.
- `prompts/sysprompt.md`: documents the new README guidance behavior; file also includes pre-existing broader worktree edits.
- `internal/prompt/prompt_test.go`: aligns prompt fixtures with the README guidance wording.
- `ideas/project-map.md`: new requested idea for project structure mapping and prompt injection.
- `ideas/memory-bank.md`: extracted from the legacy ideas file.
- `ideas/context-router.md`: extracted from the legacy ideas file.
- `ideas/task-focused-summarization.md`: extracted from the legacy ideas file.
- `IDEAS.md`: removed after extraction into individual idea files.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
