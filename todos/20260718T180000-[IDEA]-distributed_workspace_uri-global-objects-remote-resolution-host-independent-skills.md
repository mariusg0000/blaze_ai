# IDEA: Distributed workspace objects

## WHAT MUST BE DONE
Explore representing shared skill data as explicit distributed workspace objects, for example `global://personal-nas/ideas`, resolved by the local device's federation profile.

## WHY IT MUST BE DONE
Skills should describe the workspace they use without embedding whether the implementation is SSH, SSHFS, or another transport. Explicit workspace identity also separates global data from local application state.

## HOW IT MUST BE DONE
Evaluate a host-independent workspace reference and its resolution rules. Keep the first version focused on selected global skill data; do not expand it into general remote tool routing or a complete coordinator/worker architecture without a separate decision.
