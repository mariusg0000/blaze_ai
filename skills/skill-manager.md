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

[DATA]
skill.format.behavior=rules for how to work
skill.format.data=persistent facts in key=value format
skill.scopes=two runtime scopes: global (app_home/skills/) and project (workdir/.blazeai/skills/)
skill.ids=global/name and project/name for canonical IDs; builtin skills are templates seeded to global at startup
skill.seeding=embedded builtin skills are copied to app_home/skills/ on first start; delete folder+restart to restore originals
skill.global_layout=app_home/skills/<name>/skill.md
skill.project_layout=workdir/.blazeai/skills/<name>/skill.md
skill.resolution=unqualified name resolves if unique across scopes; ambiguous names error listing candidates; scoped names (global/x, project/x) resolve exactly
