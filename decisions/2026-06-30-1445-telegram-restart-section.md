# Session Decision Summary: telegram-restart-section

Date: 2026-06-30 14:45
Base commit: 641033d

## Context
User experienced repeated sudo prompts when restarting the Telegram bridge service with separate commands. Requested a dedicated section in the Telegram docs documenting both the no-sudo kill method and the chained-sudo method.

## Changes Made
- Added `## Restart` section after `## Startup And Services` in `skills/config-manager/docs/telegram.md`.
- Documents two restart methods: kill+systemd respawn (no sudo) and chained `systemctl restart && status` in one `sudo sh -c` call (single password prompt).

## Decisions And Rationale
- No-sudo kill method is preferred because `Restart=always` makes systemd respawn instantly, and it avoids sudo entirely.
- When sudo is required, chaining commands prevents the "3 sudo prompts" problem the user encountered with separate commands.

## Files Included
- `skills/config-manager/docs/telegram.md`: added ## Restart section.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
