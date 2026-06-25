# Session Decision Summary: Skill Unload Policy

Date: 2026-06-26 00:31
Base commit: e745d2a

## Context
The user wanted a short, imperative rule in the system prompt to reduce skill load/unload churn and avoid immediate unload/reload cycles that hurt prompt caching. The session also already contained an uncommitted idea note about periodic skill review, which was kept in the same commit to leave the worktree clean.

## Changes Made
- Added a compact sticky-skill policy to `prompts/sysprompt.md` under `[SKILLS]`.
- The new rule tells the model to keep active skills loaded through likely follow-up work, avoid immediate unloads after use, and unload only when a skill is clearly irrelevant for about 10 user turns or conflicts with the current task.
- Included the previously created `ideas/periodic-skill-review.md` note in the commit.

## Decisions And Rationale
- The rule was placed in the system prompt rather than the tool description because it is a behavioral policy, not tool semantics.
- The wording is intentionally short and imperative so it is likely to survive prompt pressure.
- The policy is conservative: if the model is unsure, it keeps the skill loaded rather than churning it.

## Implementation Approach
- Updated the `[SKILLS]` block in `prompts/sysprompt.md` to add four explicit lines governing sticky active skills and delayed unloads.
- Kept the change local to the prompt template so it is applied on every LLM call without runtime code changes.

## Files Included
- `prompts/sysprompt.md`: adds the sticky skill unload policy under `[SKILLS]`.
- `ideas/periodic-skill-review.md`: existing idea note added to the commit so the repository is clean.
- `decisions/2026-06-26-0031-skill-unload-policy.md`: this summary.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
