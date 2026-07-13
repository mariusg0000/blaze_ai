# Session Decision Summary: Maintenance UI Notifications / Compaction Task Switch

Date: 2026-07-11 12:45

## Context

Compaction and TaskSwitcher ran silently with no user-facing progress indication. Users could not tell when background maintenance was running, had completed, or had failed.

## Changes Made

- Added maintenance callbacks to `runtime.Handler`
- Reused console, desktop, and Telegram tool activity rendering so final status stays on the same line
- Added compaction/task-switch pruned counters
- Persisted asynchronous TaskSwitcher errors for runtime consumption and display
- Updated runtime integration tests and Handler mocks

## Decisions And Rationale

Show compaction and TaskSwitcher progress in the UI using the existing inline tool-activity style. Compaction starts with a compaction emoji and "Compacting on max token limits". Completion shows a checkmark and the number of pruned and summarized messages. Errors show an error badge; timeouts/deadlines show the timeout badge. Confirmed task switches start with a topic-change notification and finish with a checkmark and pruned-message count. No task-switch notification is shown for `null` or pending detection.

## Validation

- `go test ./...` — passed
- `go build ./...` — passed
- `git diff --check` — passed
