# Session Decision Summary: session features + UI fixes

Date: 2026-06-24 17:30
Base commit: d3cffa3

## Context
Session continuation with multiple small improvements: ASCII table separator, task planning tools, markdown rendering fix, and helper catalog expansion.

## Changes Made
1. **ASCII table separator** — `responseSeparator()` renders three-line box-drawing table with CTX tokens, workdir, current model.
2. **Task tools** — `task_write` and `task_read` for project-scoped task persistence (`<project>/tasks.md`).
3. **Underscore italic fix** — `_` delimiter now checks word boundaries, preventing `task_write` from rendering as italic.
4. **pandoc + sqlite3 helpers** — added to helpers catalog (KindCore) and `setup_helpers` skill installation guidance.
5. **sysprompt** — added "Task Planning" section encouraging task tools for multi-step work.

## Decisions And Rationale
- Three-line table separator: user explicitly requested ASCII table with corners.
- Task tools use `func() string` closure over `agent.WorkDir` for project-scoped storage.
- Underscore italic fix: regex `(?:^|\s)_([^_]+)_(?:\s|$)` replaces `toggleDelimited("_")`.
- pandoc/sqlite3 as KindCore: always relevant, not project-conditional.
- Task Planning in sysprompt: ~35 tokens, zero new code, LLM decides when to use task tools.

## Files Included
- internal/console/console.go: ASCII table separator + underscore italic fix
- internal/console/console_test.go: updated TestOnUsage
- internal/tools/task_tools.go: new file (task_write + task_read)
- internal/runtime/runtime.go: register task tools
- internal/helpers/helpers.go: pandoc + sqlite3
- internal/helpers/helpers_test.go: updated for 7 core helpers
- prompts/sysprompt.md: Task Planning section
- skills/setup_helpers.md: pandoc + sqlite3 installation guidance
