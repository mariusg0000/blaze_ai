# Session Decision Summary: apphome readmes mode priority

Date: 2026-06-25 10:20
Base commit: 324caea

## Context
After adding startup-copied app-home README files, the user requested coverage for the remaining top-level folders `config`, `projects`, and `memories`. The `memories` folder needed a strict guard so its `README.md` is never treated as a loadable memory file. The user also questioned the relationship between `last_mode` and `last_model` and clarified that the active mode should be more important because it already carries the active model.

## Changes Made
- Added embedded startup README templates for `config`, `projects`, and `memories`.
- Expanded bootstrap tests to expect README creation in those folders.
- Added an explicit `README.md` skip in memory discovery and test coverage for that guard.
- Updated prompt wording and prompt fixtures to reflect README coverage across app-home top-level folders.
- Changed runtime model selection so `last_mode` is the primary persisted source of truth for the active model when a mode exists.
- Updated `SetModel()` to persist model changes into `modes.json` when a mode is active, and to use `config.json:last_model` only as a legacy fallback when no active mode exists.
- Stopped `SetMode()` from mutating `config.json:last_model`.
- Added runtime tests to verify persisted mode updates and that active mode wins over legacy `last_model` on startup.

## Decisions And Rationale
- `memories/README.md` must be ignored explicitly instead of relying on parse failure, because folder documentation should never become accidental runtime context.
- `config/README.md` documents both `config.json` and `modes.json` because the current codebase already split modes into a separate file and users need that reflected accurately.
- `last_mode` was promoted to the canonical active-state selector because it already determines both behavior and model through the selected mode. Keeping `last_model` as a normal persistence path would duplicate state and allow drift.
- `last_model` remains documented only as a legacy fallback for rare no-mode situations to avoid a larger compatibility migration in the same patch.

## Implementation Approach
- Extended the embedded README asset list in `internal/platform/apphome_readmes.go` and added concise new Markdown templates under `internal/platform/apphome_readmes/`.
- Updated `internal/memories/memories.go` to skip `README.md` case-insensitively before parsing candidate memory files.
- Adjusted `internal/runtime/runtime.go` so agent startup seeds fallback model selection from `roles.default`, active modes override that selection, mode-bound model changes save to `modes.json`, and mode switching no longer writes `config.json:last_model`.
- Added focused tests in `internal/runtime/runtime_test.go`, `internal/memories/memories_test.go`, and `internal/platform/platform_test.go`, then validated the affected packages.

## Files Included
- `internal/platform/apphome_readmes.go`: expanded startup README copy targets.
- `internal/platform/apphome_readmes/config/README.md`: explains config files and key parameters.
- `internal/platform/apphome_readmes/memories/README.md`: documents memory file rules and the README non-loadable guard.
- `internal/platform/apphome_readmes/projects/README.md`: explains top-level project-scoped runtime storage.
- `internal/platform/platform_test.go`: expects the new README files at bootstrap.
- `internal/memories/memories.go`: explicit `README.md` ignore guard.
- `internal/memories/memories_test.go`: verifies README ignore behavior.
- `internal/runtime/runtime.go`: mode-first active model persistence logic.
- `internal/runtime/runtime_test.go`: covers persisted mode updates and mode priority over legacy last_model.
- `prompts/sysprompt.md`: updated README guidance wording.
- `internal/prompt/prompt_test.go`: synchronized prompt fixture wording.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
