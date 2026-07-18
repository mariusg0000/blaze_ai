# Decision Summary: execution policy, global KISS, fast execution, venv Python, script tasks

Date: 2026-07-18 07:13

## Context

Agent overengineering and vague execution guidance. Blaze was instructed to prefer shell over Python but lacked concrete rules for batching, script placement, Python venv usage, and a global anti-overengineering directive. LLMs tend to overengineer and add speculative work.

## Changes Made

- `prompts/sysprompt.md` — added `[BEHAVIOR]` with global KISS rule; replaced scripts/venv environment entries with task-script and venv-mandatory entries; replaced two-line `Execution preference:` with full `[EXECUTION]` section covering batching, sequential dependencies, shell-vs-script criteria, script location, and Python venv policy
- `prompts/sysprompt.agent.md` — identical `[BEHAVIOR]` and `[EXECUTION]` sections added for child agents; scripts/venv environment entries updated; child agent privileged-commands prohibition preserved
- `internal/platform/apphome_readmes/scripts/README.md` — rewritten to describe task-script storage, `write_file` auto-mkdir, and Python-only-through-venv
- `specs/01-product-scope.md` — execution model bullets expanded: fast execution policy, direct-vs-script decision criteria, script language selection, task-script location, Python venv rule
- `specs/17-platform.md` — scripts tree expanded with `scripts/tasks/<task-slug>/`; venv description expanded with user-approval requirement and system-environment exception
- `specs.md` — pre-existing unrelated rewrite (not part of this session's work)

## Decisions And Rationale

- **Global KISS rule** — prevents overengineering across all interactions, not just scripts. Placed in `[BEHAVIOR]` as the first behavioral directive after identity.
- **Concrete script trigger criteria** — replaces vague "for complex tasks" with explicit patterns: loops, repeated operations, structured parsing, multi-step transforms, branching, fragile quoting, or token reduction.
- **No Bash-first bias** — agent chooses between OS-native shell and Python based on simplest, most robust, token-efficient solution, not an artificial preference.
- **Task scripts in `{APP_HOME}/scripts/tasks/<task-slug>/`** — isolates scripts from user projects; grouped by task; `write_file` creates parent dirs automatically, eliminating separate `mkdir` calls.
- **Python venv mandatory** — all Python task scripts and library installs go through `{APP_HOME}/scripts/venv/`. System Python/pip only for initial venv creation (with user approval) or when task explicitly targets system Python.
- **No duplicate KISS in `[EXECUTION]`** — global rule already covers scripts; repetition would consume tokens without adding value.
