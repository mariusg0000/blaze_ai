# Session Decision Summary: Compaction prompt restructure and TaskSwitcher abort handling

Date: 2026-07-11 14:07

## Context

The compaction prompt did not separate historical summaries from newly pruned messages, causing cumulative summaries that re-summarized old content. The TaskSwitcher worker also failed with provider stream aborted errors when its parent turn context was canceled, persisting an error result that blocked the next turn.

## Changes Made

- Restructured `buildSummaryPrompt` into two clearly delimited sections: reference-only historical summaries and the new pruned messages as the exclusive summarization source. Added explicit rules against resummarization, copying, rewriting, or repeating old facts.
- Updated the prompt contract test for the new section headers and rules.
- Added `provider.ErrAborted` detection in `runTaskSwitchJob`. A canceled provider stream is treated as a benign job cancellation: the protocol file is removed and no error is persisted.
- Added a regression test for canceled context in the TaskSwitcher worker.

## Decisions And Rationale

The two-section layout makes it impossible for the model to confuse reference context with summarization input. The reference section says "do not summarize, copy, rewrite, or include" instead of the weaker "read-only context." A canceled TaskSwitcher call must not be recorded as a summarization failure; the next turn should never encounter a stale error. Genuine timeouts and actual provider errors remain distinct.

## Validation

- `go test ./...`
- `go build ./...`
- `git diff --check`
