# Architecture Optimization Plan

## Goal

Prepare BlazeAI for the Telegram bridge and a future web transport with the smallest useful architectural changes.

The optimization must preserve the current console behavior while removing two transport blockers:

- model switching is currently tied to global persistence
- compaction can keep using a stale provider after model changes

The optimization should also make the startup path easier to extend for multiple transports without introducing a transport registry, factory framework, or speculative abstraction.

## Current Constraints

### Runtime Model Switching Is Too Coupled

`internal/runtime/runtime.go` currently uses `SetModel()` for three different concerns:

1. validate model format and provider existence
2. recreate the provider client and update the running agent
3. persist the selected model into global config or modes state

This is correct for console mode, but it blocks Telegram and future web cases where a transport may need:

- local model switching
- per-instance persistence outside global config
- no mutation of `config.json` or `modes.json`

### Compaction Can Use A Stale Provider

`internal/compaction/compaction.go` stores a concrete provider client inside `Manager`.

`runtime.Agent` changes `a.Provider` on model and mode switches, but the compactor is created once at agent construction time.

That means compaction summarization can keep calling the old provider after a model switch.

This is already a console bug and becomes a direct blocker for Telegram per-instance model switching.

### Startup Is Linear But Too Packed Into `main.go`

`main.go` is still small, but all bootstrap work is inside one `run()` function.

That is acceptable for one transport, but Telegram and later web would make it harder to follow and maintain.

The goal is not to introduce a transport framework. The goal is only to split obvious bootstrap chunks into direct helpers.

## Approved Optimization Scope

### In Scope

- split runtime model application from runtime model persistence
- add a local non-persisting model setter for transport-specific use
- ensure compaction always uses the active provider after model or mode changes
- split `main.go` startup into clearer helper functions while preserving behavior
- add tests for the new runtime behavior
- keep the transport contract unchanged

### Out Of Scope

- no transport registry
- no command framework shared by console and Telegram
- no multi-agent runtime manager
- no refactor of session storage layout
- no refactor of prompt building
- no Telegram implementation yet
- no web implementation yet

## Target Architecture After Optimization

### Runtime Responsibilities

`runtime.Agent` should expose two model paths:

- `SetModel()`
  - validate
  - apply in memory
  - persist globally for console behavior

- `SetModelLocal()`
  - validate
  - apply in memory
  - do not persist globally

Both paths must share the same internal application logic so provider replacement and compactor synchronization happen in exactly one place.

### Compaction Responsibilities

`compaction.Manager` should always use the current active provider for summarization.

The simplest implementation path is:

- keep `Manager.Provider`
- centralize all provider switching in one runtime helper
- update both `a.Provider` and `a.Compactor.Provider` in that helper

This avoids a new callback-based abstraction while fixing the stale-provider bug.

### Startup Responsibilities

`main.go` should remain simple, but clearer.

Recommended shape:

- `run()` parses flags and chooses transport path
- shared bootstrap stays in small helpers
- console path remains explicit
- future Telegram path can reuse the same helpers later

No registry or generic transport factory is needed.

## Likely Changed Files

- `architecture_optimization.md`
- `internal/runtime/runtime.go`
- `internal/runtime/runtime_test.go`
- `main.go`

Potentially no direct changes needed in:

- `internal/compaction/compaction.go`

unless documentation there must mention the runtime-managed provider update.

## Detailed Implementation Plan

### Phase 1 - Runtime Model Split

Goal:

- isolate in-memory model application from persistence side effects

Steps:

1. Add a private runtime helper such as `applyModel(modelID string) error`.
2. Move into it:
   - model validation
   - provider client construction
   - assignment of `a.Provider`
   - assignment of `a.ModelID`
   - synchronization of `a.Compactor.Provider`
3. Rewrite `SetModel()` to:
   - call `applyModel()`
   - preserve the current persistence rules
4. Add `SetModelLocal()` to:
   - call `applyModel()`
   - skip all persistence

Constraints:

- no console behavior change
- no duplication between `SetModel` and `SetModelLocal`

Validation:

- existing `SetModel` tests still pass
- new `SetModelLocal` tests prove no global persistence

### Phase 2 - Compactor Provider Sync

Goal:

- ensure summarization uses the active provider after every model change

Steps:

1. Update `applyModel()` to refresh `a.Compactor.Provider` when non-nil.
2. Update mode switching paths to reuse the same model application helper instead of open-coding provider replacement.

Constraints:

- no new indirection unless strictly necessary
- one source of truth for provider replacement

Validation:

- tests confirm compactor provider matches `a.Provider` after `SetModel`, `SetModelLocal`, and `SetMode`

### Phase 3 - Startup Cleanup

Goal:

- make `main.go` transport-ready without architecture bloat

Steps:

1. Extract config loading into a small helper.
2. Extract session resolution into a small helper.
3. Extract builtin asset resolution and seeding into a small helper.
4. Extract console startup into `runConsole(...)` or equivalent.
5. Keep `run()` as a direct orchestration function.

Constraints:

- no new package just for bootstrap
- no behavior change
- no Telegram branch yet unless required by compilation shape

Validation:

- console startup tests or build remain unchanged
- `go test ./...` passes

### Phase 4 - Test Coverage

Goal:

- prove the optimization works and does not regress current behavior

Tests to add or update:

1. `SetModelLocal()` updates `Agent.Provider` and `Agent.ModelID`.
2. `SetModelLocal()` does not modify persisted global model state.
3. `SetModel()` still updates persisted global model state.
4. `SetMode()` still works.
5. Compactor provider matches the active provider after:
   - `SetModel()`
   - `SetModelLocal()`
   - `SetMode()`

### Phase 5 - Validation And Follow-Up

Goal:

- confirm the codebase is now ready for Telegram runtime integration

Validation commands:

- `go test ./internal/runtime ./internal/console ./internal/compaction`
- if clean, `go test ./...`

Expected outcome:

- console remains unchanged
- runtime now supports transport-local model switching
- compaction provider no longer goes stale after model changes
- `main.go` is easier to extend with Telegram and web startup paths

## Risks And Tradeoffs

### Risk: Over-Refactoring Startup

Avoid extracting a bootstrap framework. Only extract plain helpers that remove obvious repetition.

### Risk: Hidden Mode Interaction

`SetMode()` currently also changes provider/model. If it does not reuse the same internal path, the stale-provider bug can remain.

Mitigation:

- ensure `SetMode()` reuses the shared model application logic

### Risk: Accidentally Changing Console Semantics

Console depends on global persistence today.

Mitigation:

- keep `SetModel()` behavior exactly the same from the console's perspective
- cover it with tests

## Final Decision

The correct optimization is a small runtime-centered refactor, not a system-wide redesign.

The implementation should deliver:

- shared in-memory model application
- explicit local model switching for future transports
- synchronized compactor provider
- clearer startup helpers in `main.go`

This is the minimum architecture work that meaningfully reduces risk before Telegram and future web transport implementation.
