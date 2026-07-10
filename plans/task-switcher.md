# Async Task-Switcher Implementation Plan

## Goal

Replace the current blocking task-switch detection path with an asynchronous, file-backed task-switch protocol.

The main LLM turn must never wait for task-switch detection after the last visible token. Task-switch detection runs in parallel with the main turn, has its own timeout, writes a validated result file, and is consumed only by the main runtime at safe prompt-build boundaries.

## Current Problem

Current flow in `internal/runtime/runtime.go`:

- `RunTurn()` starts task-switch detection in a goroutine.
- Detection calls the summarization provider with `context.Background()`.
- After the main LLM loop finishes, `RunTurn()` blocks on `detection := <-detectCh`.
- `Ctrl+C` cancels the main turn context, but does not cancel detection.
- The console cannot accept another message until detection finishes.
- Task-switch cleanup can still be applied after the user has attempted to abort.

This causes visible delay between the last streamed assistant character and the final console separator.

## Target Behavior

- Main LLM and task-switch detection start in parallel.
- Main LLM never waits for task-switch detection at turn end.
- Task-switch detection has an explicit timeout.
- Task-switch worker writes only task-switch control/result files, never `session.json`.
- Runtime is the only component allowed to mutate `session.json`.
- Runtime consumes task-switch results at safe prompt-build boundaries.
- A task-switch worker never starts if a pending or unconsumed result exists.
- Normal compaction invalidates any pending/unconsumed task-switch job.
- `/clear`, `/new`, and session close invalidate pending task-switch state.
- A stale worker must not be able to write a valid result after invalidation.

## Current Files Likely Changed

- `internal/runtime/runtime.go`
- `internal/compaction/taskswitch.go`
- `internal/compaction/compaction.go`
- `internal/compaction/taskswitch_test.go`
- `internal/runtime/runtime_test.go`
- `internal/console/console.go`

## Disk Protocol

Store task-switch state inside the session folder.

Files:

- `taskswitch.pending.json`
- `taskswitch.result.json`

All writes must be atomic:

1. write `*.tmp`
2. rename to final path

No partial JSON file should be consumed.

### Pending File

```json
{
  "job_id": "20260710-...-random",
  "status": "running",
  "snapshot_user_count": 12,
  "snapshot_message_count": 45,
  "started_at": "2026-07-10T12:00:00Z",
  "deadline_at": "2026-07-10T12:00:15Z"
}
```

### Result File: No Switch

```json
{
  "job_id": "20260710-...-random",
  "status": "done",
  "changed": false,
  "finished_at": "2026-07-10T12:00:03Z"
}
```

### Result File: Switch

```json
{
  "job_id": "20260710-...-random",
  "status": "done",
  "changed": true,
  "user_index": 10,
  "switch_user_hash": "sha256:...",
  "summary": "Summary of the old task.",
  "finished_at": "2026-07-10T12:00:03Z"
}
```

### Result File: Timeout

```json
{
  "job_id": "20260710-...-random",
  "status": "timeout",
  "changed": false,
  "error": "task-switch detection timed out after 15s",
  "finished_at": "2026-07-10T12:00:15Z"
}
```

## Worker Ownership Rules

A worker may write `taskswitch.result.json` only if:

- `taskswitch.pending.json` exists
- `pending.job_id == worker.job_id`
- session folder still exists

If any check fails:

- worker exits
- worker writes no result

## Runtime Flow

### Turn Start

After appending the user message and creating the task-switch snapshot:

1. Check if `taskswitch.pending.json` or `taskswitch.result.json` exists.
2. If either exists, do not start a new task-switch job.
3. If neither exists and `ShouldDetectTaskSwitch()` allows detection:
   - create `taskswitch.pending.json`
   - start async worker with snapshot and `job_id`
   - do not wait for it

### Safe Prompt-Build Boundary

Before every `Builder.Build(...)` inside the runtime loop:

1. `sanitizeSession()`
2. consume task-switch result if present
3. if result applied, optionally `sanitizeSession()` again
4. build prompt
5. stream main LLM call

This allows pruning during long tool loops, not only at the next user message.

### Result Consumption

When `taskswitch.result.json` exists:

1. Read and parse result.
2. Read `taskswitch.pending.json`.
3. If `status == "timeout"`:
   - delete pending + result
   - continue without pruning
4. If `changed == false`:
   - delete pending + result
   - continue without pruning
5. If `changed == true`:
   - validate `user_index`
   - validate `switch_user_hash`
   - call `CompactByTaskSwitch(sess, userIndex, summary)`
   - delete pending + result only after successful compaction

## User Index Safety

Appending messages after the snapshot does not invalidate earlier user indices. If the worker detects switch at `user_index = 10`, later user messages become `11`, `12`, etc. The boundary remains stable under append-only mutation.

Still validate `switch_user_hash` because non-append mutations can happen:

- normal compaction
- task-switch compaction
- `/clear`
- `/new`
- stale worker result

## Normal Compaction Interaction

Normal compaction changes the session prefix, so it invalidates any active task-switch snapshot.

Before normal compaction mutates `session.json`:

1. delete `taskswitch.pending.json`
2. delete `taskswitch.result.json`
3. proceed with normal compaction

## Abort Interaction

If the main turn is aborted:

- runtime should not wait for task-switch worker
- pending task-switch state for that turn should be invalidated
- delete `taskswitch.pending.json`
- delete `taskswitch.result.json` if present

## `/clear` And `/new`

Both must invalidate task-switch state.

Required behavior:

- delete pending/result files for current session
- cancel in-memory worker context if available
- ensure stale worker cannot write result because pending no longer exists

## Timeout

Task-switch timeout should be short and explicit.

Recommended initial value:

- `15s`

Preferred initial implementation:

```go
const taskSwitchTimeout = 15 * time.Second
```

## Error Policy

Follow the project no-fallback rule.

Fatal or explicit errors:

- invalid pending JSON
- invalid result JSON
- result job_id mismatch
- changed result missing summary
- changed result missing valid user_index
- switch_user_hash mismatch

Non-fatal explicit outcomes:

- timeout result
- changed false result
- worker invalidated before result write

## Implementation Steps

### Step 1: Add task-switch state types

In `internal/compaction/taskswitch.go`:

- `TaskSwitchPending`
- `TaskSwitchResult`
- constants for file names
- helper for `job_id`
- helper for user content hash

### Step 2: Add atomic JSON helpers

Add small helpers:

- `writeTaskSwitchJSONAtomic(path string, value any) error`
- `readTaskSwitchPending(folder string) (TaskSwitchPending, error)`
- `readTaskSwitchResult(folder string) (TaskSwitchResult, error)`
- `removeTaskSwitchState(folder string) error`

### Step 3: Start async worker

Replace runtime’s `detectCh` model with:

- create pending file
- start goroutine
- worker calls timeout-aware detection
- worker writes result only if pending job_id still matches

### Step 4: Make detection context-aware

Change task-switch detection from:

```go
DetectTaskSwitch(sess, summaries)
```

to:

```go
DetectTaskSwitch(ctx, sess, summaries)
```

### Step 5: Consume result at safe prompt-build boundary

In `RunTurn()` loop, before `Builder.Build(...)`:

```go
if err := a.consumeTaskSwitchResultIfReady(); err != nil {
    return err
}
```

### Step 6: Invalidate on normal compaction

Before `a.Compactor.Compact(...)` mutates session:

- delete pending/result
- then compact

### Step 7: Invalidate on abort and session reset

On abort path:

- delete pending/result for current session

On `/clear` and `/new` command handlers:

- delete pending/result before switching/clearing session

### Step 8: Remove blocking wait

Delete final blocking section based on `detectCh`.

### Step 9: Tests

Add tests for:

- main turn does not wait for slow task-switch worker
- worker writes result after successful detection
- worker writes timeout result after timeout
- runtime consumes `changed:false` and deletes files
- runtime consumes `changed:true`, applies pruning, deletes files
- runtime consumes result during same turn before follow-up LLM prompt after tool result
- normal compaction deletes pending/result
- stale worker cannot write result after pending deletion
- invalid JSON causes explicit error
- hash mismatch prevents pruning
- abort invalidates pending/result and does not wait

### Step 10: Validation

Run:

```sh
go test ./internal/compaction
go test ./internal/runtime
go test ./internal/console
go test ./...
go build ./...
```

## Risks

- File protocol bugs can leave stale pending files.
- Applying task-switch result inside tool loops must happen only at safe prompt-build boundaries.
- Timeout handling must be explicit without turning task-switch into a user-facing turn failure.

## Open Decisions

1. Should task-switch timeout be hardcoded first (`15s`) or added to config now?
2. Should timeout result be shown via `OnSystem`, or only stored/consumed as a protocol outcome?
3. Should session locking be included in this implementation or tracked as a follow-up?
