# Session Decision Summary: Maintenance UI notifications

Date: 2026-07-11 12:45

## Context

Compaction and TaskSwitcher ran silently or exposed only a separate system notification. Users needed visible progress and completion status to understand context maintenance and choose a different summarization model when it failed.

## Changes Made

The runtime Handler contract now supports maintenance start/result callbacks. Compaction displays an inline tool-style activity with a compaction emoji, pruned-message count, and success/error/timeout status. A confirmed TaskSwitcher displays the same style with a topic-change emoji and pruned-message count. Console, desktop, and Telegram handlers render these callbacks through their existing tool activity paths, keeping the final status on the same line. Async TaskSwitcher errors are persisted through the protocol file so they can reach the next runtime boundary and UI.

## Decisions And Rationale

A dedicated maintenance callback was chosen instead of pretending internal operations are real tool calls in the runtime contract. Transport implementations reuse tool-style rendering to preserve the requested one-line start/result interaction. No notification is emitted for a TaskSwitcher `null` result or while detection is still pending, avoiding false topic-change messages. Counts come from the actual pruned session prefix. Timeout and deadline errors use the existing tool status protocol so they render with the timeout badge.

## Validation

- `go test ./...`
- `go build ./...`
- `git diff --check`
