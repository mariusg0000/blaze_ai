# Session Decision Summary: Context Display And Tool Separator Newlines

Date: 2026-06-22 18:10
Base commit: afa7454

## Context
User wanted a human-readable context occupancy footer after each provider response, and tool separators not to collide with streamed content.

## Changes Made
- Added `OnUsage(promptTokens int)` to the Handler interface in runtime, called after each provider Stream() returns when usage is reported.
- Added `lineOpen` tracking in console to detect when streamed content ends without a newline, forcing a newline before block-level output (separators, tool call/result markers).
- Added `responseSeparator()` that embeds a compact "CTX: Nk" label when prompt tokens are available, replacing the plain `userSeparator()` at the end of each turn.
- Added `formatCompactInt()` for short human-readable token counts (e.g. 11k, 256k, 1.3k).

## Decisions And Rationale
- Context display shows only `usage.prompt_tokens` from the provider, no max/percentage — prevents misleading display when model context window is unknown.
- Tool separator newline fix uses a stateful `lineOpen` bool instead of blindly printing newlines, so the fix works correctly for both content-first and tool-first scenarios.
- `ensureLineBreakBeforeBlock()` is called before any separator or tool marker to guarantee correct separation.

## Implementation Approach
- Handler interface extended with `OnUsage(promptTokens int)`.
- Console stores the value and renders it in the end-of-turn separator.
- `OnContent` tracks whether output is mid-line via `lineOpen`.
- `OnToolCall`, `OnToolResult`, and separator functions call `ensureLineBreakBeforeBlock()` to force a newline when needed.

## Files Included
- internal/runtime/runtime.go: Handler.OnUsage, report usage after provider Stream
- internal/console/console.go: lineOpen, ensureLineBreakBeforeBlock, responseSeparator, formatCompactInt, OnUsage, tool newline guards
- internal/runtime/runtime_test.go: mockHandler.OnUsage, usage in SSE test streams
- internal/console/console_test.go: mockHandler.OnUsage, 4 new tests for context and tool newlines
