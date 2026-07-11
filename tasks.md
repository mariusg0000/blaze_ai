# Tasks

- [x] Add cancellable, generation-guarded TaskSwitcher job state
- [x] Reorder runtime finalization so token compaction has priority over starting TaskSwitcher
- [x] Remove pending TaskSwitcher protocol files when token compaction cancels a job
- [x] Add regression tests for pending cancellation, stale results, and trigger ordering
- [x] Run `go test ./...` and `go build ./...`
