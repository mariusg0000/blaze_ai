# TODO: Implement project-scoped skills and memories

## WHAT MUST BE DONE
Add discovery and runtime handling for project-scoped skills under `{workdir}/.blazeai/skills/` and memories under `{workdir}/.blazeai/memories/`, including project labeling, collision precedence, and automatic unload when the project changes.

## WHY IT MUST BE DONE
Project-specific conventions and actions should stay near the code, be shareable with the project, and avoid polluting the global skill and memory namespaces.

## HOW IT MUST BE DONE
Integrate project discovery into every prompt build, mark project items distinctly, make project versions win over global collisions, clear stale project context on workdir or mode changes, and ensure `.blazeai/` handling is gitignore-aware for secrets and keys.
