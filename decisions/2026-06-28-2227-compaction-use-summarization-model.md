# Session Decision Summary: compaction-use-summarization-model

Date: 2026-06-28 22:27
Base commit: 1398b0c70a50526fb38528f16c1812308ac65f01

## Context
Context compaction was using the default/current model for summarization, while a dedicated `summarization` role existed in config. The user wanted compaction to use the configured `summarization` model.

## Changes Made
Added `SummarizationProvider *provider.Client` to `compaction.Manager`. The `summarize()` method now uses `SummarizationProvider` (falling back to `Provider` when nil). `runtime.go` creates a dedicated summarization client when `summarization` role is configured with a different model.

## Decisions And Rationale
A dedicated client avoids coupling compaction summarization to the active agent model. The summarization role is optional per spec: if unset or set to the same model as default, no extra client is created.

## Implementation Approach
- `internal/compaction/compaction.go`: new field, updated constructor, conditional provider in `summarize()`.
- `internal/runtime/runtime.go`: resolves `summarization` model via `cfg.ModelForRole`, creates second client only when the model differs.
- `internal/compaction/compaction_test.go`: updated all `NewManager` calls to three-argument form.

## Files Included
- `internal/compaction/compaction.go`: SummarizationProvider support.
- `internal/runtime/runtime.go`: summarization client creation.
- `internal/compaction/compaction_test.go`: updated constructor calls.
- `decisions/2026-06-28-2227-compaction-use-summarization-model.md`

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
