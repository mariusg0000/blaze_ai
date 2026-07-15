# Session Decision Summary: reasoning-cycle-hotkey-ctrl-right-bracket-console

Date: 2025-05-24 17:55

## Context

User requested a Ctrl+] hotkey to cycle through supported abstract reasoning levels
(none/min/low/med/high/xhigh/max) for the active model in the console transport.

## Changes Made

- `internal/console/console.go` — Added `cycleReasoningLevel()` method using
  `reasoning.SupportedForModel()` for level enumeration, `ActiveReasoningLevel()` for
  current position, and `SetActiveReasoningLevel()` for persistence. Registered
  `blazeai-reasoning-next` action and bound `\C-]` via the existing readline binding
  pattern. Updated both Shortcuts sections in the startup splash.
- `internal/console/console_test.go` — Added 4 tests: unsupported-model error+no-mutation,
  cycle order (none→min→low), full wraparound (max→none), and splash shortcut display.
- `internal/console/shortcuts_test.go` — Added `\C-]` → 0x1d to the decode test.
- Provider files, runtime reasoning APIs, and the existing fixed-bottom statusbar
  restored from the rollback tag were left untouched.

## Decisions And Rationale

- Empty current level selects the first supported level on first invocation rather than
  no-opping, because users expect immediate feedback when pressing a new shortcut.
- The six-level cycle (none/min/low/med/high/xhigh/max) follows the standard reasoning
  level order defined in `internal/reasoning/levels.go`; provider-specific descriptor
  restrictions are enforced by `SupportedForModel()`.
- Unsupported models produce a concise console error and zero state mutation per the
  project's no-fallback rule.
- The hotkey uses the same readline registration and binding pattern as existing shortcuts
  (Tab, Ctrl+\ etc.) for consistent code flow and maintainability.
