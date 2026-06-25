# Session Decision Summary: coding-desc-analysis-phase

Date: 2026-06-25 16:15
Base commit: a502b73

## Context
The previous imperative DESCRIPTION only covered code production verbs (write, edit, generate, refactor, debug). When the user asked to "analyze a project and implement something", the LLM saw "analyze" first — which wasn't in the trigger list — and skipped loading the coding skill entirely. The loading decision happened at the start of the turn and never recovered.

## Changes Made
- `skills/coding.md`: DESCRIPTION expanded to cover the full task lifecycle: analysis, review, understanding, planning, implementing, building — not just production verbs. Exclusion narrowed from "simple file reads" to "read-only file inspection without modification intent".

## Decisions And Rationale
- Analysis and planning are part of the coding workflow and must trigger the coding skill at the start, not after the LLM has already decided to skip it. Once skipped, the skill is never loaded for the remainder of the session.
- "Skip only for read-only file inspection without modification intent" is more precise than "simple file reads" — it excludes passive exploration but catches any task with implementation intent.

## Files Included
- `skills/coding.md`: Expanded DESCRIPTION triggers
- `decisions/2026-06-25-1615-coding-desc-analysis-phase.md`: This summary

## Commit Linkage
This summary is committed together with the implementation changes.
