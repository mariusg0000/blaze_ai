# Session Decision Summary: Installer, Customize Rename, Directive Fix

Date: 2026-06-27 07:58
Base commit: (working tree)

## Context
Three tasks from today's work: 1) Linux user-level installer with self-contained binary, 2) rename builtin `customize_me` to `customize-me` with docs/ subtree seeding, 3) remove `[MODE DIRECTIVE]` injection from tool responses and keep it only on user messages.

## Changes Made

### Linux Installer
- Created `blazeai_installer/install.sh` — self-contained single-file installer with binary embedded as base64. User runs one command.
- Created `deploy_nas.sh` — builds binary, packages installer, deploys to remote server, runs remote install. Handles running-binary replacement with `rm -f` before `cp`.
- Removed old `installer/`, `release/`, `blazeai/` layout attempts.

### Builtin Skill: customize_me → customize-me
- Renamed `skills/customize_me.md` to `skills/customize-me.md`.
- Added `skills/customize-me/docs/telegram.md` and `skills/customize-me/docs/helpers.md` with distilled content from the former `telegram_bridge` and `setup_helpers` skills.
- Updated `customize-me` [DESCRIPTION] to a short generic routing label covering providers, API keys, roles, work modes, Telegram bridge, and host helpers.
- The [BEHAVIOR] section now links docs via `{SKILL_DIR}/docs/` instead of duplicating guidance.
- Updated `skill-manager.md` builtin list, `specs.md`, `specs/02-core-runtime.md`, `specs/06_telegram.md` to reference `customize-me`.

### Builtin Seeding: docs/ Subtree
- Extended `SeedBuiltins` in `internal/skills/skills.go` with `copyBuiltinSubtree()` that copies an optional same-named subtree (e.g., `customize-me/docs/`) alongside the main `skill.md` on first seed.
- Existing customised skills are not touched and their docs/ subtree is not created.
- Updated `embed.go` embed directive from `skills/*` to `skills` to allow subdirectory embedding.
- Added `internal/skills/skills_test.go` tests for docs seeding and existing-skill skip.

### [MODE DIRECTIVE] Fix
- Changed `injectDirective()` in `internal/runtime/runtime.go` to find the latest `user` message instead of always appending to the last message.
- Tool results (`tool` role) are left untouched so the LLM receives exact tool output.
- Added `TestInjectDirectiveSkipsToolTail` test to verify tool messages remain unchanged.

## Decisions And Rationale
- **Self-contained installer**: A single `install.sh` with embedded binary (base64) avoids any download, tar, or multi-file steps for the user. One command, done.
- **docs/ subtree for builtins**: Storing detailed guidance in separate files under `docs/` keeps the main skill.md focused on routing and high-level rules. The agent reads the relevant doc only when the user's task matches that doc's scope. This reduces prompt noise from unused detail.
- **customize-me as meta-configuration**: Renaming to `customize-me` (hyphen instead of underscore) matches the naming pattern of other builtins. Making its description a short routing list instead of a full feature description helps the agent decide when to load it.
- **Directive on user message only**: The [MODE DIRECTIVE] is a user-facing behavior modifier. Injecting it into tool messages would break LLM contract — tool results must be sent verbatim. The user saying "proceed" after a tool call should see the directive applied to their proceed message, not retroactively to a previous assistant tool call.

## Implementation Approach
- `copyBuiltinSubtree` is recursive and uses `fs.ReadDir` + `filepath.ToSlash` for cross-platform path handling within the embed.FS.
- The base64 embedding in the installer uses `awk` to locate the `# __BLAZEAI_BINARY__` marker and decode everything after it.
- deploy_nas.sh regenerates the installer from scratch on each run so the binary is always fresh.

## Alternatives Considered
- **Installer folder with separate tar.gz**: Rejected because the user insisted on a single command `./install.sh` with no extraction step.
- **Keeping `customize_me` as a flat .md**: Rejected. The docs/ split improves maintainability and reduces token waste.
- **Appending directive to last message unconditionally**: Rejected because it corrupted tool results.

## Files Included
- `.gitignore`: unrelated/pre-existing change (removed `/blazeai/` pattern)
- `skills/telegram_bridge.md`: unrelated/pre-existing change (Telegram bridge startup instructions update)
- `blazeai_installer/install.sh`: new — self-contained Linux installer
- `deploy_nas.sh`: new — remote build-and-deploy script
- `embed.go`: updated embed directive for subdirectory support
- `internal/skills/skills.go`: added `copyBuiltinSubtree`, updated `SeedBuiltins`
- `internal/skills/skills_test.go`: added docs seeding tests
- `internal/runtime/runtime.go`: directive injection targets user messages only
- `internal/runtime/runtime_test.go`: added tool-tail directive test
- `skills/customize_me.md → customize-me.md`: renamed and updated
- `skills/customize-me/docs/telegram.md`: new
- `skills/customize-me/docs/helpers.md`: new
- `skills/skill-manager.md`: updated builtin list
- `specs.md`: updated builtin skill reference
- `specs/02-core-runtime.md`: updated customize_me → customize-me
- `specs/06_telegram.md`: updated cross-reference

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
