# Session Decision Summary: specs-manager-user-directives-and-discovery

Date: 2026-06-30 14:30
Base commit: 4e89d80

## Context
User requested three enhancements to the specs-manager skill: a user-owned directives section in generated specs.md, a discovery fallback chain for project rationale, and renumbered workflow steps.

## Changes Made
- Added `## User Directives` section to the `specs_dot_md_template` in [DATA], placed between title and Description, with placeholder text noting items should be marked with `!!IMPORTANT!!` or `!!MANDATORY!!`.
- Added step 6 to Path A: before writing any file, ask the user for project-specific directives. LLM may reformulate text for clarity without distorting meaning.
- Added safety rule: `## User Directives` is user-owned. LLM reads and follows directives, modifies only at explicit user request, never on own initiative.
- Added discovery fallback chain to Scanner Rules and Path A step 3: check `decisions/` (list then read only relevant summaries) → `git log --oneline` → rely on code only. Decisions are read only when relevant to the section being written (Description, or a specific spec concept). Map-only does not need decisions.
- Renumbered all workflow steps (Path A/B/C) from original to accommodate the new step 6, verified sequential 1–27 without gaps or duplicates.

## Decisions And Rationale
- User Directives section: placed at the top of specs.md so it is immediately visible to the LLM as a set of project-specific rules. Using `!!IMPORTANT!!` / `!!MANDATORY!!` markers provides clear prioritization. Owned by the user protects against unintended LLM modifications.
- Discovery fallback chain: decisions/ → git log → code. This gives the best available rationale for architectural choices without over-reading. Decisions are filtered by relevance (not "latest 20") to avoid context waste in large projects with many decision files.
- LLM reformulation: allowed because the user's raw text may need structural clarity, but meaning must be preserved intact.

## Alternatives Considered
- Reading "latest 20 decisions blindly": rejected because it wastes context in projects with many decision files and may miss relevant older ones while reading irrelevant recent ones.

## Files Included
- `skills/specs-manager.md`: all three enhancements (User Directives, discovery chain, step renumbering).

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
