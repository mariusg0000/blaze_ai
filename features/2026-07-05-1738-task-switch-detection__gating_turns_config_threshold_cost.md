## Feature Description
Add configurable `taskSwitcherTurns` to gate task-switch detection to run every N user turns instead of every turn, with a default of 3.

## Rationale And Implementation
Task-switch detection calls the summarization LLM on every single user turn, which is expensive (extra latency + token cost). The user wanted detection to trigger only every N turns, configurable in settings. Added `TaskSwitcherTurns` field to `Compaction` config (JSON: `taskSwitcherTurns`), an internal counter on the `Manager` struct, and `ShouldDetectTaskSwitch()` to gate the runtime's detection startup. Default is 3; 1 = every turn (legacy), 0 = disabled.

## Modified Files
- internal/config/config.go: added `TaskSwitcherTurns` field to `Compaction` struct with default 3
- internal/compaction/compaction.go: added `taskSwitchTurnCounter` and `ShouldDetectTaskSwitch()` method to `Manager`
- internal/runtime/runtime.go: wired `ShouldDetectTaskSwitch()` into `RunTurn` detection gate
- internal/runtime/runtime_test.go: set `TaskSwitcherTurns=1` in test fixture for deterministic behavior
