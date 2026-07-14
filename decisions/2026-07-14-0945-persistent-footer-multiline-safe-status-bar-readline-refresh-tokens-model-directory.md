# Session Decision Summary: Persistent footer multiline-safe status bar

Date: 2026-07-14 09:45

## Context

The console transport needed a stable bottom status bar showing current session context (tokens, model, work directory) that persists while the user edits multiline prompts. Earlier `Hint.Persist` with a full-width background disappeared during keystroke redraws, so the implementation had to be hardened for readline's multiline refresh behavior.

## Changes Made

- `internal/console/console.go`
  - Added multiline-safe persistent footer using readline `Hint.Persist` with ANSI background.
  - Repersisted the footer on every primary-prompt refresh so readline redraws do not erase it.
  - Added terminal-width reading and padding that stops one column short to avoid `pending-wrap` overwrite behavior.
  - Updated footer on model changes, mode changes, work-directory changes, and after each turn.
  - Reset persisted token counts on session clear/new so the footer reflects the new session state.
  - Wired readline shell lifecycle into the console transport (`c.rl`) for safe footer updates.
- `tasks.md`
  - Added unrelated follow-up notes for reasoning block compaction because the file was already modified in this working tree.

## Decisions And Rationale

- Kept `Hint.Persist` instead of a right prompt, since the console transport must support long multiline input prompts without stealing prompt width.
- Chose to re-persist on every primary prompt refresh rather than patching readline internals, because it preserves upstream stability while achieving multiline-safe display.
- Chose a full-width dark-gray background footer with a trailing one-column margin, balancing visibility with terminal-wrap safety.

### Included unrelated or pre-existing changes

- Reasoning block compaction notes were committed from `tasks.md` because they were present in the working tree and relevant as project notes.
