[DESCRIPTION]
Load when creating, reviewing, or modifying a skill. Use for skill file structure, skill-memory separation, related memory guidance, and optimizing skills after errors, inefficient tool sequences, or user corrections.

[DETAILS]

# Skill Manager

## Purpose

Use this skill before creating, reviewing, or modifying any skill.

This skill ensures that:

* skill files keep the required format;
* procedural instructions stay inside skills;
* persistent facts and inventories go into memory banks;
* related skills and memories are declared when useful;
* skill updates improve future behavior instead of merely appending notes.

## Skill Format

* Every skill main file is Markdown and must be named `skill.md`.
* Every skill must contain exactly two fixed top-level sections on their own lines:

  * `[DESCRIPTION]`
  * `[DETAILS]`
* `[DESCRIPTION]` must appear before `[DETAILS]`.
* Skills without both sections are invalid and must be reported as errors.
* Do not add extra top-level bracket sections unless the runtime explicitly supports them.

## Description Rules

The `[DESCRIPTION]` section must contain a short summary shown in Available Skills.

State two things:

* `Load when`: the user intent, topic, command, domain, or task that should activate the skill.
* `Use for`: what the skill helps accomplish.

Good descriptions:

* mention concrete keywords, tools, systems, and task types the user is likely to say;
* are concise, ideally one sentence;
* avoid vague descriptions like "useful for many things";
* do not include long instructions, inventories, credentials, or changelog content.

Example:
`Load when the user wants Node-RED, MQTT, Zigbee, or smart-home actions. Use for SSH access patterns, MQTT workflows, troubleshooting strategy, and safe command examples.`

## What Belongs In A Skill

A skill should contain procedural and behavioral guidance.

Put these in a skill:

* workflows;
* decision rules;
* preferred strategies;
* known pitfalls;
* safe command patterns;
* validation steps;
* troubleshooting order;
* fallback logic;
* rules for using related memory banks;
* small examples that clarify procedure.

A skill must define:

* when it should be loaded;
* what it helps accomplish;
* how the agent should behave in that domain;
* what to try first;
* what to avoid;
* when to stop or ask the user.

## What Does Not Belong In A Skill

Do not store persistent data in a skill when it belongs in a memory bank.

Do not put these in a skill unless they are tiny and inseparable from the procedure:

* credentials;
* API keys;
* private IP inventories;
* server lists;
* service catalogs;
* customer facts;
* project facts;
* sensor mappings;
* device inventories;
* environment-specific configuration;
* long-lived notes;
* changelogs;
* raw command output dumps.

Use a related memory bank for persistent facts, inventories, mappings, configuration, preferences, or domain data.

## Skill vs Memory Rule

Use this distinction:

* Skill: how to work.
* Memory bank: what is true in a specific environment.
* Tool: what can be executed.
* Policy: what must not be done.

If a skill needs facts to work correctly, reference or create a related memory bank instead of embedding those facts directly.

Examples:

* `node_server` skill: MQTT workflow, SSH patterns, troubleshooting order.
* `node-server-data` memory: IP address, MQTT base topic, friendly names, service paths.
* `network_troubleshooting` skill: diagnostic procedure.
* `my-network` memory: servers, roles, IPs, VLANs, DNS records.

## Related Memory Guidance

If a skill depends on reusable facts, declare or mention the related memory bank.

When creating or updating a skill, consider whether it should include guidance such as:

* load related memory `<memory-name>` when using this skill;
* use the memory for IPs, service names, paths, mappings, or environment facts;
* do not duplicate memory content inside the skill.

If the runtime supports metadata or descriptions for related items, keep relationships explicit:

* skill -> related memories;
* memory -> related skills.

Do not load or recommend related items blindly. They must be relevant to the current task.

## Required Content For Useful Skills

A useful skill should usually include:

* When to use it.
* Preferred workflow.
* First commands or first checks.
* Known pitfalls.
* Fallbacks.
* Safety constraints.
* Related memories, if any.
* Examples, if they reduce ambiguity.

For troubleshooting skills, prefer this structure:

1. Fast path: the most reliable first attempt.
2. Known pitfalls: mistakes or assumptions to avoid.
3. Validation: how to confirm the result.
4. Fallbacks: alternative checks only if the fast path fails.
5. Stop conditions: when not to continue without user input.

## Skill Update Optimization

When updating an existing skill after an error, timeout, inefficient attempt, user correction, or newly discovered fact, do not merely append the new fact.

First analyze the previous workflow:

* What was the user trying to accomplish?
* Which tool calls or commands were useful?
* Which tool calls or commands were unnecessary, redundant, misleading, or based on a wrong assumption?
* What assumption caused the failure or inefficiency?
* What should be tried first next time?
* What should be avoided next time?
* Does the new information belong in the skill or in a related memory bank?

Then update the skill to improve future behavior.

A good update should usually add or revise:

* the preferred strategy;
* a decision rule;
* a known pitfall;
* a first-choice command or workflow;
* a fallback path;
* a stop condition;
* a "do not try this first" warning when a previous attempt was wasteful.

Do not preserve obsolete advice if new evidence contradicts it. Remove or rewrite misleading instructions instead of adding exceptions around them.

If the user points out that a previous command was useless or based on a bad assumption, treat that as high-priority feedback and encode the corrected reasoning into the skill.

## Updating Existing Skills

Before editing an existing skill:

1. Read the current `skill.md`.
2. Identify the section that needs modification.
3. Decide whether the change belongs in the skill or a memory bank.
4. Preserve the required `[DESCRIPTION]` and `[DETAILS]` structure.
5. Prefer rewriting the relevant block over appending scattered notes.
6. Remove stale or misleading instructions.
7. Keep the skill concise and procedural.
8. Verify the final file after editing.

When updating because of a failed or inefficient interaction, rewrite the workflow so the same mistake is less likely to happen again.

Do not create a changelog inside the skill. The skill should describe the current best behavior, not the history of edits.

## Editing Strategy

Prefer targeted edits:

* replace the smallest coherent block that needs changing;
* avoid rewriting the whole file unless structure is poor;
* keep headings stable when possible;
* preserve useful examples;
* remove obsolete examples.

Use broad rewrites only when:

* the skill mixes data and procedure badly;
* the structure is confusing;
* repeated patches would leave contradictions;
* the skill has grown into a notes file.

After editing, inspect the result to ensure:

* the two required sections still exist;
* the description is concise;
* procedural guidance is clear;
* facts that belong in memory were not embedded;
* obsolete advice was removed;
* examples match the current recommended strategy.

## Command And Tool Guidance For Skill Edits

Use the smallest safe set of commands or tool calls needed.

Prefer:

* read the target file first;
* make targeted edits;
* verify the edited content.

Avoid:

* editing without reading the current file;
* blind append-only updates;
* duplicating existing instructions;
* leaving contradictory old and new advice;
* storing secrets or long-lived inventories in the skill.

For simple edits, use shell-native tools.
For complex structured rewrites, use an OS-native script if practical.
Use Python only when shell is not a good fit, and only through the BlazeAI-managed virtual environment.

## Storage

Custom skills are stored in the real on-disk folder `{APP_HOME}/skills/`.

Every custom skill has its own folder:
`{APP_HOME}/skills/<skill-name>/`

The main skill file must be:
`{APP_HOME}/skills/<skill-name>/skill.md`

The skill folder name is the skill identifier.

Skill folder names must:

* be lowercase;
* use underscores;
* avoid spaces;
* avoid ambiguous names.

If the skill needs scripts, data, templates, or other local resources, create them inside that skill folder.

## Injectable Variables

The following variables are injected automatically where supported:

* `\{APP_HOME\}`: the resolved BlazeAI app home path.
* `\{WORK_DIR\}`: the current working directory.
* `\{OS_INFO\}`: the detected operating system description.
* `\{SKILL_DIR\}`: the current custom skill folder when a skill description or details block is injected.

Use `\{SKILL_DIR\}` inside custom skill content when referencing local resources such as scripts or data files.

Example:
`\{SKILL_DIR\}/scripts/run.py`

Do not hardcode the absolute skill folder path inside reusable skill content when `\{SKILL_DIR\}` is available.

## Execution Preferences

* Prefer shell-native scripts first.
* Prefer `bash` or shell scripts before Python when either option is reasonable.
* Use Python only when shell is not a good fit.
* If Python is necessary, run it only through the BlazeAI-managed virtual environment at `{APP_HOME}/scripts/venv/`.
* Never rely on system Python, `python`, `python3`, or global `pip` inside a skill.

## Collision Rules

* Do not create a skill with the same name as an existing one.
* When the user asks to change an existing skill, update that skill instead of creating a duplicate.
* If a requested new skill overlaps strongly with an existing skill, prefer updating the existing skill unless the user explicitly wants a separate one.

## How To Create A Skill

1. Choose a concise lowercase underscore skill name.
2. Create the folder `{APP_HOME}/skills/<skill-name>/`.
3. Write the main skill content to `{APP_HOME}/skills/<skill-name>/skill.md`.
4. Include both required top-level section headers:

   * `[DESCRIPTION]`
   * `[DETAILS]`
5. Put procedural guidance in the skill.
6. Put persistent facts in a related memory bank.
7. If needed, add local resources under that same folder, for example:

   * `{APP_HOME}/skills/<skill-name>/scripts/`
   * `{APP_HOME}/skills/<skill-name>/data/`
8. Reference local resources from the skill content with `{SKILL_DIR}`.
9. The skill becomes available at the next prompt build.
10. Use `load_skill` with the skill name to activate it.

## How To Review A Skill

When reviewing a skill, check:

* required format;
* description clarity;
* load trigger quality;
* procedural usefulness;
* separation from memory-bank data;
* obsolete or contradictory advice;
* unnecessary verbosity;
* missing pitfalls or fallback logic;
* related memory guidance;
* command safety.

Report concrete changes, not vague quality comments.

## How To Update A Skill After A User Correction

When the user says that a previous approach was wrong, wasteful, misleading, or incomplete:

1. Treat the correction as authoritative unless it conflicts with direct evidence.
2. Identify the wrong assumption.
3. Identify the command or workflow that should not be repeated.
4. Identify the better first strategy.
5. Update the skill to encode the new strategy.
6. Remove or rewrite the old misleading instruction.
7. Verify the final skill content.

The update should make the next run more efficient, safer, and less speculative.

## Example: Good Update Pattern

Bad update:

* Add a note saying "Sometimes this may not work."

Good update:

* Explain why the old method fails.
* Move the better method into the preferred workflow.
* Add the old method under fallback or known pitfall if still relevant.
* Remove advice that caused wasted tool calls.

Example:

* Wrong assumption: a battery-powered sensor can be queried on demand.
* Better rule: battery sensors publish periodically and should be read from retained MQTT state when available.
* Preferred strategy: use Zigbee2MQTT friendly-name topics first.
* Pitfall: do not use raw Zigbee IEEE addresses as MQTT topics unless config proves that topic exists.
* Misleading advice to remove: increasing timeout as a primary solution.

