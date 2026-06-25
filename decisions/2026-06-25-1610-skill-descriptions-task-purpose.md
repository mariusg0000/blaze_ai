# Session Decision Summary: skill-descriptions-task-purpose

Date: 2026-06-25 16:10
Base commit: 3268b9f

## Context
User reported two issues: (1) the coding skill was not being loaded by the LLM during coding sessions, and (2) the task_write/task_read tools lacked the `purpose` field in their schemas, breaking consistency with the other four tools. Additionally, the project-hub skill was loading on overly broad triggers (e.g., "todo" queries in unrelated working directories).

## Changes Made
- `skills/coding.md`: DESCRIPTION changed from passive "Load when..." to imperative "MUST load using `load_skill coding` before...". The LLM now sees a concrete action command with an explicit tool name, matching the pattern that works for the skill-manager gate.
- `internal/tools/task_tools.go`: Added `purpose` field to both `task_write` and `task_read` schemas, consistent with shell/load_skill/unload_skill/replace_block. Purpose is `required` in the JSON schema and displayed by `FormatArgs` with fallback to content/empty.
- `home_folder_backup/skills/project-hub/skill.md`: DESCRIPTION narrowed — removed overly broad triggers ("project", "notes", "planning") and added explicit exclusion rule for working-directory task queries. Also edited the live user skill at `/home/marius/blazeai/skills/project-hub/skill.md` (outside repo).

## Decisions And Rationale
- Imperative DESCRIPTION format works because it gives the LLM a concrete action (`load_skill coding`) rather than a passive guideline ("Load when..."). Same pattern as the Mandatory Skill Manager Gate which the LLM consistently respects.
- Purpose as `required` in the schema forces the LLM to articulate intent before each tool call, improving UX (visible purpose in tool display) and reducing speculative tool use.
- Project-hub trigger narrowing is defense-in-depth: common words like "project" and "notes" match too many user queries, causing spurious skill loads in unrelated working directories.

## Implementation Approach
- Coding DESCRIPTION: replaced first sentence, kept body identical.
- Task tools: added `Purpose` field to args structs, added `purpose` to JSON schema properties + required arrays, updated `FormatArgs` to prefer purpose display, added `"strings"` import.
- Project-hub DESCRIPTION: added explicit DO NOT trigger rule for working-directory task queries, narrowed matching vocabulary.

## Files Included
- `skills/coding.md`: Imperative DESCRIPTION
- `internal/tools/task_tools.go`: purpose field in both task_write and task_read schemas
- `home_folder_backup/skills/project-hub/skill.md`: Narrowed DESCRIPTION
- `decisions/2026-06-25-1610-skill-descriptions-task-purpose.md`: This summary

## Commit Linkage
This summary is committed together with the implementation changes.
