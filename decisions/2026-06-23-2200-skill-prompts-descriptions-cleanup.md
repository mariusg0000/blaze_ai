# Session Decision Summary: skill-prompts-and-descriptions-cleanup

Date: 2026-06-23 22:00
Base commit: 4520e94 (2026-06-23 17:57)

## Context
Two interrelated problems were discovered during review of LLM behavior with BlazeAI skills:

1. **Active skill state divergence on resume**: `ActiveList` starts empty per session and is never reconstructed on `-c` resume. The prompt builder injects `## Active Skills` solely from `active.List()`. However, the LLM saw `load_skill` tool results in conversation history and inferred (incorrectly) that skills were still active.

2. **`create_skill` parser collision**: The `create_skill` [DESCRIPTION] contained the literal text `[DESCRIPTION] and [DETAILS]`, which the naive substring parser treated as a real section header, producing corrupted [DETAILS] injected into `## Active Skills`.

3. **Missing load cues**: Skill [DESCRIPTION] blocks were missing "Load when…" hints, making LLM skill activation decisions unreliable. Inert `[TRIGGER]` sections from imported Hermes skills were present but unsupported by BlazeAI's parser.

## Changes Made

### Prompt guard against history-vs-state confusion
- `internal/prompt/prompt.go`: Added explicit rule in `buildSkillsSection` after Available Skills block: "Only skills listed under `## Active Skills` are active right now. Do not infer current active skills from older `load_skill` or `unload_skill` tool results in the conversation history. If there is no `## Active Skills` section below, then no skills are currently active."
- `internal/prompt/prompt_test.go`: Two assertions added — one for the general guidance text in no-active-skills case, one for the history-versus-state guidance in active-skills case.

### Skill [DESCRIPTION] rewrite with load cues
All 6 skills (4 builtin + 2 custom) had descriptions rewritten to answer:
- **Load when**: user intent, topic, or task
- **Use for**: what the skill provides

Builtin skills updated:
- `skills/create_skill.md` — "Load when the user wants a new skill or a skill update."
- `skills/customize_me.md` — "Load when the user wants to configure BlazeAI models or providers."
- `skills/memory.md` — "Load when the user wants to store, update, or clean persistent memory."
- `skills/setup_helpers.md` — "Load when host tools are missing or the user asks about installing helpers."

Custom skills updated:
- `~/blazeai/skills/duckduckgo_search/skill.md` — removed inert `[TRIGGER]` block
- `~/blazeai/skills/node_server/skill.md` — removed inert `[TRIGGER]` block

### `create_skill` internal fixes
- Removed `[DESCRIPTION]` and `[DETAILS]` literal text from the short description to prevent parser collision
- Added `## Description Rules` section with rules and GOOD/BAD examples
- Explicitly named both section headers in `## Skill Format` and `## How To Create`
- Fixed variable injection confusion: replaced "Always use the injected `{APP_HOME}` variable" with descriptive text ("BlazeAI injects the app home path automatically")

## Decisions And Rationale
- Kept `ActiveList` semantics unchanged (not persisted, starts empty on resume per spec)
- Fixed the confusion at the prompt layer only — explicit guidance prevents LLM from mistaking history for current state
- Chose description-based load cues over re-adding `[TRIGGER]` as a parsed section (simpler, no parser changes, no schema changes)
- Removed `[TRIGGER]` entirely from custom skills since BlazeAI parser ignores it

## Files Included
- `internal/prompt/prompt.go`: active skills state guidance
- `internal/prompt/prompt_test.go`: assertions for new guidance
- `skills/create_skill.md`: description rules, variable fix, header naming
- `skills/customize_me.md`: load-cue description
- `skills/memory.md`: load-cue description
- `skills/setup_helpers.md`: load-cue description

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
