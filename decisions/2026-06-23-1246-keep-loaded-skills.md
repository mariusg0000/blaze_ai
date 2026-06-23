# Session Decision Summary: keep-loaded-skills

Date: 2026-06-23 12:46
Base commit: e5e3dac

## Context
The LLM was reflexively unloading domain skills immediately after completing a single action, causing unnecessary skill reloads on follow-up turns and producing duplicate assistant responses. The `unload_skill` model was wrong: it treated every domain skill as a disposable one-shot utility rather than session-scoped working context.

## Changes Made
- Added three rules to the universal sysprompt under "Tool Discipline":
  - Keep relevant loaded skills active across follow-up turns on the same topic/task
  - Do not unload immediately after one successful action if the conversation is continuing in the same domain
  - Unload only on clear topic/task change or when the skill would interfere with the next turn
- Updated `UnloadSkillTool.Description()` and `unload_skill` parameter schema to explicitly frame unloading as a topic-shift action, not a reflex cleanup
- Added tests for the new guidance rules in the prompt builder and the unload skill description
- Fixed a pre-existing test (`TestBuildRuntimePartNoSkills`) that was reading real skills from `HOME` instead of isolating to `TempDir`

## Decisions And Rationale
- The fix is entirely at the guidance level (prompt rules + tool descriptions) — no runtime enforcement or heuristics, keeping the implementation minimal
- The rules live in the universal sysprompt to apply OS-independently and across all models

## Implementation Approach
- Added three new bullet points to "Tool Discipline" in `prompts/sysprompt.md`
- Extended `UnloadSkillTool.Description()` and updated the `name` and `purpose` descriptions in its JSON schema
- Added two string assertions to `TestBuildRuntimePartFull` and a new `TestUnloadSkillDescription` check
- Isolated `TestBuildRuntimePartNoSkills` with `t.Setenv("HOME", t.TempDir())` to prevent real skill file interference

## Files Included
- `prompts/sysprompt.md`: keep-loaded guidance
- `internal/tools/skill_tools.go`: unload description and schema updated
- `internal/prompt/prompt_test.go`: new assertions and test isolation fix
- `internal/tools/skill_tools_test.go`: new description verification test

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
