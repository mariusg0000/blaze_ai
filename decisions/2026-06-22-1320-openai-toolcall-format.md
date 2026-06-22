# Session Decision Summary: OpenAI tool call format, response separator

Date: 2026-06-22 13:20
Base commit: fde2313

## Context
After loading a skill, the LLM returned 400: "missing field `id`" on a tool call message. The tool call format stored in session messages didn't match the OpenAI API spec. Also, user wanted a separator after the LLM response, not just before.

## Root Cause
`ToolCall` struct had no JSON tags, serializing as `{"ID":"...","Name":"...","Arguments":"..."}`. OpenAI API requires `{"id":"...","type":"function","function":{"name":"...","arguments":"..."}}`. The raw struct was sent back to the API on subsequent turns, causing deserialization failure.

## Fix
- **tools.go**: Added `OpenAIToolCall` struct matching the API format (`id`, `type`, `function.name`, `function.arguments`) and `ToOpenAIToolCall()` converter.
- **runtime.go**: Convert `resp.ToolCalls` to `[]tools.OpenAIToolCall` when building the assistant message.
- **console.go**: Added `c.userSeparator()` after `RunTurn()` completes, so the separator appears both before and after the LLM response.

## Validation
`go test ./...` — all 197+ tests pass.

## Files Changed
- `internal/console/console.go`: separator after response
- `internal/runtime/runtime.go`: convert to OpenAIToolCall
- `internal/tools/tools.go`: OpenAIToolCall struct + converter
