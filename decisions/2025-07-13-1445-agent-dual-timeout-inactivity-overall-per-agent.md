# Session Decision Summary: Agent dual timeout system

Date: 2025-07-13 14:45

## Context

Child agents had a single fixed 2-minute timeout that did not reset on activity. An active child doing continuous tool calls could be killed mid-work. The user requested two separate timeouts: an inactivity timeout that resets on each tool event, and an overall hard-cap timeout configurable per agent definition with a 20-minute default.

## Changes Made

- `internal/agents/agents.go` — Added `Timeout time.Duration` to `Definition` struct; parses `timeout` front matter key via `time.ParseDuration`; validates positive duration.
- `internal/agents/agents_test.go` — Added 3 tests: timeout parsed correctly, default zero when omitted, invalid duration rejected.
- `internal/runtime/agent_orchestration.go` — Replaced single `childTimeout` with dual-context system: `childInactivityTimeout` (2 min, resets on activity) and `defaultChildOverallTimeout` (20 min, never resets). Added `activityForwarder` struct that wraps `childHandler` and signals a channel on each `OnToolCall`/`OnToolResult`. Goroutine watches channel and resets inactivity timer. Distinct error messages for inactivity vs overall timeout.

## Decisions And Rationale

- **Two separate timeouts** — Inactivity (2 min) catches stuck/deadlocked children. Overall (20 min) prevents runaway children that keep producing activity.
- **Per-agent override via `timeout` front matter** — Agents with large tasks can request more time; agents with small tasks can enforce tighter limits. Zero means use the 20-minute default.
- **`activityForwarder` wraps `childHandler`** — Clean separation: `childHandler` still suppresses child text and emits activity; `activityForwarder` adds the inactivity reset signal without modifying the existing handler.
- **Non-blocking signal send** — Buffer of 1 with default case avoids blocking the handler when the timer has already been reset by a recent event.
- **Context hierarchy `otctx` → `actx`** — Overall timeout cancels everything. Inactivity cancels only `actx`. Parent cancellation checked first for clean abort detection.
