# Session Decision Summary: architecture-optimization

Date: 2026-06-26 19:20
Base commit: 9dab90b

## Context
Prepare BlazeAI runtime for Telegram bridge and future web transports by decoupling model switching from global persistence, fixing a stale-provider bug in compaction, and cleaning up main.go startup for transport clarity.

## Changes Made
- Extracted `applyModel(modelID)` private method — shared validation, provider recreation, compactor sync for all model switch paths.
- Added `SetModelLocal(modelID)` for transport-local model switching without global persistence.
- `SetModel()` now calls `applyModel()` then persists globally (unchanged console behavior).
- `SetMode()` now calls `applyModel()` instead of open-coding provider creation — compactor stays synced.
- Split `main.go` into `loadRuntimeConfig()`, `openSession()`, `prepareBuiltinAssets()`, `runConsole()` for clearer transport entry points.
- Added tests for `SetModelLocal()` (no persistence, compactor sync).
- Fixed stale test drift in `internal/skills` (DiscoverFromDirs → discoverFromRoots) and `internal/prompt` (BuiltinSkillsFS removed, skills seeded via app home).
- Created `architecture_optimization.md` with full plan, scope, risks, and validation.

## Decisions And Rationale
- `applyModel()` centralizes provider replacement — avoids duplicating compactor sync across SetModel, SetModelLocal, SetMode.
- `SetModelLocal()` is the minimal public API needed for Telegram; no config/modes file touched.
- `main.go` helpers are plain functions, not a bootstrap package — keeps complexity low, no transport registry or factory.
- Fixed stale tests (DiscoverFromDirs → DiscoverGlobalFromDir) — they were broken before this session, unrelated to the optimization scope.

## Implementation Approach
- Patched `internal/runtime/runtime.go`: extracted applyModel, added SetModelLocal, updated SetMode.
- Refactored `main.go` by extracting sequential bootstrap steps into explicit helpers.
- Updated runtime tests for new API surface and compactor sync assertions.
- Fixed pre-existing test breakage in skills and prompt packages to restore `go test ./...` green.

## Alternatives Considered
- Compactor getter callback vs direct field update: chose direct field update in applyModel for simplicity.
- NewAgentOpts flag to skip mode resolution for Telegram: postponed — Telegram can override model post-construction via SetModelLocal.
- Shared command dispatch layer across transports: rejected — each transport (console, Telegram, web) has distinct command UX.

## Files Included
- architecture_optimization.md: detailed optimization plan
- internal/runtime/runtime.go: model switching decoupling, compactor sync
- internal/runtime/runtime_test.go: SetModelLocal + compactor sync tests
- main.go: startup helper extraction
- internal/prompt/prompt_test.go: test drift fix for removed BuiltinSkillsFS field
- internal/skills/skills_test.go: test drift fix for DiscoverFromDirs → discoverFromRoots

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
