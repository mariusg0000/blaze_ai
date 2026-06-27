# Session Decision Summary: sharper-runnable-criteria

Date: 2026-06-27 12:13
Base commit: da0087f

## Context
The `Choose runnable when` criteria in skill-manager were too permissive. They would justify creating a runnable skill for any simple shell command (`df -h`, `ls`, `free -m`) since "does one clear, repeatable, deterministic action" applies to almost any command. The user pointed out that `df -h` qualifies under the old criteria but clearly does not warrant a skill — just use `shell` directly.

## Changes Made
- Replaced `Choose runnable when` criteria with tighter rules requiring non-triviality, reusability across sessions, stable argument interface, and practical tedium/error-proneness as the threshold.
- Added two new items to `Do not make runnable when`: one-liners reproducible from memory, and one-time runs.
- Added `Rule of thumb` section: "If you can type the command in `shell` right now and it works, do not make a runnable skill for it."
- Only repo version was modified; app-home copy was not touched (user will handle manually).

## Decisions And Rationale
- The old criteria were structurally correct but insufficiently discriminative. A command satisfying all "choose runnable" criteria can still be a bad fit because it's trivial.
- The real threshold is not determinism — it's whether the script is non-trivial enough to merit saving as a named reusable unit with a stable interface.
- The rule of thumb gives the LLM a quick heuristic instead of weighing 6 criteria.

## Implementation Approach
- Two targeted edits in `skills/skill-manager.md`: replace the 6-item bullet list under `Choose runnable when`, replace the 7-item list under `Do not make runnable when` and add the rule of thumb after it.
- Validated with `go test -count=1 ./internal/prompt ./internal/skills`.

## Files Included
- `skills/skill-manager.md`: tightened runnable criteria

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
