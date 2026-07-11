## Feature Description
Preserve analysis conclusions during token compaction, keep async TaskSwitcher detection alive after a turn, and render maintenance results with the intended inline details.

## User Behavior
- Compaction summaries retain final findings, verdicts, uncertainty, evidence, and relevant file references.
- TaskSwitcher no longer fails immediately when the completed turn context is canceled.
- Console compaction/task-switch results show pruned-message details instead of the generic `CTX` token suffix.
- Maintenance errors and timeouts remain inline with their start message.

## Implementation
- Extended the compaction prompt and prompt contract test.
- Made TaskSwitcher worker context session-scoped with explicit manager cancellation.
- Added runtime regression coverage for canceled turn contexts.
- Added a dedicated console maintenance result renderer and formatting tests.

## Validation
- `go test ./...`
- `go build ./...`
- `git diff --check`
