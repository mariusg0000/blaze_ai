[DESCRIPTION]
Load when creating, reviewing, or modifying a skill. Use for designing clear procedural skill content, separating skills from memory banks, and converting user corrections or failed workflows into better skill rules.

[DETAILS]
# Skill Manager

## Purpose
Use this skill to design, review, or improve the content of a skill.

A skill should improve future agent behavior by defining:
- when it applies;
- what workflow to follow;
- what to try first;
- what to avoid;
- what facts must come from memory;
- what pitfalls or wrong assumptions are known.

This skill is about the content and structure of skills, not about low-level file editing strategy.

## Required Skill Format
Every skill must contain exactly two required sections:
- `[DESCRIPTION]`
- `[DETAILS]`

`[DESCRIPTION]` must appear before `[DETAILS]`.

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

## What A Skill Should Contain
A skill should contain procedural guidance.

Good skill content includes:
- preferred workflow;
- decision rules;
- first-choice strategy;
- known pitfalls;
- fallback logic;
- validation steps;
- stop conditions;
- safety constraints;
- references to related memories;
- concise examples that clarify behavior.

A skill should answer:
- What should the agent do first?
- What should the agent not try first?
- What assumptions are dangerous?
- What signals indicate success or failure?
- When should the agent stop and ask the user?
- Which related memory bank contains domain facts?

## What A Skill Should Not Contain
Do not put long-lived environment data in a skill.

Avoid storing:
- credentials;
- API keys;
- private IP inventories;
- server catalogs;
- device inventories;
- sensor mappings;
- customer facts;
- project facts;
- paths that are environment facts rather than procedural examples;
- raw command output;
- changelogs;
- long notes.

Use a related memory bank for persistent facts, inventories, mappings, service names, IPs, paths, configuration, and user-specific data.

## Skill vs Memory
Use this distinction:

- Skill: how to work.
- Memory bank: what is true.
- Tool: what can be executed.
- Policy: what is allowed or forbidden.

Examples:
- `node_server` skill: how to query MQTT safely and troubleshoot Zigbee2MQTT.
- `node-server-data` memory: server IP, MQTT base topic, friendly names, service paths.
- `music_player` skill: how to control playback and avoid bad command patterns.
- `music-player-data` memory: playlists, directories, preferred player settings.

## Related Memories And Skills
If a skill needs persistent domain facts, mention the related memory bank by name.

A skill may say:
- load memory `<memory-name>` when this skill needs environment facts;
- use the memory for mappings, paths, service names, IPs, or inventories;
- do not guess domain facts that should come from memory.

Do not duplicate memory content inside the skill.

If a memory has a related skill, the skill should define the workflow for using that memory safely and efficiently.

## Skill Structure
For most operational skills, prefer this structure:

1. Purpose
2. Related memories
3. Preferred workflow
4. First checks or first strategy
5. Known pitfalls
6. Fallbacks
7. Validation
8. Stop conditions
9. Examples

Do not force this structure if the skill is very small. For small skills, concise procedural rules are better than many headings.

## Skill Update After Errors
When updating a skill after a failed attempt, timeout, inefficient workflow, or user correction, do not merely append a note.

First identify:
- the wrong assumption;
- the inefficient or incorrect path;
- the better future strategy;
- what should be tried first next time;
- what should be avoided next time;
- whether the lesson belongs in the skill or in memory.

Then update the skill content so the same mistake is less likely to recur.

Good updates:
- replace obsolete advice;
- move the better strategy into the preferred workflow;
- add a known pitfall if it prevents repeated errors;
- add a stop condition if continuing would waste time;
- add a fallback only when it is genuinely useful.

Bad updates:
- append a vague note;
- keep old misleading advice and add exceptions around it;
- turn the skill into a changelog;
- copy persistent environment facts into the skill;
- add many examples without changing the decision rule.

## Handling User Corrections
Treat user corrections as high-priority evidence about the domain or workflow.

When the user says an approach was wrong, wasteful, or based on a bad assumption:
- identify the corrected rule;
- encode the corrected rule in the skill;
- remove or rewrite the old rule that caused the mistake;
- decide whether any persistent fact should be moved to memory.

The goal is not to document that the user corrected the agent. The goal is to improve the future workflow.

## Procedural Rule Quality
Skill rules should be:
- specific enough to guide action;
- general enough to apply next time;
- phrased as behavior, not as history;
- ordered by preferred strategy first;
- clear about pitfalls and fallbacks;
- free of contradictions.

Prefer:
`Use Zigbee2MQTT friendly-name topics first when reading sensor state.`

Avoid:
`Last time, using the raw address did not work.`

Prefer:
`Do not treat battery sensors as on-demand devices; read retained MQTT state when available.`

Avoid:
`Maybe increase the timeout if it fails.`
