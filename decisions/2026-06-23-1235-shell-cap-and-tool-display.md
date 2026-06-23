# Session Decision Summary: shell-cap-and-tool-display

Date: 2026-06-23 12:35
Base commit: b5f0ba4

## Context
After the directional tool UI and `purpose` metadata were added, two follow-up issues remained. First, shell commands could still return extremely large outputs into session history and prompt context. Second, tool-call display needed to preserve full `purpose` text while keeping fallback command/path/name previews compact and readable.

## Changes Made
- Added a hard output cap to the `shell` tool at 150 kB across combined stdout and stderr.
- Stopped the running shell process immediately when the output cap is exceeded.
- Returned a strong error message with byte counts and guidance to refine the command or read the target in sequential chunks below 150 kB.
- Added explicit chunking guidance to the `shell` tool description and `command` parameter schema.
- Moved fallback truncation to tool-specific `FormatArgs()` logic so `purpose` is never truncated while fallback command/path/name previews remain capped at 80 characters.
- Adjusted console rendering so tool responses are visually distinct from tool calls: call marker stays green, response marker is blue, and status text remains color-coded.

## Decisions And Rationale
- The output cap belongs in `shell` execution, not only in UI rendering, because once a large result is stored in session history it has already polluted context.
- The cap kills the process instead of truncating and returning partial output, avoiding accidental injection of large incomplete results into the conversation.
- `purpose` should remain fully visible because it is the human-facing explanation. Only fallback display strings are truncated.
- The cap is a hard safety ceiling, not a recommended target. The schema now explicitly tells the model to narrow commands and chunk large reads.

## Implementation Approach
- Added a shared limiter for stdout and stderr writers in `internal/tools/shell.go`.
- When the combined byte budget is exhausted, the limiter triggers `killShellCommand(cmd)` exactly once and suppresses further captured output.
- After process completion, shell returns a metadata-only error message if the cap was hit.
- Added a shared truncation helper for tool fallback displays and applied it to shell command fallback, replace_block path fallback, and skill-name fallback.
- Kept console formatting changes scoped to the tool marker lines only.

## Files Included
- `internal/console/console.go`: distinct colors for tool call vs tool response markers
- `internal/tools/shell.go`: 150 kB hard cap, process stop, refinement guidance, chunking rule in description/schema
- `internal/tools/shell_test.go`: output-limit coverage for stdout and stderr
- `internal/tools/tools.go`: shared fallback truncation helper
- `internal/tools/tools_test.go`: fallback truncation coverage and purpose/fallback display expectations
- `internal/tools/replace_block.go`: fallback path truncation only when `purpose` is absent
- `internal/tools/skill_tools.go`: fallback skill-name truncation only when `purpose` is absent

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
