# IDEA: Local and global skill scope

## WHAT MUST BE DONE
Explore a skill model in which each skill is explicitly marked as either local or global. A global skill, such as an ideas or personal-tracker skill, must use shared data regardless of which BlazeAI device receives the input.

## WHY IT MUST BE DONE
The NAS and PC instances currently accumulate different personal data and skills. Only selected skills need universal data, so the solution should avoid making the whole application distributed.

## HOW IT MUST BE DONE
Evaluate adding local/global scope to skill metadata and exposing the choice through Skill Manager. Keep this as an architectural idea until the storage and federation model are selected.
