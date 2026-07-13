# TODO: Implement memory banks

## WHAT MUST BE DONE
Implement on-demand memory banks stored under `app_home/memory-banks/<name>/memory-bank.md`, with `[DESCRIPTION]` and `[DETAILS]` sections, discovery alongside skills, and load/unload support.

## WHY IT MUST BE DONE
Relevant contextual knowledge such as network, deployment, or client-specific information should enter prompts only when needed instead of remaining permanently active.

## HOW IT MUST BE DONE
Fully assimilated by the skills system: loadable skills with `[DATA]` sections replace memory banks entirely. Skills are discovered from builtin, global, and project sources, loaded/unloaded via `ActiveList`, and their `[DATA]` content injects into the prompt when active. Verified by existing skills: `my-network`, `personal-info`, `todo-memory` serve as real memory banks. No separate memory subsystem needed.
