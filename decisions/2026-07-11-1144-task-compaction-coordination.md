# Session Decision Summary: Task compaction coordination

Date: 2026-07-11 11:44

## Context

The runtime previously started TaskSwitcher before the main LLM call and skipped token compaction whenever the TaskSwitcher protocol marker existed. A slow detector could therefore keep the marker pending across turns and starve token compaction indefinitely.

## Changes Made

`internal/runtime` now decides token compaction first after a completed LLM turn and starts TaskSwitcher only when the token threshold was not reached. `internal/compaction` tracks an active cancel function and generation, cancels pending detection during token compaction, removes protocol files, and rejects stale workers before they publish results. Tests were updated for next-turn TaskSwitcher application and pending protocol cancellation.

## Decisions And Rationale

Token compaction is the hard safety mechanism and must not be gated by an asynchronous semantic detector. TaskSwitcher remains asynchronous so normal turns do not wait for summarization, but its result is consumed at the next turn boundary. Cancellation alone is not sufficient because a worker could finish after file removal, so generation validation is used before writing a result.

## Validation

- `go test ./...`
- `go build ./...`
- `git diff --check`
