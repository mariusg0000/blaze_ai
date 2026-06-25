[DESCRIPTION]
Load when creating, reviewing, or modifying a skill. Use for designing clear procedural skill content, authoring data sections, separating behavior from data, and converting user corrections or failed workflows into better skill rules.

[BEHAVIOR]
# Skill Manager

## Purpose
Design, review, or improve skill content. A skill improves future agent behavior by defining when it applies, what workflow to follow, what to try first, what to avoid, and what pitfalls are known.

## Skill Format

Every skill is a folder containing a `skill.md` file with these sections:

1. `[DESCRIPTION]` — required, must appear first.
2. `[BEHAVIOR]` — optional, procedural guidance.
3. `[DATA]` — optional, persistent facts.

At least one of `[BEHAVIOR]` or `[DATA]` must be present.

### DESCRIPTION

Short, triggers-based. Must state:
- `Load when`: user intent, topic, domain, or task that activates the skill.
- `Use for`: what the skill helps accomplish.

Good description rules:
- Concrete trigger words the user is likely to say.
- Names relevant tools, systems, or domains.
- Concise — no long instructions.
- No inventories, credentials, logs, or changelogs.

Example:
`Load when the user wants Node-RED, MQTT, Zigbee, or smart-home actions. Use for MQTT workflows, Zigbee2MQTT troubleshooting, SSH access patterns, and safe smart-home command guidance.`

### BEHAVIOR

Procedural guidance: workflow, decision rules, strategy, pitfalls, fallbacks, validation, safety constraints.

A good Behavior section answers:
- What should the agent do first?
- What should the agent not try first?
- What assumptions are dangerous?
- What signals indicate success or failure?
- When should the agent stop and ask the user?

### DATA

Durable facts, reference data, mappings, preferences. Compact `scope.key=value` format, one fact per line.

Good data: short, dense, factual. No headings, prose, or narratives. No credentials. No step-by-step procedures (those go in BEHAVIOR).

### BEHAVIOR vs DATA
- BEHAVIOR: how to work.
- DATA: what is true.

A skill may contain only BEHAVIOR, only DATA, or both. If a skill needs persistent domain facts, define them in its own DATA section — do not create a separate skill just for data.

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

Do not force this structure for very small skills — concise procedural rules are better than many headings.

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
- Remove absolute paths → use `\{SKILL_DIR\}`, `\{APP_HOME\}`, `\{GLOBAL_SKILLS_DIR\}`, `\{PROJECT_SKILLS_DIR\}`, or generic descriptions.
- Remove project/branch names and project-specific conventions.
- Write examples generically so they apply to any project.
- If after generalizing nothing useful remains, keep it project.

## Restoring Builtin Skills

Builtin skills (skill-manager, customize_me, setup_helpers) are seeded into `\{GLOBAL_SKILLS_DIR\}` on startup. To restore a builtin to its factory version, delete its folder and restart BlazeAI.

[DATA]
skill.format=folder/<name>/skill.md with [DESCRIPTION] (required) and at least one of [BEHAVIOR] or [DATA]
skill.ids=bare name for global skills (default); project/name for project-scoped skills
skill.resolution=bare name resolves to global by default; project/name resolves to project scope exactly
skill.scopes=two runtime scopes: global and project
skill.seeding=embedded builtin skills are copied to global on first start; delete folder+restart to restore originals