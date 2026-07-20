# IDEA: Federation-aware Skill Manager

## WHAT MUST BE DONE
Explore extending Skill Manager so it asks whether a skill is local or global when creating or installing one, and requests a federation profile for global skills.

## WHY IT MUST BE DONE
Skill scope should be established deliberately rather than inferred from the current machine. The manager is the natural place to enforce the required discipline and expose configuration problems early.

## HOW IT MUST BE DONE
Evaluate validation for global skill metadata, profile availability, remote reachability, and installation on another device. A separate management skill may guide the workflow, but storage guarantees should remain in application configuration or tools rather than relying only on prompt instructions.
