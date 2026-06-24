# Session Decision Summary: skill-manager gate

Date: 2026-06-24 12:20
Base commit: 2f677bae8c68ba1481f01b47b2da6a0dd9162c27

## Context
The system prompt was updated to make skill editing safer and less fragmented. The new rule adds an explicit gate around any skill operation so the model must have `skill-manager` active before touching skill files.

## Changes Made
Updated `prompts/sysprompt.md` with a `Mandatory Skill Manager Gate` section. The prompt now says that no skill operation is allowed unless `skill-manager` is active, and that the next tool call must be `load_skill skill-manager` when it is not active.

## Decisions And Rationale
The gate is explicit because skill edits are high-risk and should not start until the correct workflow skill is loaded. This also makes the prompt's instructions harder to ignore when the model is about to touch a skill file.

## Implementation Approach
This is a prompt-only change. No runtime behavior changed; the new rule is purely in the system prompt text.

## Alternatives Considered
Leaving the guidance as a soft recommendation would not prevent the model from starting a skill edit with the wrong active state. Hardcoding the workflow in more than one place was unnecessary.

## Files Included
- `prompts/sysprompt.md`: added the mandatory skill-manager gate.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
