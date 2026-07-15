# Session Decision Summary: status bar, separator, replace_block, tool removal

Date: 2026-07-15 07:40

## Context

Multiple improvements and simplifications accumulated since the last commit:
1. The status bar CTX format needed breakdown into cache hit/miss/summary tokens.
2. The boxed response separator was too heavy; a simple hyphen line was preferred.
3. `replace_block` partial-success behavior caused confusion and needed all-or-nothing semantics.
4. `list_files` and `search_files` tools were redundant wrappers; `shell` can run `fd` and `rg` directly.

## Changes Made

- `internal/console/console.go` — Replaced boxed metadata separator with terminal-width hyphens (max 80). Updated status bar to `CTX xxk(H:xxk|M:xxk|S:xxk)` format. Added `assistantContentRendered` turn state to gate separator emission. Added `readLineSafely()` recovery wrapper. Changed `formatStatusTokens` suffix from uppercase K to lowercase k.
- `internal/console/console_test.go` — Replaced `TestOnUsage`/`TestOnUsageZero` with `TestBuildStatusBarContextBreakdown`, `TestResponseSeparator`, and `TestResponseSeparatorWithoutContent`.
- `internal/compaction/compaction.go` — Added public `SummaryTokens(sessionFolder)` method estimating synthetic summary token cost.
- `internal/tools/replace_block.go` — Changed to all-or-nothing: pre-validates all blocks against one file snapshot, rejects on any failure, returns complete live file plus compact diagnostics. Removed file-size limit on live content return.
- `internal/tools/replace_block_test.go` — Rewrote `TestReplaceBlockMultiplePartialSuccess` as `TestReplaceBlockMultipleAllOrNothing` asserting no file changes on failure.
- `internal/runtime/runtime.go` — Removed `list_files` and `search_files` tool registration.
- `internal/tools/list_files.go` — Deleted.
- `internal/tools/search_files.go` — Deleted.
- `internal/tools/files_tools_test.go` — Deleted.

## Decisions And Rationale

- **Hyphen separator over boxed table**: The boxed table was visually heavy and complex to maintain. A plain hyphen line matching terminal width (capped at 80) is simpler and sufficient.
- **H:M:S status format**: Explicit breakdown gives the user visibility into cache hits, misses, and summary token overhead without guessing.
- **Lowercase k**: Consistent with standard token-counting conventions.
- **All-or-nothing replace_block**: Partial application created ambiguity about which blocks were applied. Pre-validation with a single snapshot eliminates this and makes retries predictable.
- **Remove list_files/search_files**: These tools added schema and parsing complexity for functionality `rg`, `fd`, and `find` already provide natively through shell. Removing them reduces tool count and eliminates regex parsing issues caused by the wrapper's artificial parameter separation.
