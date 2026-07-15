# Session Decision Summary: readline panic fix and fd/rg file tools

Date: 2025-07-15 08:45

## Context

A `slice bounds out of range [:-1]` panic was triggered in the vendored readline library (`third_party/readline/emacs.go:transposeWords`) when ESC interrupted streaming. The panic was reproducible: streaming LLM output, pressing ESC, then pressing transpose-words (`Alt+t`) caused `selection.Pop()` to return `-1` positions that were later used as slice bounds. Separately, `list_files` and `search_files` tools using `fd` and `rg` directly were developed but not yet committed.

## Changes Made

- `third_party/readline/emacs.go` — Added empty-line early return in `transposeWords()` and `shellTransposeWords()`. Added validation after every `selection.Pop()` call checking for negative, zero-length, out-of-range, and inverted bounds. Added final ordering validation before slice assembly.
- `internal/console/console.go` — Added `readLineSafely()` wrapper around `rl.Readline()` that uses `defer`/`recover` to convert unexpected readline panics into ordinary errors with full stack traces.
- `internal/console/console_test.go` — Added `TestReadLineSafelyConvertsPanic` verifying the recovery wrapper converts a nil-shell panic into a diagnostic error.
- `internal/tools/list_files.go` — New fd-based file/directory discovery tool with typed args, output limits, and missing-fd error.
- `internal/tools/search_files.go` — New rg-based text/regex search tool with typed args, output limits, and missing-rg error.
- `internal/tools/helper_exec.go` — Shared bounded execution helper for direct read-only helper wrappers (fd, rg).
- `internal/tools/files_tools_test.go` — Tests for list_files and search_files tools.
- `internal/runtime/runtime.go` — Registered `ListFilesTool` and `SearchFilesTool` in the agent tool registry.

## Decisions And Rationale

- **Defensive validation in readline as primary fix**: Validates `selection.Pop()` results and slice bounds before every slice operation. This is the correct fix because it addresses the root cause — invalid positions entering slice expressions.
- **Recovery wrapper as secondary safety net**: Placed only at the `Readline()` call boundary in the console transport. Logs the original stack trace and returns a normal error so the caller can stop cleanly. Does not hide the problem — it preserves diagnostics.
- **Both `transposeWords` and `shellTransposeWords` patched**: They share the same `selection.Pop()` pattern and could produce the same panic.
- **fd and rg tools use direct exec, not shell**: Consistent with the project's no-fallback policy. Missing tools produce explicit errors. Shared bounded-execution logic lives in `helper_exec.go`.
- **Pre-existing untracked changes included**: The fd/rg tools and their registration in `runtime.go` were ready and validated; committing them together avoids orphaned files.
