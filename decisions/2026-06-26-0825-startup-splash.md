# Session Decision Summary: startup-splash

Date: 2026-06-26 08:25
Base commit: 048f4a5

## Context
User requested a startup splash screen for the BlazeAI REPL console: title, commands list, available skills, current model and working folder. After discussion, evolved to sectioned layout with muted separators and per-section accent colors.

## Changes Made
- Added `showStartupSplash()` to Console — renders boxed title, sectioned Commands/Skills/Session info, only in TTY mode
- Added `sectionLabel(label, labelColor)` — helper for section headers with colored label + light gray dash separator
- Added `slashCmd` struct and `slashCommands` var — declarative command list for the splash
- Title box: yellow (`colorOrange`) border, blue (`colorBlue`) text
- CTX status table (`responseSeparator`): yellow frame, blue cell content (changed from purple)
- Updated `ResponseSeparator` TTY rendering to per-segment colored output (frame in yellow, text in blue)
- Fixed pre-existing test drift: `mockAgent` passed extra skills FS arg to `NewAgent`
- Updated all tests for new splash format

## Decisions And Rationale
- Splash only in TTY: preserves clean pipe/script output
- Section separator dashes in `colorLightGray` rather than `colorGray` for visibility
- Per-section colors (Commands=blue, Skills=green, Session=orange) to visually distinguish sections without cluttered UI
- Box + section layout preferred over full-table or card design: lightweight, modern, no fragile alignment
- Skills discovered via existing `skills.DiscoverAll` — no new discovery logic

## Implementation Approach
- `showStartupSplash` called from `Console.Run()` before the REPL loop, guarded by `IsTTY`
- Skills list formatted as 3-column grid, `global/` prefix stripped for display
- No splash output in non-TTY mode
- Tested with mocked agent and temp HOME dir for skill discovery

## Files Included
- `internal/console/console.go`: splash methods, section helper, updated response separator
- `internal/console/console_test.go`: splash tests + pre-existing drift fix
- `decisions/2026-06-26-0825-startup-splash.md`: this summary
