# Session Decision Summary: custom-skill-folders-and-divider-color

Date: 2026-06-23 13:00
Base commit: 3cb9782

## Context
Two separate requirements were implemented back-to-back: (1) migrating custom skills from flat `.md` files to folders with `skill.md` and a `{SKILL_DIR}` injection variable, and (2) making divider line colors match their labels for visual consistency.

## Changes Made

### Custom skill folder layout
- Custom skills now live in `{APP_HOME}/skills/<skill-name>/skill.md` folders instead of flat `.md` files.
- Sub-resources (scripts, data, templates) can be created inside the skill folder.
- Added `{SKILL_DIR}` injection variable that resolves to the skill's folder path when the skill description or details block is injected into the prompt.
- Added a `Dir` field to `skills.Skill` to record the on-disk folder for custom skills.
- `discoverCustomFromDir` scans subdirectories looking for `skill.md`; builtin skills remain flat `.md` files.
- `DiscoverFromFS` (the runtime discovery path) had a bug: it still called the old `discoverFromDir` instead of `discoverCustomFromDir` for custom skills. Fixed.

### Divider color consistency
- Modified `divider()` so the separator line uses the label color when set (instead of always light gray).
- Changed `closeToolGroup()` to pass `colorGreen`, so both the top and bottom lines of the tools group are green.
- `ctx` separator line now matches the `colorPurple` label color.

### create_skill skill updated
- Full rewrite of `[DETAILS]` to describe the folder layout, all four injectable variables, bash-first execution preference, and Python venv-only rule.

## Files Included
- `internal/console/console.go`: divider line color matches label + closeToolGroup green
- `internal/prompt/prompt.go`: `{SKILL_DIR}` injection via `injectVariablesForSkill`
- `internal/prompt/prompt_test.go`: `{SKILL_DIR}` injection test
- `internal/skills/skills.go`: `discoverCustomFromDir`, `Skill.Dir`, `DiscoverFromFS` fix
- `internal/skills/skills_test.go`: custom folder discovery tests
- `skills/create_skill.md`: rewritten for folder layout, variables, execution rules

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
