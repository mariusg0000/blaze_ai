# Session Decision Summary: tool-purpose-ui

Date: 2026-06-23 11:45
Base commit: cc5aeb5

## Context
User wanted a cleaner tool display: replace the verbose `[CALL]`/`[OK]`/`[ERROR]`/`[TIMEOUT]` labels with directional arrows (`[>>> tool]` / `[<<< tool]`) and add a `purpose` field for LLMs to describe what each tool call does, rather than show raw command arguments.

## Changes Made
1. Added `Purpose` field to all tool argument structs (`ShellArgs`, `ReplaceBlockArgs`, `SkillArgs`) with JSON tag `purpose,omitempty`.
2. Updated JSON schema for every tool to include `purpose` as a required string property with tailored descriptions.
3. Made `FormatArgs()` prefer `purpose` when present, falling back to existing argument display (command, file path, skill name).
4. Changed console tool call rendering from `[CALL] tool_name args` to `[>>> tool_name] description`.
5. Changed console tool result rendering from `[OK]/[ERROR]/[TIMEOUT] tool_name preview` to `[<<< tool_name] ok/error/timeout: preview`.
6. Added fallback FormatArgs tests for every tool and schema validation helper.

## Decisions And Rationale
- `purpose`, not `thoughts` or `reasoning`: avoids confusion with reasoning/content semantics. Purpose is a clear directive for the LLM to state intent.
- `omitempty` JSON tag: if the LLM omits it, tool execution is unaffected — FormatArgs gracefully falls back to command/args.
- Separator `tools ---` kept: user confirmed the grouping section is useful visual context.

## Implementation Approach
- Pure UI metadata change — no effect on tool execution logic.
- Each tool's `FormatArgs()` checks `purpose` first, then falls back on its existing display logic.
- Console `OnToolCall`/`OnToolResult` format changed in-place with matching test assertions updated.

## Files Included
- `internal/console/console.go`: tool call/result format
- `internal/console/console_test.go`: updated assertion strings
- `internal/tools/shell.go`: ShellArgs purpose, schema, FormatArgs
- `internal/tools/replace_block.go`: ReplaceBlockArgs purpose, schema, FormatArgs
- `internal/tools/skill_tools.go`: SkillArgs purpose, schema, FormatArgs
- `internal/tools/tools_test.go`: preference + fallback tests, schema helper
- `internal/tools/shell_test.go`: schema validation
- `internal/tools/replace_block_test.go`: schema validation
- `internal/tools/skill_tools_test.go`: schema validation

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
