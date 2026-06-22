# Session Decision Summary: Prompt section labels for all injected sources

Date: 2026-06-22 15:30
Base commit: a8b8363

## Context
The LLM was using `shell` to read memory instead of using the memory content already injected into the system prompt. The model didn't recognize memory content because it was injected without any label. The same issue applied to AGENTS.md.

## Changes Made
- **prompt.go**: Memory and AGENTS.md are now wrapped with clear runtime-injected headers:
  - `## Memory (memory.md)` — tells the model the content is the persistent memory file and not to use shell to read it
  - `## Project Rules (AGENTS.md)` — labels instructions from the working directory
- **sysprompt.md**: Added `## Memory` section with explicit rules about automatic memory injection
- **skills/memory.md**: Moved the "no load for reading" rule to `[DESCRIPTION]` (always visible) and removed it from `[DETAILS]`

## Decision Rationale
- The model needs to know what each injected section is and why it's there
- Headers prevent redundant shell calls that waste tokens and confuse the user
- Universal sysprompt rules about memory prevent the behaviour at the system level

## Files Changed
- `internal/prompt/prompt.go`: headers for memory and AGENTS.md
- `prompts/sysprompt.md`: Memory section with automatic-injection rules
- `skills/memory.md`: DESCRIPTION clarifies load vs read, DETAILS simplified
