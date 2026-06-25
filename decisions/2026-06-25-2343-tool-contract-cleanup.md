# Session Decision Summary: Tool Contract Cleanup

Date: 2026-06-25 23:43
Base commit: 2484d81

## Context
The session focused on simplifying tool contracts and UI labels so the model is not forced to supply unnecessary intention text for mechanical tools. The user wanted `task_read` and `task_write` to be fixed UI-only actions, `load_skill` / `unload_skill` to be similarly mechanical, and `replace_block` to keep `purpose` but show a relative file path in the UI.

## Changes Made
- Simplified `shell` descriptions earlier in the session to encourage compact inline host commands.
- Removed `purpose` from `task_read` and `task_write`, and replaced the UI with fixed labels: `Loading tasks` and `Saving tasks`.
- Removed `purpose` from `load_skill` and `unload_skill`, and replaced the UI with fixed labels: `Loading skill: <name>` and `Unloading skill: <name>`.
- Kept `purpose` for `replace_block`, reduced it to two sentences, and changed the UI to `Editing: <relative path> — <purpose>`.
- Made `replace_block` display paths relative to the current working directory when possible.
- Updated the runtime constructor wiring and the existing tool tests to match the new contracts.

## Decisions And Rationale
- Mechanical tools were stripped down to avoid asking the model for redundant explanation text that does not affect execution.
- `replace_block` kept `purpose` because file editing benefits from an explicit, model-authored intent summary, but the UI now supplies the file context deterministically.
- Relative paths were chosen for the UI because they are more readable and align with the current working context.

## Implementation Approach
- `task_tools.go` now exposes minimal schemas and fixed display labels for both task tools.
- `skill_tools.go` now exposes only `name` and deterministic UI text for load/unload actions.
- `replace_block.go` now accepts a work-dir resolver, converts `file_path` to a relative display path when possible, and composes the UI label from file context plus `purpose`.
- `runtime.go` passes the current work directory into `NewReplaceBlockTool`.

## Files Included
- `internal/runtime/runtime.go`: pass work dir into the replace-block tool.
- `internal/tools/replace_block.go`: keep purpose, add relative-path UI formatting.
- `internal/tools/replace_block_test.go`: update constructor usage in tests.
- `internal/tools/shell.go`: earlier shell contract refinement remains in the current worktree.
- `internal/tools/skill_tools.go`: remove purpose from load/unload skill tools.
- `internal/tools/skill_tools_test.go`: update schema and description expectations.
- `internal/tools/task_tools.go`: remove purpose from task tools and set fixed labels.
- `internal/tools/tools_test.go`: update shared FormatArgs expectations.
- `decisions/2026-06-25-2343-tool-contract-cleanup.md`: this summary.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
