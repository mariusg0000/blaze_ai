# Session Decision Summary: console-ui-task-location

Date: 2026-06-25 16:30
Base commit: e925b31

## Context
Three separate changes requested in quick succession: (1) simplify the user prompt label to show only the active mode, (2) reorder and truncate the response separator columns, and (3) move tasks.md to the working directory. Additionally, the coding skill was deleted entirely because its directives duplicate AGENTS.md.

## Changes Made

### Console UI
- `internal/console/console.go`: `promptLabel()` now returns `[<mode> mode]>` (e.g. `[planning mode]>`, `[default mode]>`) — no USER prefix, no model ID. `responseSeparator()` now shows CTX, model, then working dir (absolute path, tail-truncated to 30 chars with `...` prefix when over). Added `truncatePathTail()` helper. Removed unused `path/filepath` import.
- `internal/console/console_test.go`: Updated `TestPromptLabelWithMode` and `TestPromptLabelWithoutMode` to expect new format (no model in label, no USER text).

### Task Location
- `internal/tools/task_tools.go`: `task_write` and `task_read` now use `t.workDir()` directly instead of `platform.EnsureProjectDir()`. Tasks are stored at `tasks.md` in the working directory, not in `app_home/projects/<project>/sessions/`. Removed `internal/platform` dependency.

### Coding Skill Removal
- `skills/coding.md`: Deleted. Its directives already exist in AGENTS.md (injected into every prompt when present in the working directory). Having both caused duplication and confusion.

## Decisions And Rationale
- Prompt label simplification: USER is redundant since the user is obviously the one typing. Model is shown in the separator instead. Mode is the only context-relevant info at the prompt.
- Separator reorder (CTX, model, path): model next to CTX groups runtime context; truncated tail path shows what matters (the project name) with `...` for length awareness.
- Working dir for tasks.md: tasks are project-local by nature. Storing them in `app_home/projects/...` made them invisible when browsing the working directory and required app-home knowledge to find. Direct storage is simpler and more transparent.
- Coding skill removal: AGENTS.md is the authoritative source for coding rules. A secondary skill adds load overhead and creates confusion when directives differ or drift.

## Files Included
- `internal/console/console.go`: promptLabel + responseSeparator + truncatePathTail
- `internal/console/console_test.go`: Updated test expectations
- `internal/tools/task_tools.go`: tasks.md in working directory, removed platform dep
- `skills/coding.md`: Deleted
- `decisions/2026-06-25-1630-console-ui-task-location.md`: This summary

## Commit Linkage
This summary is committed together with the implementation changes.
