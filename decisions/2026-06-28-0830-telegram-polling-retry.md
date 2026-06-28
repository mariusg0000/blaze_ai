# Session Decision Summary: telegram-polling-retry

Date: 2026-06-28 08:30
Base commit: 8c9ae58

## Context
The Telegram bridge stopped overnight after a transient `getUpdates` transport failure. The bridge process had no in-process retry for long-poll network resets, and the service policy was still documented as `Restart=on-failure`.

## Changes Made
Added retry/backoff handling for transient Telegram polling failures in `internal/telegram/telegram.go`, including startup drain polling, so `EOF`, timeouts, and connection resets do not terminate the bridge. Added focused tests for retry classification and retry behavior. Updated Telegram bridge docs, the customization guide, and the Telegram spec to describe the new failure policy and `Restart=always` systemd guidance.

## Decisions And Rationale
The bridge now retries transient polling errors in-process because a long-poll reset is a recoverable transport condition, not a fatal application state. `Restart=always` was documented because the service should still recover if the process exits for another reason. IPv4-forcing was not baked in because the reported failure is better handled as transport hardening first.

## Implementation Approach
Wrapped Telegram polling in a small retry loop with a short fixed backoff. Classified retryable errors by standard network/error signals and common transport strings. Kept non-retryable errors fail-fast. Verified the change with targeted Go tests.

## Alternatives Considered
Forcing IPv4 system-wide or in the bridge was considered but not implemented because it is a narrower workaround than the observed failure pattern justifies.

## Files Included
- internal/telegram/telegram.go: retry/backoff for polling and startup drain.
- internal/telegram/telegram_test.go: retry/error-classification coverage.
- skills/customize-me/docs/telegram.md: systemd restart guidance and polling resilience notes.
- telegram.md: bridge plan updated to reflect retry policy.
- specs/06_telegram.md: spec updated with polling failure policy.
- decisions/2026-06-28-0830-telegram-polling-retry.md: session record for the commit.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
