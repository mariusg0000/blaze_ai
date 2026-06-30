[DESCRIPTION]
MUST load first before creating, reviewing, modifying, repairing, or debugging any skill, including runnable skills. Use for skill type decisions, skill structure, separating instructions from facts, and turning user corrections or failed workflows into better skill rules.

[BEHAVIOR]
# Skill Manager

## Purpose
Design, review, or improve skill content. A skill improves future behavior by defining when it applies, what workflow to follow, what to try first, what to avoid, and which pitfalls matter.

## Skill Format

Every skill is a folder containing `skill.md`. A skill is exactly one of two exclusive types. Never mix loadable sections with runnable sections in the same skill.

### Type 1 — Loadable skill

Sections: `[DESCRIPTION]` (required) plus `[BEHAVIOR]`, `[DATA]`, or both.

Loaded with `load_skill`. Its `[BEHAVIOR]` and `[DATA]` enter the prompt context and guide the agent's reasoning and decisions.

### Type 2 — Runnable skill

Sections: `[DESCRIPTION]` (required) plus `[SYNTAX]` and valid `[CODE]`.

Executed with `run_skill`. It does not enter the prompt context. It runs as a shell command and returns output. It must not contain `[BEHAVIOR]` or `[DATA]`.

### Exclusive types

Never add `[SYNTAX]` or `[CODE]` to a loadable skill. Never add `[BEHAVIOR]` or `[DATA]` to a runnable skill. If a use case needs both guidance and a runnable command, create two separate skills: one loadable for guidance, one runnable for execution.

## Skill Type Decision

Choose the skill type by purpose, not by user wording.

### Choose loadable when the skill:

- teaches the agent how to work (workflow, strategy, safety)
- provides decision rules or stop conditions
- stores durable facts the agent needs to know
- guides which tools to use and when
- defines first checks, pitfalls, and fallbacks
- requires context-dependent interpretation or conversation
- depends on data from other skills, SSH hosts, APIs, or user input

### Choose runnable when the skill:

- is a non-trivial, reusable shell script worth saving across sessions
- has a stable argument interface you would otherwise retype or get wrong
- produces directly useful output without model interpretation
- runs as one self-contained script with no conversation during execution
- failure can be reported by the script clearly
- invoking it repeatedly via `shell` directly would be tedious or error-prone

### Do not make runnable when:

- the action is a simple one-liner the model can reproduce from memory (use `shell` directly)
- you would run it only once
- the task needs exploration or filesystem search before deciding what to run
- the task needs conversation with the user
- the task requires model judgment to interpret results
- the task modifies files contextually based on current state
- the task requires approvals, sudo, destructive actions, or credential handling
- the script would become a broad program or orchestration layer
- user intent is ambiguous and the correct tooling depends on context

### Rule of thumb

If you can type the command in `shell` right now and it works, do not make a runnable skill for it. A runnable skill earns its place by being non-trivial, reusable, and stable. Otherwise just run it.

### DESCRIPTION

Keep it short and trigger-based. It must state:
- `Load when`: user intent, topic, domain, or task that activates the skill.
- `Use for`: what the skill helps accomplish.

Rules:
- Concrete trigger words the user is likely to say.
- Names relevant tools, systems, or domains.
- Concise. No long instructions.
- No inventories, credentials, logs, or changelogs.

Example:
`Load when the user wants Node-RED, MQTT, Zigbee, or smart-home actions. Use for MQTT workflows, Zigbee2MQTT troubleshooting, SSH access patterns, and safe smart-home command guidance.`

### BEHAVIOR

Use BEHAVIOR for workflow, decision rules, strategy, pitfalls, fallbacks, validation, and safety constraints.

A good BEHAVIOR section answers:
- What should the agent do first?
- What should the agent not try first?
- What assumptions are dangerous?
- What signals indicate success or failure?
- When should the agent stop and ask the user?

### Runnable Skills (v1)

A runnable skill is a standalone executable tool. It is not a loadable skill with extra sections. It must not contain `[BEHAVIOR]` or `[DATA]`.

Sections:
1. `\[DESCRIPTION\]` — required, must appear first. Same rules as loadable skills.
2. `\[SYNTAX\]` — required, one-line compact argument syntax. Describes arguments only. Never repeats the skill name.
3. `\[CODE\]` — required, a fenced code block with language `shell`. No other languages in v1.

If the skill takes no arguments, set `[SYNTAX]` to `""`. Call it with `run_skill(name, "")`.

The skill body runs with env vars: `BLAZE_SKILL_ARGS` (raw string), `BLAZE_SKILL_DIR`, `BLAZE_SKILL_ID`, `BLAZE_SKILL_NAME`.

`[SYNTAX]` is compact, single‑line — the model sees it directly in the prompt's runnable skills section. A runnable skill does not enter the prompt context as active content; it is only listed by name and syntax.

The model uses `run_skill` (not `load_skill`) to execute it. Runnable skills are always visible in the available list; they need not be loaded.

Example:
````
\[SYNTAX\]
<path> [--dry-run]

\[CODE\]
```shell
rsync -av "$BLAZE_SKILL_ARGS"
```
````

Zero-argument example:

````
\[SYNTAX\]
""

\[CODE\]
```shell
df -h
```
````

(The outer example fences use four backticks so the inner `shell` fence stays literal.)

When creating or fixing a skill, load this skill first and use its path rules directly. Do not browse unrelated skills just to rediscover the folder layout.

### DATA

Use DATA for durable facts, reference data, mappings, and preferences. Keep `scope.key=value`, one fact per line.

Keep DATA short, dense, and factual. No headings, prose, narratives, credentials, or step-by-step procedures.

### BEHAVIOR vs DATA
- BEHAVIOR: how to work.
- DATA: what is true.

A loadable skill may contain only BEHAVIOR, only DATA, or both. If a loadable skill needs persistent domain facts, put them in its own DATA section. Do not create a separate skill just for data. Runnable skills do not use BEHAVIOR or DATA.

## Recommended Structure

For most operational skills, structure BEHAVIOR in this order:
1. Purpose
2. Preferred workflow
3. First checks or first strategy
4. Known pitfalls
5. Fallbacks
6. Validation
7. Stop conditions
8. Examples

For data-only skills, use a concise DATA section with `key=value` facts.

Do not force this structure for very small skills. Concise procedural rules are better than many headings.

## Where Skills Live

Skills are stored on disk in one of two scopes:

- **Global** — shared across all projects. Path: `\{GLOBAL_SKILLS_DIR\}/<name>/skill.md`
- **Project** — scoped to the current project. Path: `\{PROJECT_SKILLS_DIR\}/<name>/skill.md`

Both use the same folder layout: `<name>/skill.md`. Never use a flat `.md` file directly under the skills directory — flat files are invalid and will not be discovered.

## Creating or Editing a Skill

### Prerequisites

- An active skill's BEHAVIOR and DATA are already in your context. Do not read the file from disk unless you need to modify it.
- To modify a skill, read the file first, then write the updated version.

### Commands

Create the folder and write `skill.md`:

```
mkdir -p {GLOBAL_SKILLS_DIR}/<name>
```
or
```
mkdir -p {PROJECT_SKILLS_DIR}/<name>
```

Use these paths directly. Do not search the filesystem for skills directories when `{GLOBAL_SKILLS_DIR}` or `{PROJECT_SKILLS_DIR}` already gives the exact target path.

The variables above resolve to real paths automatically. Use `{SKILL_DIR}` inside skill content to reference the skill's own folder (e.g., for bundled scripts).

### Injectable Variables

These variables can be used inside skill content and resolve automatically:

- `\{APP_HOME\}` — BlazeAI app home directory
- `\{WORK_DIR\}` — current working directory
- `\{OS_INFO\}` — human-readable OS description
- `\{SKILL_DIR\}` — the skill's own folder on disk
- `\{GLOBAL_SKILLS_DIR\}` — global skills root directory
- `\{PROJECT_SKILLS_DIR\}` — current project skills root directory

## Choosing Global vs Project

- **Global** when the skill is generic, reusable across projects, about personal tools/preferences, or cross-project knowledge (network, backup scripts, music player, smart home).
- **Project** when the skill is specific to this codebase: build rules, architecture, deploy, project conventions, project-specific paths or names.
- **Unsure?** Ask the user.

### Generalizing for Global

If promoting a project skill to global:
- Remove absolute paths. Use `\{SKILL_DIR\}`, `\{APP_HOME\}`, `\{GLOBAL_SKILLS_DIR\}`, `\{PROJECT_SKILLS_DIR\}`, or generic descriptions.
- Remove project/branch names and project-specific conventions.
- Write examples generically so they apply to any project.
- If after generalizing nothing useful remains, keep it project.

## Restoring Builtin Skills

Builtin skills (`skill-manager`, `customize-me`, `setup_helpers`, `session-retrospective`, `specs-manager`, `telegram_bridge`) are seeded into `\{GLOBAL_SKILLS_DIR\}` on startup. To restore a builtin to its factory version, delete its folder and restart BlazeAI.

[DATA]
skill.format=folder/<name>/skill.md with \[DESCRIPTION\] (required) and at least one of \[BEHAVIOR\] or \[DATA\]
skill.ids=bare name for global skills (default); project/name for project-scoped skills
skill.resolution=bare name resolves to global by default; project/name resolves to project scope exactly
skill.scopes=two runtime scopes: global and project
