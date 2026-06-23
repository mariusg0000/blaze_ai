[DESCRIPTION]
Load when the user wants a new skill or a skill update. Use for skill structure, `skill.md` format, folder layout, and `{SKILL_DIR}` resource paths.

[DETAILS]
# Create Skill

## Skill Format
- Every skill main file is Markdown and must be named `skill.md`.
- Every skill must contain exactly two fixed sections on their own lines:
  - `[DESCRIPTION]` — the short summary shown in Available Skills.
  - `[DETAILS]` — the full instruction block injected when the skill is active.
- Skills without both sections are invalid and reported as errors.

## Description Rules
- The first section header after `[DESCRIPTION]` must be a single line — the short description shown in Available Skills.
- State two things every time:
  - **Load when**: the user intent, topic, or task that signals this skill should be activated.
  - **Use for**: what the skill provides and what it helps accomplish.
- Mention concrete keywords, tools, and task types the user is likely to say.
- Keep it concise: one sentence if possible, at most two.
- Never include inactive section headers such as `[TRIGGER]` — they are ignored by the runtime.
- Good: `Load when the user asks for web search or DuckDuckGo news. Use for ddgs text, news, image, and video searches without an API key.`
- Bad:  `Free web search via DuckDuckGo.` (missing load cue and intent patterns)
- Bad:  `Configure BlazeAI provider endpoints, API keys, favorite models, and role assignments in config.json.` (missing load cue)

## Storage
- Custom skills are stored in the real on-disk folder `{APP_HOME}/skills/`.
- BlazeAI injects the app home path automatically. Use the resolved path shown below, not a literal placeholder.
- Every custom skill has its own folder: `{APP_HOME}/skills/<skill-name>/`.
- The main skill file must be `{APP_HOME}/skills/<skill-name>/skill.md`.
- The skill folder name is the skill identifier.
- Skill folder names must be lowercase and use underscores.
- If the skill needs scripts, data, templates, or other local resources, create them inside that skill folder.

## Injectable Variables
- The following variables are injected automatically where supported:
  - `{APP_HOME}`: the resolved BlazeAI app home path.
  - `{WORK_DIR}`: the current working directory.
  - `{OS_INFO}`: the detected operating system description.
  - `{SKILL_DIR}`: the current custom skill folder when a skill description or details block is injected.
- Use `{SKILL_DIR}` inside custom skill content when referencing local resources such as scripts or data files.
- Example: `{SKILL_DIR}/scripts/run.py`

## Execution Preferences
- Prefer shell-native scripts first.
- Prefer `bash`/shell scripts before Python when either option is reasonable.
- Use Python only when shell is not a good fit.
- If Python is necessary, run it only through the BlazeAI-managed virtual environment at `{APP_HOME}/scripts/venv/`.
- Never rely on system Python or a global `pip` inside a skill.

## Collision Rules
- You cannot create a skill with the same name as an existing one.
- If a custom skill has the same name as a builtin skill, the custom skill wins.
- The builtin skill with the same name is ignored when a custom one exists.

## How To Create
1. Create the folder `{APP_HOME}/skills/<skill-name>/`.
2. Write the main skill content to `{APP_HOME}/skills/<skill-name>/skill.md` using the `shell` tool.
3. Include both required top-level section headers: `[DESCRIPTION]` then `[DETAILS]`.
4. If needed, add local resources under that same folder, for example `{APP_HOME}/skills/<skill-name>/scripts/` or `{APP_HOME}/skills/<skill-name>/data/`.
5. Reference local resources from the skill content with `{SKILL_DIR}`.
6. The skill becomes available at the next prompt build.
7. Use `load_skill` with the skill name to activate it.
