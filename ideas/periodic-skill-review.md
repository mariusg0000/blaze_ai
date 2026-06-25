# Periodic Skill Review

## Goal

Add a lightweight background review that periodically checks whether loaded skills are still relevant and unloads stale ones.

## Flow

1. Count user messages in the current session.
2. Every 10 user messages, run a secondary LLM review.
3. Send the last 20 turns plus the list of currently active skills.
4. If no skills are active, skip the review entirely.
5. The review model decides whether any loaded skills are no longer relevant to the recent topic.
6. The runtime unloads only the skills explicitly marked for removal.

## Decision Scope

- Focus on unloading stale skills, not loading new ones.
- A skill should be unloaded only if the recent conversation no longer depends on it.
- If the last 10 turns still match the skill domain, keep it loaded.
- The review model should return strict JSON only.

## Why It May Help

- Reduces prompt noise from skills that were useful earlier but are no longer relevant.
- Keeps manual skill loading as the primary workflow, while trimming obvious leftovers.
- Is much cheaper and lower risk than task-aware compaction.

## Learning Angle

- The same periodic review could emit non-executing observations about skill usefulness.
- Over time, these observations could be stored as product feedback for future tuning.
- Example signals: a skill stayed active for many turns without being used, or a skill remained relevant much longer than expected.

## Design Notes

- The review model never produces user-facing output.
- The review should be disabled unless an explicit review model is configured.
- The runtime should apply only unload decisions, not free-form reasoning.
- The review window should be based on recent turns, not the full session.
