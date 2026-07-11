## Feature Description
Show compaction and TaskSwitcher progress in the UI using the existing inline tool-activity style.

## User Behavior
- Compaction starts with a compaction emoji and `Compacting on max token limits`.
- Completion shows a checkmark and the number of pruned and summarized messages.
- Errors show an error badge; timeouts/deadlines show the timeout badge.
- Confirmed task switches start with a topic-change notification and finish with a checkmark and pruned-message count.
- No task-switch notification is shown for `null` or pending detection.

## Implementation
- Added maintenance callbacks to `runtime.Handler`.
- Reused console, desktop, and Telegram tool activity rendering so final status stays on the same line.
- Added compaction/task-switch pruned counters.
- Persisted asynchronous TaskSwitcher errors for runtime consumption and display.
- Updated runtime integration tests and Handler mocks.

## Validation
- `go test ./...`
- `go build ./...`
- `git diff --check`
