# Session Decision Summary: Append-only compaction prompts

Date: 2026-07-11 11:52

## Context

The existing compaction prompt requested a dense summary but did not strongly separate historical summaries from the newly pruned span. The TaskSwitcher prompt had the correct basic protocol but needed stronger memory-boundary rules and explicit exclusions for related follow-ups.

## Changes Made

`internal/compaction/compaction.go` now builds an append-only technical memory prompt that summarizes only `NEW PRUNED MESSAGES`, treats historical summaries as read-only context, preserves implementation plans and continuation-critical facts, and excludes global-state inference. `internal/compaction/taskswitch.go` now builds a stricter classifier prompt with exact `[user N]` handling, clear switch/non-switch rules, and a `null` or JSON-only output contract. Contract tests were added for both prompt builders.

## Decisions And Rationale

The two prompts remain separate because compaction produces a free-form memory chunk while TaskSwitcher produces a strict boundary decision plus a pre-switch summary. Historical summaries are allowed only for continuity and deduplication; the newly pruned transcript remains the sole source for the new compaction chunk. TaskSwitcher summaries exclude the new task and later messages to prevent boundary contamination.

## Validation

- `gofmt` on modified Go files
- `go test ./...`
- `go build ./...`
- `git diff --check`
