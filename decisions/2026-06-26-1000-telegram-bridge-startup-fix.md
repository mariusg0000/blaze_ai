# Session Decision Summary: telegram-bridge-startup-fix

Date: 2026-06-26 10:00
Base commit: b940e3d

## Context
After phases 1-6 Telegram bridge implementation, running `blazeai --telegram <instance>` would replay all pending Telegram Bot API updates (`offset = 0`). Old messages sent earlier to expose `chat.id` were processed as normal agent input, causing the LLM to run shell commands and start duplicate bridge instances.

## Changes Made
- Added `drainPendingUpdates` that fetches and discards pending updates at startup, computing the correct starting offset from `max(update_id)+1`.
- Changed `runPolling` to accept a `telegramClient` interface (instead of `*BotClient`) so drain logic is testable with mocks.
- Added `/start` as a local command that returns help, preventing it from reaching the LLM.
- Added `telegram_test.go` with tests for `drainPendingUpdates` and `nextOffsetFromUpdates`.
- Added `TestHandleCommandStartReturnsHelp` to `commands_test.go`.

## Decisions And Rationale
- `startupDrainTimeoutSeconds = 0` (instant poll) to minimize startup delay during drain.
- `telegramClient` interface extracted so drain and polling logic are fully testable without real API calls.
- `/start` mapped to same response as `/help` instead of a separate greeting, keeping command behavior minimal.

## Implementation Approach
- `runPolling` calls `drainPendingUpdates` before entering the main polling loop.
- `drainPendingUpdates` calls `GetUpdates(ctx, 0, 0)` once and computes the offset via `nextOffsetFromUpdates`.
- `nextOffsetFromUpdates` is a pure helper function, independently unit-tested.
- HandleCommand switch now includes `"/start"` in the same case as `"/help"`.

## Files Included
- internal/telegram/telegram.go: startup drain, telegramClient interface
- internal/telegram/commands.go: /start handling
- internal/telegram/commands_test.go: /start test
- internal/telegram/telegram_test.go: drain and offset tests

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
