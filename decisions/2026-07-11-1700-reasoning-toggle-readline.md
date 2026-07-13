# Session Decision Summary: Ctrl+T Reasoning Toggle

Date: 2026-07-11 17:00

## Context

The readline migration replaced the custom raw input parser, which also removed the Ctrl+T reasoning toggle (`0x14 → "reasoning_switch"` → toggle + save + status line). The toggle was silently lost.

## Changes Made

- Overrode the readline default `\C-t` binding (transpose-chars) with a custom command `blazeai-reasoning-toggle`.
- Registered the command via `Keymap.Register`, which toggles `ShowReasoning`, persists the config, and displays a transient status message via `PrintTransientf`.
- Transient messages disappear on the next keystroke, which matches the expected ephemeral UX.

## Decisions And Rationale

Readline's API uses `Config.Bind` + `Keymap.Register` for custom commands. `PrintTransientf` is the correct output method because it does not collide with the active input buffer.

## Validation

- `go build ./...` — passed
- `go test -count=1 ./...` — passed
- `go vet ./...` — passed
- `git diff --check` — passed
