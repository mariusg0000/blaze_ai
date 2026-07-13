# Session Decision Summary: Compaction Task Switch Maintenance

Date: 2026-07-11 13:45

## Context

Compaction summaries were losing analysis conclusions (findings, verdicts, uncertainty, evidence). Async TaskSwitcher failed immediately when the completed turn context was canceled. Console maintenance results showed a generic `CTX` token suffix instead of pruned-message details.

## Changes Made

- Extended the compaction prompt and prompt contract test
- Made TaskSwitcher worker context session-scoped with explicit manager cancellation
- Added runtime regression coverage for canceled turn contexts
- Added a dedicated console maintenance result renderer and formatting tests

## Decisions And Rationale

Preserve analysis conclusions during token compaction — summaries retain final findings, verdicts, uncertainty, evidence, and relevant file references. Async TaskSwitcher detection stays alive after a turn by using a session-scoped worker context with explicit manager cancellation instead of the turn context. Console compaction/task-switch results show pruned-message details instead of the generic `CTX` token suffix. Maintenance errors and timeouts remain inline with their start message.

## Validation

- `go test ./...` — passed
- `go build ./...` — passed
- `git diff --check` — passed
