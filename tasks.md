# Tasks

- [x] Delete `internal/compaction/taskswitch.go` and `internal/compaction/taskswitch_test.go`
- [x] Delete `plans/task-switcher.md`
- [x] Clean compaction.go: fields, methods, imports
- [x] Clean compaction_test.go: no task switch tests left
- [x] Clean runtime.go: all task switch integration removed
- [x] Clean runtime_test.go: task switch tests removed
- [x] Clean config.go: TaskSwitcherTurns removed
- [x] Clean console.go: task_switch emoji removed
- [x] Check specs/ references — none found
- [x] Run go test ./..., go build ./... — all pass