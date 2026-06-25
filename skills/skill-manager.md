[DESCRIPTION]
Load when creating, reviewing, or modifying a skill. Use for designing clear procedural skill content, authoring data sections, separating behavior from data, and converting user corrections or failed workflows into better skill rules.

[BEHAVIOR]
# Skill Manager

## Purpose
Use this skill to design, review, or improve the content of a skill.

A skill should improve future agent behavior by defining:
- when it applies;
- what workflow to follow;
- what to try first;
- what to avoid;
- what facts must be defined in DATA;
- what pitfalls or wrong assumptions are known.

## Required Skill Format
Every skill must contain `[DESCRIPTION]` and at least one of `[BEHAVIOR]` or `[DATA]`.

`[DESCRIPTION]` must appear first.

The description must be short and must state:
- `Load when`: the user intent, topic, domain, command, or task that activates the skill.
- `Use for`: what the skill helps accomplish.

## Good Description Rules
A good skill description:
- contains concrete trigger words the user is likely to say;
- names important tools, systems, or domains when relevant;
- says what the skill is for;
- is concise;
- does not contain long instructions;
- does not contain inventories, credentials, logs, or changelogs.

Example:
`Load when the user wants Node-RED, MQTT, Zigbee, or smart-home actions. Use for MQTT workflows, Zigbee2MQTT troubleshooting, SSH access patterns, and safe smart-home command guidance.`

## Skill Sections

### BEHAVIOR
Procedural guidance: workflow, decision rules, strategy, pitfalls, fallbacks, validation, safety constraints.

Good behavior content includes:
- preferred workflow;
- decision rules;
- first-choice strategy;
- known pitfalls;
- fallback logic;
- validation steps;
- stop conditions;
- safety constraints;
- references to DATA sections when domain facts are needed;
- concise examples that clarify behavior.

A good Behavior section answers:
- What should the agent do first?
- What should the agent not try first?
- What assumptions are dangerous?
- What signals indicate success or failure?
- When should the agent stop and ask the user?

### DATA
Durable facts, reference data, mappings, preferences, identifiers. Compact key=value format.

Good data content:
- one fact per line using `scope.key=value`;
- short, dense, factual lines;
- avoids headings, prose, narratives, decorative Markdown;
- avoids repeating the same fact under different keys;
- dates only when time-sensitive (deadlines, expirations, changing status);
- no dates for stable identity facts, preferences, or static project facts.

Forbidden in DATA:
- step-by-step procedures, workflows, or behavior rules (use BEHAVIOR);
- credentials or API keys;
- transient reasoning or verbose narratives;
- duplicate facts under different keys.

## Skill vs Data
Use this distinction:
- BEHAVIOR: how to work.
- DATA: what is true.

A skill may contain only BEHAVIOR, only DATA, or both. If a skill needs persistent domain facts, define them in its DATA section rather than creating a separate skill.

## Skill Structure
For most operational skills, prefer this structure in BEHAVIOR:
1. Purpose
2. Preferred workflow
3. First checks or first strategy
4. Known pitfalls
5. Fallbacks
6. Validation
7. Stop conditions
8. Examples

For data-only skills, use a concise DATA section with key=value facts.

Do not force this structure if the skill is very small. For small skills, concise procedural rules are better than many headings.

## Where to Create Skills

Skills live on disk in one of two scopes:

- **Global** — `\{GLOBAL_SKILLS_DIR\}/<name>/skill.md`. Shared across all projects.
- **Project** — `\{PROJECT_SKILLS_DIR\}/<name>/skill.md`. Scoped to the current project.

### Choosing Global vs Project

- **Global** when the skill is: generic, reusable across projects, about personal tools/preferences, cross-project knowledge (network, backup scripts, music player, smart home).
- **Project** when the skill is: specific to this codebase (build rules, architecture, deploy, project conventions, project-specific paths or names).
- **Unsure?** Ask the user.

### Generalizing for Global

If promoting a project skill to global:
- Remove absolute paths → use `\{SKILL_DIR\}`, `\{APP_HOME\}`, `\{GLOBAL_SKILLS_DIR\}`, `\{PROJECT_SKILLS_DIR\}`, or generic descriptions.
- Remove project/branch names and project-specific conventions.
- Write examples generically so they apply to any project.
- If after generalizing nothing useful remains, keep it project.

### Creating the Skill File

Use the shell tool with the real paths (these variables resolve automatically):

```
mkdir -p {GLOBAL_SKILLS_DIR}/<name>
```
or
```
mkdir -p {PROJECT_SKILLS_DIR}/<name>
```

Then write `skill.md` inside that folder with `[DESCRIPTION]` and at least one of `[BEHAVIOR]` or `[DATA]`.

### Restoring Builtin Skills

Builtin skills (skill-manager, customize_me, setup_helpers) are seeded into \{GLOBAL_SKILLS_DIR\} on startup. To restore a builtin to its factory version, delete its folder and restart BlazeAI.

[DATA]
skill.format.behavior=rules for how to work
skill.format.data=persistent facts in key=value format
skill.ids=bare name for global skills (default scope); project/name for project-scoped skills
skill.resolution=bare name resolves to global by default; project/name resolves to project scope exactly
skill.scopes=two runtime scopes: global (app_home/skills/) and project (app_home/projects/<project>/skills/)
skill.global_layout=\{APP_HOME\}/skills/<name>/skill.md
skill.project_layout=\{APP_HOME\}/projects/<project>/skills/<name>/skill.md
skill.variable.app_home=\{APP_HOME\} — absolute path to the BlazeAI app home directory (e.g., /home/user/blazeai)
skill.variable.work_dir=\{WORK_DIR\} — current working directory (project root)
skill.variable.os_info=\{OS_INFO\} — human-readable OS description (e.g., "linux (Linux 6.8.0)" )
skill.variable.skill_dir=\{SKILL_DIR\} — the folder of the currently active skill on disk (e.g., /home/user/blazeai/skills/music_player). Resolves to NULL if the skill is embedded. Use for scripts or assets bundled with the skill.
skill.variable.global_skills_dir=\{GLOBAL_SKILLS_DIR\} — root of the global skills directory (e.g., /home/user/blazeai/skills). Use when creating a new global skill: mkdir -p {GLOBAL_SKILLS_DIR}/<name>
skill.variable.project_skills_dir=\{PROJECT_SKILLS_DIR\} — root of the current project skills directory (e.g., /home/user/blazeai/projects/<p>/skills). Use when creating a new project skill: mkdir -p {PROJECT_SKILLS_DIR}/<name>
