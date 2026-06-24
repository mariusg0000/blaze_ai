# Session Decision Summary: memory-skill-manager

Date: 2026-06-24 08:41
Base commit: 8fdb247

## Context
The session evolved around two runtime knowledge systems: skills and memory banks. The user asked to rename and tighten the builtin `memory` skill into `memory-manager`, then to rename `create_skill` into `skill-manager`, and to keep memory-bank handling strict and minimal. A startup panic also exposed duplicate tool registration for `load_skill`.

## Changes Made
- Renamed the builtin memory skill to `skills/memory-manager.md` and reduced its instructions to the strict activation and formatting rules requested.
- Renamed the builtin create/update skill to `skills/skill-manager.md` and changed its description to imperative wording.
- Kept memory-bank runtime support under `internal/memories/` with `load_memory` and `unload_memory` tools and prompt injection for available versus active memories.
- Removed duplicate `load_skill` / `unload_skill` registration from `main.go` so startup no longer panics.
- Updated prompt tests, skills tests, tool tests, runtime wiring, and spec text to match the renamed builtin skills and memory-bank model.

## Decisions And Rationale
- The builtin memory skill was narrowed because its job is only to explain when memory banks must be created or modified, not to teach tool policy or unrelated runtime behavior.
- The create-skill helper was renamed to `skill-manager` to better reflect that it is the mandatory entrypoint for creating or modifying skills.
- Tool registration stayed in `runtime.NewAgent()` only, because registering the same tool twice in `main.go` and runtime caused a hard duplicate-name panic.
- Memory-bank behavior remained separate from skills at the runtime layer, but the user-facing instructions were trimmed to the requested scope.

## Implementation Approach
- Moved the builtin skill files to the new names and rewrote their `[DESCRIPTION]` and `[DETAILS]` blocks.
- Updated tests and specs to use the new builtin names and expected runtime text.
- Removed the second registration site for `load_skill` and `unload_skill` from the application entrypoint.
- Validated the result with the full Go test suite.

## Alternatives Considered
- Keeping `create_skill` would have preserved older terminology, but it did not match the requested naming update.
- Leaving memory-manager instructions broad would have mixed skill guidance with runtime policy and made the builtin harder to reason about.

## Files Included
- `skills/memory-manager.md`: strict memory-bank creation/modification guidance.
- `skills/skill-manager.md`: renamed builtin for creating and modifying skills.
- `main.go`: removed duplicate skill-tool registration.
- `internal/memories/*`, `internal/prompt/*`, `internal/runtime/*`, `internal/skills/*`, `internal/tools/*`: runtime, tests, and tool wiring updates for the renamed builtins and memory-bank flow.
- `specs.md`, `specs/02-core-runtime.md`: synchronized spec text with the renamed builtin skills.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
