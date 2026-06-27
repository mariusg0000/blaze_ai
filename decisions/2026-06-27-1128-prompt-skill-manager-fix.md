# Session Decision Summary: prompt-skill-manager-fix

Date: 2026-06-27 11:28
Base commit: 6ace696

## Context
Multiple sessions of runnable skills debugging revealed three structural issues:
1. Prompt builder emitted `NULL` for empty `SKILLS_ACTIVE` placeholder.
2. `skill-manager` had unescaped `[SYNTAX]`/`[CODE]` markers in examples, which caused the parser to truncate the `[BEHAVIOR]` section prematurely — the model received incomplete rules and searched disk for syntax info despite having the skill loaded.
3. `disk-space` skill had malformed `[SYNTAX]` repeating the skill name with verbose options.

## Changes Made
- Fixed `internal/prompt/prompt.go`: registered `SKILLS_ACTIVE` as known variable so empty values render cleanly without `NULL`.
- Fixed `skills/skill-manager.md`: escaped `[SYNTAX]` and `[CODE]` markers in runnable examples to `\[SYNTAX\]` and `\[CODE\]` so the parser does not treat them as section boundaries.
- Fixed `/home/marius/blazeai/skills/skill-manager/skill.md`: same escape fix applied to app-home copy.
- Fixed `/home/marius/blazeai/skills/disk-space/skill.md`: corrected `[SYNTAX]` from `disk-space [--human] [--all] [--path <dir>]` to `[--all] [--path <dir>]` — no skill name, no verbose prose.
- Added test `TestParseKeepsBehaviorAfterEscapedRunnableExamples` in `internal/skills/skills_test.go`.
- Updated skill-manager `[DESCRIPTION]` in both repo and app-home to explicitly mention runnable skills.
- Added "load this skill first, do not browse" rule to skill-manager `[BEHAVIOR]`.

## Decisions And Rationale
- `[BEHAVIOR]` section in prompt for active skills is by design (builder injects it). The real bug was truncation, not presence.
- Escaping section markers in examples is the minimal fix — parser already handled `\[escaped\]` correctly, examples just needed to use the escape form.
- The parser's `extractSection` ends at `\n[` — any unescaped `[SECTION]` inside examples, even inside fenced code blocks, would truncate. No change to parser needed; fix is in the skill content.
- App-home skill-manager was stale because `SeedBuiltins` does not overwrite existing files. Manual sync was required for both copies.

## Implementation Approach
- Parser `extractSection` looks for `\n[` as section boundary. Escaped `\[` is not matched. Changed example markers to `\[...\]` so they stay inside `[BEHAVIOR]`.
- Test verifies that `[BEHAVIOR]` retains content after escaped example blocks and that no spurious top-level `Syntax`/`Code` fields appear.
- Prompt builder fix: added `SKILLS_ACTIVE` to the `hasVar` switch so empty value renders as empty string instead of `NULL`.

## Alternatives Considered
- Changing the parser to ignore markers inside fenced blocks would be more robust but adds complexity for a single content fix. Escape-in-content is simpler and matches the existing convention.
- Replacing the entire section-based parsing with a structured format (YAML frontmatter, etc.) was rejected as scope expansion.

## Files Included
- `internal/prompt/prompt.go`: registered SKILLS_ACTIVE as known variable
- `skills/skill-manager.md`: escaped section markers, updated description, added "load first" rule
- `internal/skills/skills_test.go`: new parser test for escaped markers in behavior
- `internal/prompt/prompt_test.go`: updated for clean empty-section behavior (from prior session)
- `internal/runtime/runtime_test.go`: test updates from runnable skills work (prior session)
- `internal/tools/skill_tools.go`: run_skill tool (prior session)
- `internal/tools/skill_tools_test.go`: run_skill tests (prior session)
- `internal/console/console_test.go`: console test updates (prior session)
- `prompts/sysprompt.md`: updated runnable skills placeholder (prior session)

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
