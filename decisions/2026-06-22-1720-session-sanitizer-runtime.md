# Session Decision Summary: Session sanitization before every LLM call

Date: 2026-06-22 17:20
Base commit: f2f856c

## Context
- Resuming an interrupted session caused a 400 error: "An assistant message with 'tool_calls' must be followed by tool messages responding to each 'tool_call_id'"
- The interrupted session had an assistant message with tool_calls but no matching tool results
- The same problem would occur if any tool call round was incomplete for any reason

## Changes Made
- **session.go**: Added `Sanitize()` method — walks backwards from end, strips any trailing assistant+tool-call round that lacks complete tool results
- **session.go**: Added `assistantToolCallCount()` helper
- **runtime.go**: Added `sanitizeSession()` called at the start of `RunTurn` and before each `Provider.Stream`
- **main.go**: Removed ad-hoc sanitization from `-r` path (now handled uniformly by runtime)
- **session_test.go**: 4 tests for Sanitize (incomplete single/multi, complete single/multi)
- **runtime_test.go**: Test that confirms RunTurn sanitizes before provider stream

## Decision Rationale
- Centralizing sanitization in runtime covers all entry points, not just `-r`
- If a turn is interrupted mid-tool-call, the next message from the same session is safe
- The prompt builder does not need to worry about session validity

## Files Changed
- `internal/session/session.go`: Sanitize, assistantToolCallCount
- `internal/session/session_test.go`: 4 sanitize tests
- `internal/runtime/runtime.go`: sanitizeSession before each LLM call
- `internal/runtime/runtime_test.go`: regression test
- `main.go`: removed explicit sanitize (now in runtime)
