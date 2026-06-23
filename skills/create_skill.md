[DESCRIPTION]
Create new custom skills for BlazeAI. Define skill files with the required [DESCRIPTION] and [DETAILS] sections and save them to the custom skills folder.

[DETAILS]
# Create Skill

## Skill Format
- Every skill file is Markdown.
- Every skill must contain exactly two fixed sections:
  - `[DESCRIPTION]`: a short summary shown in the available skills block.
  - `[DETAILS]`: the full content injected when the skill is active.
- Skills without both sections are invalid and reported as errors.

## Storage
- Custom skills are stored in the real on-disk folder `{APP_HOME}/skills/`.
- Always use the injected `{APP_HOME}` variable when creating or referencing custom skill files.
- The skill file name (without extension) is the skill identifier.
- File names must be lowercase, use underscores, and end with `.md`.

## Collision Rules
- You cannot create a skill with the same name as an existing one.
- If a custom skill has the same name as a builtin skill, the custom skill wins.
- The builtin skill with the same name is ignored when a custom one exists.

## How To Create
1. Write the skill content to `{APP_HOME}/skills/<name>.md` using the `shell` tool.
2. Include the `[DESCRIPTION]` and `[DETAILS]` sections.
3. The skill becomes available at the next prompt build.
4. Use `load_skill` with the skill name to activate it.
