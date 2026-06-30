# Session Decision Summary: clean project content null

Date: 2026-06-30 13:35
Base commit: 42c78e5

## Context
Three issues in one session: (1) [PROJECT CONTEXT] showed explanatory text + NULL even when no specs.md existed. (2) Section header [PROJECT CONTEXT] and variable {PROJECT_CONTENT} were inconsistent. (3) specs-manager [DESCRIPTION] referenced specs.md/specs/ which enticed the model to read them without loading the skill.

## Changes Made
- prompts/sysprompt.md: removed explanatory text, renamed section to [PROJECT CONTENT] to match variable name
- internal/prompt/prompt.go: added PROJECT_CONTENT, AGENTS_CONTENT to allowsEmptyTemplateValue so empty files produce "" not "NULL"
- internal/prompt/prompt_test.go: updated test to expect empty not NULL for missing AGENTS.md
- skills/specs-manager.md: removed specs.md/specs/ folder reference from [DESCRIPTION] — only trigger concepts remain

## Decisions And Rationale
- NULL should never appear for optional file sections; empty is correct.
- Section header and template variable must match (both now PROJECT CONTENT).
- DESCRIPTION must not mention operated files — only trigger concepts.

## Files Included
- prompts/sysprompt.md
- internal/prompt/prompt.go
- internal/prompt/prompt_test.go
- skills/specs-manager.md
- decisions/2026-06-30-1335-clean-project-content-null.md
