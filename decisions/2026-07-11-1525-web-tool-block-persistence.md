# Session Decision Summary: Preserve Web Tool Blocks

Date: 2026-07-11 15:25

## Context

The web transport displayed a tool call while it was executing, but the tool row disappeared or was replaced after the next streaming event. The prior implementation identified blocks by type or by the last block, which was unsafe when reasoning, assistant content, tool calls, and maintenance events interleaved.

## Changes Made

- `internal/web/server.go`
  - Added an explicit server-side block index to SSE block payloads.
  - Split append and replacement operations into `appendBlock` and `replaceBlock`.
  - Replay now includes each stored block index.
- `internal/web/handler.go`
  - Tracks assistant and reasoning block indices per turn.
  - Replaces only the exact block that is being streamed.
  - Tool results replace the most recently appended tool-call block instead of searching by type.
- `internal/web/page.go`
  - Uses the server-provided index for streaming replacements.
  - New blocks are appended without searching for another block of the same type.

## Decisions And Rationale

Block identity is now positional and explicit. A type is not unique: many assistant, reasoning, and tool blocks can exist in one transcript. Tracking indices prevents a later event from overwriting an earlier tool or assistant row.

Tool calls remain visible after completion by replacing their own pending row with the completed badge/result row. Assistant and reasoning streams also update their own rows without affecting neighboring blocks.

## Validation

- `gofmt -w internal/web/*.go` — passed
- `go build ./...` — passed
- `go vet ./...` — passed
- `go test ./...` — passed
- `git diff --check` — passed
