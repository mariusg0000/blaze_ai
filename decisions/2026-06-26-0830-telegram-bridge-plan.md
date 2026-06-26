# Session Decision Summary: telegram-bridge-plan

Date: 2026-06-26 08:30
Base commit: e9cc777

## Context
User requested a Telegram bridge transport for BlazeAI. After architectural discussion, agreed on a single-bot/single-chat-per-instance model with per-instance state, no multi-chat multiplexing, and configuration via skill.

## Changes Made
- `telegram.md`: detailed architecture and implementation plan for the Telegram bridge transport

## Decisions And Rationale
- One `runtime.Agent` per bot instance, one process per bot: matches the existing 1:1 runtime shape
- `app_home/telegram/<instance>/bridge.json` for static config, `state.json` for mutable state (selected_model)
- Per-instance `workDir` for session isolation via existing `projects/` storage
- Model switching in Telegram uses a new local setter method that does not touch global `config.json` or `modes.json`
- No mode switching in Telegram v1
- Configuration managed by a skill, not by runtime wizard

## Implementation Approach
- New `internal/telegram/` package implementing `runtime.Handler`
- Extended `main.go` with `--telegram <instance>` flag
- New `SetModelLocal()` in runtime for instance-local model persistence
- 8-phase implementation plan documented in telegram.md

## Files Included
- `telegram.md`: full architecture and implementation plan
- `decisions/2026-06-26-0830-telegram-bridge-plan.md`: this summary
