# Session Decision Summary: create-skill-app-home

Date: 2026-06-23 12:40
Base commit: 21678dd

## Context
The `create_skill` builtin skill already used `{APP_HOME}` as a placeholder, but the text was ambiguous about whether it was a literal folder name or a runtime-injected variable. The LLM could misinterpret it and hardcode paths instead of using the injected variable.

## Changes Made
- Clarified the `[DETAILS]` section in `skills/create_skill.md` to explicitly state that `{APP_HOME}` is the real on-disk folder and must be used as-is in commands.
- Added a line instructing the model to always use the injected `{APP_HOME}` variable when creating or referencing custom skill files.
- Unified the `How To Create` step to use backtick formatting for `{APP_HOME}/skills/<name>.md`.

## Decisions And Rationale
- The fix is purely textual in the builtin skill definition — no code changes needed.
- Explicit phrasing prevents the LLM from guessing or hardcoding absolute paths.

## Files Included
- `skills/create_skill.md`: clarified `{APP_HOME}` usage for custom skill storage and creation steps.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
