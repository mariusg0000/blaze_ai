# Session Decision Summary: Compaction, TaskSwitcher, and maintenance UI

Date: 2026-07-11 13:45

## Context

The compaction summary needed to preserve final analysis outcomes, while TaskSwitcher was failing immediately after each qualifying turn because its async provider call inherited the completed turn context. The maintenance UI also displayed the generic tool context token count instead of the requested pruning result.

## Changes Made

- Added mandatory analysis-outcome instructions to the token-compaction prompt, preserving verdict status, uncertainty, scope, evidence, relevant files, and practical consequences.
- Detached TaskSwitcher jobs from the per-turn context; jobs now use a session-scoped context with the existing timeout and explicit manager cancellation.
- Added a regression test proving a TaskSwitcher result survives cancellation of the completed turn context.
- Updated console maintenance rendering to show pruning details and errors inline without the generic `CTX` suffix.
- Added console coverage for maintenance success and error formatting.

## Decisions And Rationale

Analysis conclusions are treated as high-priority summary content, but hypotheses and open questions must not be promoted to confirmed findings. TaskSwitcher must outlive `RunTurn` because it intentionally runs after the main response; explicit manager cancellation remains the correct lifecycle control. Maintenance uses a dedicated renderer configuration rather than changing normal tool output, so compaction/task-switch status can show their own details without altering existing tool CTX behavior.

## Validation

- `go test ./...`
- `go build ./...`
- `git diff --check`
