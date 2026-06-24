# Session Decision Summary: prompt single source

Date: 2026-06-24 09:54
Base commit: b6656786855a5b739b9adfc523adecff50d2f0bb

## Context
The prompt assembly still depended on multiple intermediate prompt fragment files for helpers, skills, memories, and AGENTS.md wrapping. The goal was to keep only one universal system prompt plus OS-specific prompt files, with all other runtime content injected directly.

## Changes Made
Consolidated runtime prompt assembly into `prompts/sysprompt.md` plus OS-specific prompt files. Added `prompts/readme.md` to document injectible variables. Removed intermediate prompt fragment files and updated the prompt builder and tests to render the full runtime sections directly from one template.

## Decisions And Rationale
Kept the prompt layout in `sysprompt.md` so the code only supplies data and order, not section wording. This preserves the desired split between prompt content and assembly logic while eliminating redundant fragment files.

## Implementation Approach
`internal/prompt/prompt.go` now builds the host helpers, skills, memories, and AGENTS.md sections directly and injects them into `sysprompt.md` via explicit placeholders. Tests were updated to use only the unified layout plus OS prompt fixture.

## Alternatives Considered
Keeping separate fragment files for helpers, skills, memories, and AGENTS.md was rejected because the user wanted one system prompt source and direct injection only.

## Files Included
- `internal/prompt/prompt.go`: consolidated runtime section assembly into one sysprompt injection path.
- `internal/prompt/prompt_test.go`: updated prompt fixtures and assertions for the unified layout.
- `internal/console/console_test.go`: aligned console prompt fixtures with the unified layout.
- `internal/runtime/runtime_test.go`: aligned runtime prompt fixtures with the unified layout.
- `prompts/sysprompt.md`: became the single runtime prompt template.
- `prompts/readme.md`: documented all injectible variables.
- `prompts/*.md` removed fragment files no longer used by runtime assembly.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
