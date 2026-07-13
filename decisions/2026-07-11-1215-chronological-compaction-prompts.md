# Session Decision Summary: Chronological Compaction Prompts / Remove Specs-Manager

Date: 2026-07-11 12:15

## Context

Summary output was dumping prompt or source-code excerpts instead of preserving continuation-critical work history. The runtime referenced the obsolete `specs-manager` builtin skill that no longer existed.

## Changes Made

- `internal/compaction/compaction.go`: chronological token-compaction summary prompt
- `internal/compaction/taskswitch.go`: chronological pre-switch summary rules
- `prompts/sysprompt.md`: removed obsolete hard rule
- `cmd/blazeai-desktop-backend/resources/prompts/sysprompt.md`: removed embedded obsolete hard rule
- `skills/skill-manager.md` and `cmd/blazeai-desktop-backend/resources/skills/skill-manager.md`: removed obsolete builtin listing
- `specs/02-architecture.md` and `specs/09-skill-system.md`: removed obsolete skill references
- `tasks.md`: recorded completed work

## Decisions And Rationale

Compaction and TaskSwitcher summaries now use compact chronological work logs that preserve requirements, plans, task-list status, decisions, implementation, validation, and unresolved items without reproducing code or prompt templates. The obsolete `specs-manager` builtin rule and documentation references were removed. Both runtime system-prompt copies no longer require the unavailable skill, and builtin skill documentation/spec maps now match the actual skill set.

## Validation

- `go test ./...` — passed
- `go build ./...` — passed
- `git diff --check` — passed
