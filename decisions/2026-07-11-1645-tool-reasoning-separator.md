# Session Decision Summary: Tool-Reasoning Separator

Date: 2026-07-11 16:45

## Context

The console transport produced inconsistent spacing between post-tool output blocks: `tool → content` had a blank line separator, but `tool → reasoning` did not. The user reported the missing blank line as a visual defect.

## Changes Made

- Added a `toolsStarted` check in `OnReasoning`, identical to the existing pattern in `OnContent`.
- When reasoning starts after tool output, a blank line is printed before the `🧠` prefix and `toolsStarted` is reset.

## Decisions And Rationale

Consistency requires the same transition behavior regardless of which output type follows a tool group. The `toolsStarted` flag already tracks this boundary correctly; `OnReasoning` was simply missing the check.

## Validation

- `go test ./...` — passed
- `go build ./...` — passed
- `go vet ./...` — passed
- `git diff --check` — passed
