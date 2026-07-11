## Feature Description
Token-limit compaction now has priority over asynchronous TaskSwitcher detection, preventing a slow or pending detector from starving context compaction. Active TaskSwitcher jobs can be canceled and stale results are blocked from recreating protocol state.

## Rationale And Implementation
The previous runtime skipped token compaction whenever `taskswitch.json` existed, so a detector that remained pending could prevent compaction indefinitely. TaskSwitcher now starts only after the completed turn is confirmed below the token threshold; token compaction cancels active detection, invalidates its generation, removes pending protocol files, and then compacts synchronously.

## Modified Files
- internal/compaction/compaction.go: stores cancellation, mutex, and generation state for asynchronous TaskSwitcher jobs
- internal/compaction/taskswitch.go: adds cancellation, generation validation, protocol cleanup, and updated timing documentation
- internal/compaction/taskswitch_test.go: tests pending TaskSwitcher protocol cancellation
- internal/runtime/runtime.go: gives token compaction priority and starts TaskSwitcher after the token decision
- internal/runtime/runtime_test.go: verifies the new asynchronous next-turn TaskSwitcher behavior
- tasks.md: records completed implementation and validation tasks
