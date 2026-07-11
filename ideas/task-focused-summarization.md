# Task-Focused Summarization

## Goal

Replace fixed pruning and summarization with task-aware summarization that keeps the last active task fully visible.

## Idea

- When context compaction runs, identify the latest task that is still active.
- Summarize only work that is already finished.
- Keep the active task itself uncompressed and directly visible.
- Treat completed tasks as closed history instead of always-on context.

## Why It May Help

- Reduces noise from unrelated completed work.
- Keeps the current task sharper in the prompt.
- Matches the common user mental model of one active task with summarized history behind it.

---

## Implementation (July 2026)

### Overview

The idea was implemented as an **async semantic task-switch detector** — `TaskSwitcher` — that ran a separate summarization LLM call in parallel with the main turn to detect when the user changed topic. When a switch was detected, old messages were pruned and summarized, keeping only the new task's context visible.

### Architecture

**Files:** `internal/compaction/taskswitch.go` (529 lines), `internal/compaction/taskswitch_test.go` (495 lines), plus integration in `runtime.go`, `compaction.go`, `config.go`, and UI handlers.

**Core types:**
- `DetectResult` — parsed model response: `Changed bool`, `Index int` (user message index), `Summary string`
- `TaskSwitchFile` — persisted protocol: `UserIndex`, `Summary`, optional `Error`

**Protocol files** in session folder:
- `taskswitch.json` — marker file. Empty → worker pending. Populated → result ready for consumption.
- `taskswitch_prompt.txt` / `taskswitch_response.txt` — debug artifacts (prompt sent to model, raw response).

### Flow

```
Turn N ends
    │
    ├─ ShouldDetectTaskSwitch()? (every N user turns, configurable)
    │   └─ StartTaskSwitchJob()
    │       ├─ create empty taskswitch.json marker
    │       ├─ snapshot session messages
    │       └─ goroutine: runTaskSwitchJob(ctx, ...)
    │
Turn N+1 starts
    │
    ├─ ConsumeTaskSwitchResult()
    │   ├─ taskswitch.json empty → skip (worker still running)
    │   ├─ taskswitch.json has Error → delete, report failure to UI
    │   └─ taskswitch.json valid → CompactByTaskSwitch()
    │       ├─ save summary to summaries/
    │       ├─ prune messages before switch point
    │       └─ prepend synthetic system message with all summaries
    │
    └─ Continue with pruned session
```

### Detection Transcript

A compact transcript was built from session messages:
- **Reasoning excluded** entirely.
- **Tool calls/results truncated** to 150 characters.
- **User messages indexed** as `[user N]` (0-based).
- **System messages excluded**.
- Existing summaries prepended as `[summary]` context.

This transcript was sent to the **summarization provider** with a strict system prompt instructing the model to respond with either `null` (no switch) or `{"index":"user N","summary":"..."}`.

### Concurrency Safety

- **Generation counter** (`taskSwitchGeneration uint64`): incremented on each new job and each explicit cancellation. Worker checks generation before writing results.
- **Cancel function** stored on the Manager; compaction and abort paths call `CancelTaskSwitch()`.
- **Session-scoped context** (`context.Background()` with 15-second timeout): worker outlives the per-turn request context that generated it.
- **`provider.ErrAborted`** treated as benign cancellation, not a summarization failure.

### Integration Points

- **Runtime start** (`NewAgent`): cleared stale task-switch protocol files.
- **Turn start** (`RunTurn`): consumed any pending result before appending user message.
- **Tool loop**: consumed between iterations for mid-turn application.
- **Turn end**: started new detection job only if no compaction was needed, no pending result existed, and the turn counter allowed it.
- **Compaction**: canceled any active task-switch job (compaction had priority).
- **Abort / clear / new session**: removed all task-switch state.

### UI Notifications

Detected switches and errors were displayed via `OnMaintenanceCall`/`OnMaintenanceResult`:
- Console: inline emoji + status on the same line
- Desktop: tool activity area
- Telegram: standalone activity message

### Configuration

```json
{
  "compaction": {
    "taskSwitcherTurns": 3
  }
}
```

- `0` → disabled
- `1` → every turn
- `N` → every N user turns (default: 3)

---

## Removal (July 2026)

### Why it was removed

The feature was fully implemented and tested but **did not provide relevant benefits** in practice:

1. **False positives**: the summarization model frequently detected "task switches" on follow-up questions, clarifications, and implementation detail changes — all of which should have been treated as the same task.
2. **Cumulative summaries**: summaries tended to re-summarize old content rather than being strictly incremental, creating redundancy and noise.
3. **Latency cost**: a separate LLM call per N turns added latency and token cost with no meaningful improvement in conversation quality.
4. **Complexity**: the async protocol (marker files, generation counters, cancellation, two consumption points) added significant code surface for marginal gain.
5. **The original idea — keeping the last active task uncompressed — was already served by token compaction's boundary-safe pruning**, which preserves recent messages and only summarizes older spans.

### What remained

- Token compaction (message pruning + summarization at context limits) — unchanged.
- Maintenance UI notifications (`OnMaintenanceCall`/`OnMaintenanceResult`) — kept for compaction display.
- Summary storage and synthetic message injection — kept (used by token compaction).

### Files removed

- `internal/compaction/taskswitch.go`
- `internal/compaction/taskswitch_test.go`
- `plans/task-switcher.md`
