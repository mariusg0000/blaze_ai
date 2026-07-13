# TODO: Implement periodic skill review

## WHAT MUST BE DONE
Implement a lightweight background review every configured number of user messages that sends recent turns and active skills to a secondary model and unloads only explicitly identified stale skills.

## WHY IT MUST BE DONE
Skills that remain active after their topic ends add prompt noise and cost, while periodic review could trim obvious leftovers without replacing manual loading.

## HOW IT MUST BE DONE
Track user messages, skip reviews when no skills are active, require an explicitly configured review model, request strict JSON unload decisions, apply unloads only, and keep the review based on recent turns rather than the full session.
