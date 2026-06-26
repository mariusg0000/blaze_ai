# Session Decision Summary: telegram-bridge-core

Date: 2026-06-26 08:55
Base commit: b921ff1

## Context
Implement the Telegram bridge for BlazeAI as specified in `telegram.md`. The bridge is a new transport over the existing `runtime.Handler` contract, using per-instance storage under `app_home/telegram/<instance>/`.

## Changes Made
- Added `internal/telegram/` package: config/state loader, Telegram transport startup, long polling client, buffered output handler, and local slash commands.
- Wired `--telegram <instance>` CLI flag in `main.go`.
- Extended app home bootstrap to create `app_home/telegram/` and `app_home/telegram/README.md`.
- Added tests for config/state validation, command behavior, and handler buffering.

## Decisions And Rationale
- Used standard library HTTP client for Telegram Bot API instead of a third-party library, per the plan's preference for minimal dependencies.
- `SetModelLocal()` was already implemented in a prior commit — reused directly.
- Buffer-flush strategy: 500ms interval with message edit/split at 3500 chars, keeping Telegram output readable without token-by-token streaming.
- Session resume: Telegram bridge calls `session.Last()` instead of `-c`-style clean-close filtering, since a bridge session may be interrupted without `/exit`.

## Implementation Approach
- Config and state files are loaded with strict validation; any missing or invalid field stops startup with a clear error.
- `Handler` wraps `runtime.Handler` with a background flush loop, mutex-guarded content buffer, and split/retry logic for Telegram message limits.
- Commands execute bridge-local: `/model` uses `SetModelLocal()` then persists only to `state.json`.
- Polling loop serializes one turn at a time; commands skip the LLM, other text calls `agent.RunTurn()`.

## Alternatives Considered
- Third-party Telegram library: rejected to avoid adding Go module dependencies for a simple REST API client.
- Separate `cmd/telegram/main.go`: rejected per plan recommendation — a CLI flag in `main.go` keeps the entrypoint simple.

## Files Included
- internal/telegram/ (8 new files): core bridge transport
- internal/platform/platform.go: added `telegram` subfolder to bootstrap
- internal/platform/apphome_readmes.go: added Telegram README entry
- internal/platform/apphome_readmes/telegram/README.md: folder documentation
- main.go: `--telegram` flag and startup path
- internal/console/console.go: pre-existing unrelated helper display changes
- internal/console/console_test.go: pre-existing unrelated helper test changes

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
