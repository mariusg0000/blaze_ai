# Session Decision Summary: AGENTS.md Rewrite

Date: 2026-07-11 14:36

## Context

The old AGENTS.md was a generic "Transparent Coding Assistant" guide with three distinct modes (Planning, Implementation, Commit) inherited from a previous generic tool. The project has since been rebuilt as BlazeAI — a Go-native AI terminal agent — and the old rules no longer matched the actual workflow or architecture.

## Changes Made

- `AGENTS.md` — complete rewrite from 385 lines to 58 lines. Replaced the 3-mode generic assistant guide with a compact BlazeAI-specific rule set covering: language, implementation mode, coding style, testing, documentation commit workflow, decision summaries, and commit messages.

## Decisions And Rationale

- **Removed Planning Mode / Implementation Mode / Commit Mode separation.** The old 3-mode system was designed for a different tool environment. BlazeAI's runtime already handles mode-like behavior via the system prompt and active skills; duplicating it in AGENTS.md added confusion and bloat.
- **Condensed to essential project rules.** The old file contained generic engineering advice (scope control, code standards, validation, completion reports) that is either covered by `specs.md` or by the runtime prompt itself. Keeping a single compact rule file reduces prompt overhead and ambiguity.
- **Aligned commit workflow with specs.md.** The new commit workflow explicitly forbids overwriting `~/.local/bin/blazeai` and defers to the wrapper script — matching the spec requirement in `specs.md`.
- **No fallback behavior.** Removed all "if uncertain" / "stop and ask" fallback language, consistent with the user spec's no-fallback rule.
