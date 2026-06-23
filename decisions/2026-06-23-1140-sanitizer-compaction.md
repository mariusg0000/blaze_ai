# Session Decision Summary: sanitizer-compaction

Date: 2026-06-23 11:40
Base commit: 0f29b1e

## Context
Production error: `blazeai -c` hit DeepSeek 400 — `assistant message with 'tool_calls' must be followed by tool messages`. The runtime persisted tool_calls as `[]tools.OpenAIToolCall` but the sanitizer only recognized `[]interface{}` and `[]map[string]interface{}`, so valid tool rounds were silently broken by sanitization.

## Changes Made
1. Redesigned `Session.Sanitize` to delegate to a standalone `SanitizeMessages` function that validates arbitrary message slices without mutating session state.
2. Added proper tool-call ID validation: orphan tool messages removed, mismatched `tool_call_id` dropped, incomplete assistant tool-call rounds truncated.
3. Removed tool-boundary logic from `findCutPoint` in compaction — cut point is now purely token-based.
4. Integrated `SanitizeMessages` into compaction: after a raw token split, orphan tool results from the retained tail are moved back into the pruned segment before summarization, preventing information loss.
5. Fixed `assistantToolCallIDs` to recognize `[]tools.OpenAIToolCall` — the in-memory type the runtime actually uses before JSON serialization.

## Decisions And Rationale
- **Sanitizer as single source of truth**: tool validity rules live only in `SanitizeMessages`. Neither compaction nor the runtime loop duplicates boundary logic.
- **Cut point is token-only in compaction**: the sanitizer handles correctness after the split. Compactor calls `SanitizeMessages` on the retained tail and moves removed messages into the pruned segment for summarization.
- **No lost tool results**: crucial for correctness when a raw token cut orphans tool results in the retained segment.

## Implementation Approach
- `SanitizeMessages` returns `([]Message, []Message)` — sanitized + removed messages, preserving original order.
- `Session.Sanitize` delegates to `SanitizeMessages` and discards removed messages.
- Compaction calls `SanitizeMessages` on the retained tail, appends removed messages to pruned before summarization.
- `assistantToolCallIDs` now handles three cases: `[]interface{}`, `[]map[string]interface{}`, `[]tools.OpenAIToolCall`.

## Files Included
- `internal/session/session.go`: sanitizer redesign, `SanitizeMessages`, `assistantToolCallIDs` with `tools.OpenAIToolCall` support
- `internal/session/session_test.go`: tests for orphan removal, ID mismatch, truncation, runtime `OpenAIToolCall` regression
- `internal/compaction/compaction.go`: removed tool-boundary logic from `findCutPoint`, integrated sanitizer into compaction
- `internal/compaction/compaction_test.go`: updated cut point test, added orphan-retention-in-summary test

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
