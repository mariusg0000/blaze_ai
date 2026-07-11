# Session Decision Summary: Remove TaskSwitcher

Date: 2026-07-11 14:27

## Context

The async semantic task-switch detector was implemented but never provided relevant benefits. It generated false positives, cumulative summaries, added latency costs, and increased code complexity without measurable improvement in conversation quality.

## Changes Made

- Deleted `internal/compaction/taskswitch.go` (529 lines) and `internal/compaction/taskswitch_test.go` (495 lines).
- Deleted `plans/task-switcher.md`.
- Removed 5 Manager fields (`taskSwitchTurnCounter`, `taskSwitchMu`, `taskSwitchCancel`, `taskSwitchGeneration`, `lastTaskSwitchPruned`) and 4 methods (`LastTaskSwitchPruned`, `ShouldDetectTaskSwitch`, `CompactByTaskSwitch`, `userIndexToSessionIndex`) from compaction.go.
- Removed `TaskSwitcherTurns` from config struct and defaults.
- Removed integration from runtime.go: `RemoveTaskSwitchState` startup/close/abort/clear calls, `ConsumeTaskSwitchResult` at both turn-start and tool-loop, `StartTaskSwitchJob` and `HasTaskSwitchState` at turn-end, `taskSwitchAppliedThisTurn` flag.
- Deleted 5 task-switch tests from runtime_test.go.
- Removed `task_switch` emoji from console toolEmoji.
- Documented the full implementation and removal rationale in `ideas/task-focused-summarization.md`.

## Decisions And Rationale

The feature was removed because it did not provide relevant benefits (false positives, latency, complexity, redundancy with token compaction). Token compaction continues to handle context pruning. The `OnMaintenanceCall`/`OnMaintenanceResult` handler methods are preserved because compaction uses them. All other compaction infrastructure (summary storage, synthetic messages, summarization provider) remains intact.

## Validation

- `go test ./...`
- `go build ./...`
- `git diff --check`