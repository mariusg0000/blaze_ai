# Session Decision Summary: parser-fix-backup-improvements

Date: 2026-06-25 14:50
Base commit: aea1ee4

## Context
Multi-part session analyzing real-world LLM behavior issues with skill creation and loading. User identified multiple bugs through prompt.json inspection.

## Changes Made

### Parser Fix (critical)
- `internal/skills/skills.go`: `extractSection` now requires `[SECTION]` markers at start of line (`\n[SECTION]`) instead of anywhere in text. Fixes bug where inline references like `` `[DATA]` `` inside BEHAVIOR content caused the parser to find the wrong section boundary. This caused skill-manager's DATA to contain BEHAVIOR text and the BEHAVIOR to appear duplicated in prompts.

### Display Fix
- `internal/tools/skill_tools.go`: `load_skill` success message strips `global/` prefix from displayed name (outputs `skill loaded: name` not `skill loaded: global/name`).

### Prompt Cleanup
- `prompts/sysprompt.md`: Removed all skill structure/layout information. Now just placeholder section headers. Skill details belong in skill-manager, not the universal prompt.
- `prompts/sysprompt.linux.md`, `prompts/sysprompt.darwin.md`, `prompts/sysprompt.windows.md`: Clarified "Executable scripts (bash programs, not AI skills)" to prevent LLM from saving skills in scripts/.

### Skill Content Rewrite
- `skills/skill-manager.md`: Complete logical reorganization. BEHAVIOR now flows: Purpose → Format → Structure → Where Skills Live → Creating/Editing (with injectable variables) → Choosing Scope → Restoring Builtins. DATA reduced to dense technical keys without duplicating BEHAVIOR.
- `skills/customize_me.md`: Updated to escaped variables, added folder structure emphasis.

### Backup Script
- `home_folder_backup/backup_home.sh`: Removed memories folder. Added project skills backup (`projects/<name>/skills/`). Made additive (uses `cp -ruT`, never deletes from backup). 
- `home_folder_backup/README.md`: Updated included/excluded folders.
- Backup artifacts (config/, scripts/, skills/) from script run included.

## Decisions And Rationale
- Parser fix is defense-in-depth: inline `[SECTION]` references should never interfere with section parsing. The `\n[` prefix ensures markers must be at start of line.
- Sysprompt minimalism: structural skill info (paths, format, variables) belongs in skill-manager BEHAVIOR/DATA, not the universal prompt. Saves tokens and prevents confusion.
- Additive backup: user explicitly requested that deleted skills remain in backup. `cp -ruT` achieves this by only copying newer files.

## Files Included
- `internal/skills/skills.go`: Parser fix
- `internal/tools/skill_tools.go`: Display fix
- `prompts/sysprompt.md`: Minimal skills section
- `prompts/sysprompt.{linux,darwin,windows}.md`: Scripts clarification
- `skills/skill-manager.md`: Full rewrite
- `skills/customize_me.md`: Variable/path updates
- `home_folder_backup/backup_home.sh`: Additive + project skills
- `home_folder_backup/README.md`: Updated
- `home_folder_backup/{config,scripts,skills}/`: Backup artifacts
- `decisions/2026-06-25-1450-parser-fix-backup-improvements.md`: This summary

## Commit Linkage
This summary is committed together with the implementation changes.
