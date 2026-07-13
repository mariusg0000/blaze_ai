# Session Decision Summary: Append-Only Compaction Prompts / TaskSwitcher Contract

Date: 2026-07-11 11:52

## Context

The summarizer was rewriting global session state or historical summaries instead of describing only the newly pruned span. The TaskSwitcher lacked strict output contracts for classification and boundary preservation.

## Changes Made

- `internal/compaction/compaction.go`: replaced the generic summary prompt with structured append-only instructions
- `internal/compaction/taskswitch.go`: replaced the detector prompt with stricter classification and output rules
- `internal/compaction/compaction_test.go`: added summary-prompt contract assertions
- `internal/compaction/taskswitch_test.go`: added TaskSwitcher prompt contract assertions
- `tasks.md`: recorded completed implementation and validation tasks

## Decisions And Rationale

The summarizer must describe only the newly pruned span instead of rewriting global session state or historical summaries. The TaskSwitcher must classify only clear task changes, preserve the exact `[user N]` boundary, summarize only the preceding span, and return exactly `null` or the required JSON object. Append-only memory rules and delta-focused summaries prevent context drift and redundant token usage.

## Validation

- `go test ./...` — passed
- `go build ./...` — passed
- `git diff --check` — passed
