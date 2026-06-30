# Session Decision Summary: project-specs builtin skill

Date: 2026-06-30 12:48
Base commit: 5466788

## Context
The user requested a new builtin skill that can generate, update, and maintain architecture specification documents for any project, modeled after BlazeAI's own `specs/` folder. The skill was designed collaboratively — user specified the format requirements (multilingual trigger words, index with keywords, max 20 files, incremental workflow).

## Changes Made
Created `skills/project-specs.md` — a new builtin skill with full `[DESCRIPTION]`, `[BEHAVIOR]`, and `[DATA]` sections.

## Decisions And Rationale
- **4-phase workflow**: Phase 0 (read-only understanding) → Phase 1 (concept identification with user approval) → Phase 2 (incremental spec generation) → Phase 3 (maintenance). The user must approve the concept list before any spec file is written.
- **Max 20 files**: hard cap to prevent runaway generation. One concept per file.
- **Multilingual trigger words**: specs/specifications + variants in Romanian, German, French, Spanish, Portuguese, Italian. The trigger must be present in the user message.
- **Index/tracker**: `specs/README.md` with maintenance instructions, index table with keywords (5-15 per file), and status tracking.
- **Incremental by default**: one spec at a time, user confirms each. Batch mode only on explicit request.
- **Embedded templates**: spec file template and tracker template live in `[DATA]` for the model to follow.

## Implementation Approach
Pure text — one new Markdown file in `skills/`. No code changes. The skill is auto-embedded via existing `//go:embed skills/*` directive.

## Files Included
- `skills/project-specs.md`: new builtin skill
- `decisions/2026-06-30-1248-project-specs-skill.md`: this decision summary

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
