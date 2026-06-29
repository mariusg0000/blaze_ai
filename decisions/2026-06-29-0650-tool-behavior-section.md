# Session Decision Summary: tool-behavior-section

Date: 2026-06-29 06:50
Base commit: e0f3049

## Context
The tool purpose text was always generated in English by the LLM, even when the user was conversing in another language. The user wanted the purpose text to match the conversation language while keeping English for technical terms.

## Changes Made
- Added a new `[TOOL BEHAVIOR]` section to `prompts/sysprompt.md` with a single rule: match purpose text to the user's conversation language.

## Decisions And Rationale
A central section in sysprompt avoids bloating each tool's schema with the same rule. The heading creates a natural extension point for future tool-level tips without touching individual tool code.

## Implementation Approach
Only `prompts/sysprompt.md` was changed. No Go code, tests, or schemas were modified. The section was placed after `[COMMUNICATION PROTOCOL]` because it is also a user-facing output rule.

## Alternatives Considered
- Adding the rule to each tool's purpose schema: rejected as it would repeat the same instruction three times and inflate the prompt unnecessarily.
- No change: rejected because the English-only purpose text did not match the conversation language.

## Files Included
- `prompts/sysprompt.md`: new `[TOOL BEHAVIOR]` section with the language rule.
- `decisions/2026-06-29-0650-tool-behavior-section.md`: session record for the change.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
