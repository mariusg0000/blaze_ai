# Session Decision Summary: Separate commands from hotkeys in startup splash

Date: 2026-07-16 09:45

## Context

The startup splash displayed Ctrl+T under the "Commands" section (a slash command section), had a second duplicate "Shortcuts" section after Helpers with only 3 shortcuts, and mixed hotkeys with slash commands across both sections.

## Changes Made

- `internal/console/console.go` — Removed misplaced Ctrl+T from slashCommands list; removed the duplicate Shortcuts section rendered after Helpers. Section order is now Commands → Shortcuts → Skills → Helpers → Session.
- `internal/console/console_test.go` — Updated TestStartupSplashTTY to verify all slash commands present, all 8 shortcut labels present, exactly one Shortcuts section, no Ctrl+ labels in Commands region, and full section ordering.

## Decisions And Rationale

- Commands section is reserved exclusively for slash commands (/auth, /model, /cd, /clear, /new, /exit).
- Shortcuts section contains all keyboard shortcuts (Tab, Ctrl+\\, Ctrl+F, Ctrl+R, Ctrl+T, Ctrl+], ESC, Ctrl+D).
- Eliminated duplicate section to reduce confusion.
