## Feature Description
Compaction and TaskSwitcher summaries now use compact chronological work logs that preserve requirements, plans, task-list status, decisions, implementation, validation, and unresolved items without reproducing code or prompt templates. The obsolete `specs-manager` builtin rule and documentation references were removed.

## Rationale And Implementation
Summary output must preserve continuation-critical work history rather than dump prompt or source-code excerpts. The runtime prompts now enforce telegraphic chronological records and near-complete plans/task lists. Both runtime system-prompt copies no longer require the unavailable `specs-manager` skill, and builtin skill documentation/spec maps now match the actual skill set.

## Modified Files
- internal/compaction/compaction.go: chronological token-compaction summary prompt
- internal/compaction/taskswitch.go: chronological pre-switch summary rules
- prompts/sysprompt.md: removed obsolete hard rule
- cmd/blazeai-desktop-backend/resources/prompts/sysprompt.md: removed embedded obsolete hard rule
- skills/skill-manager.md and cmd/blazeai-desktop-backend/resources/skills/skill-manager.md: removed obsolete builtin listing
- specs/02-architecture.md and specs/09-skill-system.md: removed obsolete skill references
- tasks.md: recorded completed work
