# Decision Summary: Safe cleanup and low-risk optimizations

Date: 2026-07-17 15:30

## Context

Post-simplification optimization pass. User requested all safe optimizations without functional changes: remove dead code, reduce per-turn overhead, and add debugPrompt config flag for prompt.json.

## Changes Made

### Config
- Added `debugPrompt` boolean to `Config` struct, default false, controls prompt.json generation.
- Tests for default value and JSON round trip.

### Runtime
- Removed `Handler.OnReasoning` from interface and all implementations.
- Moved prompt.json serialization to after mode directive injection so it captures exact final payload.
- Gated prompt.json writes behind `DebugPrompt` flag.
- Removed pre-append sanitize at turn start; loop sanitize retained.

### Provider
- Buffered raw SSE capture per stream: one open/flush/close per provider call instead of per-event.
- Removed stale reasoning-level comments from provider docs.

### Tools
- Cached OpenAI tool definitions in Registry with invalidation on Register/Remove.
- Defensive copy returned from AllToOpenAI.
- Tests for cache invalidation and defensive copy.

### Dead code removal
- Deleted `internal/memory/` package (3 files, unreferenced).
- Removed `KindContextual`, `ProjectFiles`, `ProjectRelevant` from helpers (unused).
- Removed commented reasoning renderer and console reasoning state.
- Removed stale runnable-skill absence tests.

### Cleanup
- Corrected stale source comments: reasoning suffix/fields, contextual helpers, web transport, memory tools, skill Syntax/Code.
- Removed stale memory prompt fixtures from test suites.
- Updated specs and builtin skills for all changes.

## Decisions And Rationale

- **debugPrompt defaults false**: No fallback; omission means disabled, which matches the no-fallback rule.
- **No prompt/skill/helper caching**: Preserves live rebuild behavior; same-session disk changes remain visible.
- **No session persistence format change**: Full-file rewrite retained as source of truth.
- **No modes fallback fix**: Out of scope for this optimization patch.

## Validation

- `go test ./...` — PASS
- `go build ./...` — PASS
- `git diff --check` — PASS
- Active grep for stale tokens — PASS
