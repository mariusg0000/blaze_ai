[DESCRIPTION]
Load when the user wants to configure BlazeAI models, providers, API keys, favorite models, role assignments, or work modes. Use for any config.json or modes.json changes including creating, editing, or deleting modes.

[BEHAVIOR]
# Customize Me

## Config Location
* **Application home (`/home/marius/blazeai`):** Contains `backups`, `config`, `projects`, `scripts`, `skills`. Each top-level folder has a `README.md` that documents its structure, use, and rules. **When a task involves any of these folders, you MUST read its `README.md` first** before inspecting or modifying any other file in that folder.
- Runtime configuration lives at `{APP_HOME}/config/config.json` — providers, models, roles, compaction, reasoning.
- Work modes live separately at `{APP_HOME}/config/modes.json` — mode definitions and last active mode.
- API keys are stored in config.json. Modes reference provider/model names but never contain keys.

## Config Structure
```json
{
  "providers": [
    {
      "name": "provider_name",
      "endpoint": "https://api.example.com/v1",
      "api_key": "sk-..."
    }
  ],
  "favorite_models": [
    "provider_name/model_name"
  ],
  "roles": {
    "default": "provider_name/model_name",
    "vision": "provider_name/model_name",
    "summarization": "provider_name/model_name",
    "advisor": "provider_name/model_name"
  },
  "compaction": {
    "maxContextTokens": 100000,
    "minContextTokens": 50000,
    "summaryMaxTokens": 2000,
    "maxSummaryFiles": 10,
    "tokenCoefficient": 3.5,
    "maxBackoffOffsetTokens": 25000
  },
  "stripReasoning": {
    "enable": true,
    "preserveLast": 5
  }
}
```

## Provider Definition
- Each provider has exactly three fields: `name`, `endpoint`, `api_key`.
- Only OpenAI-compatible providers are supported.
- No fallback providers. No automatic provider switching on failure.

## Model Roles
- `default`: required. Used for normal agent interaction and core runtime work.
- `vision`: optional. Intended for vision tasks.
- `summarization`: optional. Intended for summarization and compact review tasks.
- `advisor`: optional. Intended for one-shot delegated analysis to a stronger model.

## How To Edit
1. Read the current file with the `shell` tool.
2. Modify the JSON with the `shell` tool (use replace_block or direct file editing).
3. **config.json**: provider and role changes require a session restart.
4. **modes.json**: new/edited modes are hot-reloaded on the next Tab cycle. No restart needed.
5. **modes.json safety**: the runtime reads modes.json, validates the JSON, and falls back to a safe default mode if the file is corrupted. However, always validate the JSON before saving when possible.
6. The `/model` command changes the current model (NOT the mode). `/model` does NOT accept mode names; it only accepts `provider/model_name`.

## Fetching Models from Providers

When the user asks to browse models (e.g. "show deepseek models"), follow this process.

### Algorithm
1. Check if a helper script already exists at `{APP_HOME}/scripts/fetch_models`. Reuse it if present.
2. If not, create it on the fly using available shell tools. Python is last resort.
3. The script reads config.json to find the requested provider's endpoint and API key.
4. Calls `<endpoint>/models` with the key in the Authorization header.
5. Parses the JSON response (`data[].id`), filters by the search string (case-insensitive).
6. Outputs one `provider/model_id` per line.

### Creation guidelines
- Write the script to `{APP_HOME}/scripts/fetch_models` with OS-appropriate extension (.sh, .ps1, .py).
- Accept two arguments: `<provider_name>` and `[filter]`.
- API keys must be read from disk — never hardcoded in the script.
- Make it executable (`chmod +x` on Unix).

### Usage
Call per provider: `fetch_models <provider_name> <filter>`.

### Presenting results
1. If user specified a provider: query just that provider.
2. If not: query providers sequentially until matches are found.
3. Show results as a numbered list. Ask the user to pick.
4. Use the selected ID directly — it is already in `provider/model` format.

## Work Modes (modes.json)
Modes are stored in `{APP_HOME}/config/modes.json` — separate from config.json to isolate
frequently-edited mode data from critical provider/API key configuration. Each mode
binds a model and an optional directive that is injected into the last message
sent to the LLM (volatile — not stored in session history).

### Structure
Modes.json is a standalone JSON file (not embedded in config.json):
```json
{
  "modes": [
    {
      "name": "default",
      "model": "provider/model_name",
      "directive": ""
    },
    {
      "name": "planning",
      "model": "openai/gpt-4o",
      "directive": "You are in planning mode. Use only read-only tools and propose a plan."
    }
  ],
  "last_mode": "default"
}
```

### Rules
- `modes`: array of mode objects, at least one entry required.
- `name`: unique, non-empty.
- `model`: must exist in favorite_models and reference a configured provider.
- `directive`: free text. Empty string = no directive injected.
- `last_mode`: persists the active mode between sessions; must match an existing mode name.
- At least one mode must exist (the `default` mode is pre-created on first run).

### Operations you can perform
- Create a new mode: append an entry to the `modes` array in modes.json and persist. Validate with the rules above.
- Edit a mode's directive or model: find by `name`, update in modes.json, persist.
- Delete a mode: remove from `modes` array in modes.json. If it was `last_mode`, set `last_mode` to the first remaining mode. Never delete the last remaining mode.
- Switch active mode at runtime: the user presses Tab to cycle modes. Newly created modes are hot-reloaded and appear in the cycle immediately. Do NOT suggest `/model modename` — that command does not switch modes.
- After any edit to modes.json, validate integrity (unique names, valid models, provider references).

**After creating or editing a mode:** remind the user to Tab-cycle out and back into that mode for the changes to take effect. The active session's `CurrentMode` holds a stale snapshot until the next cycle.

### Directive behavior
The directive is appended to the last message of the payload sent to the LLM on every LLM call while the mode is active. It is not stored in session.json. Use it to constrain agent behavior for the current task (e.g. read-only, quick/cheap, verbose, etc.). Keep directives short and imperative.

**CRITICAL: Write the directive in English only. Never include translations, dual-language content, separator labels like `[MODE DIRECTIVE]`, or non-English text. The directive is read by the LLM — it is not for the user. Even if the user speaks another language, the directive must be a single block of English text.**

## Customizing Builtin Skills

Builtin skills (skill-manager, customize_me, setup_helpers) are seeded into the global skills directory at startup: `{APP_HOME}/skills/<name>/skill.md`. The embedded template is copied only if the file does not already exist — existing files are never overwritten.

**To customize a builtin skill:** edit `{APP_HOME}/skills/<name>/skill.md` directly. Changes persist across restarts.

**To restore a builtin skill to its original:** delete the folder `{APP_HOME}/skills/<name>/` and restart BlazeAI. The original template will be re-seeded on next startup.

**Rules:**
- Only edit skills in `{APP_HOME}/skills/` — never modify embedded files (they are read-only templates).

## Skill Locations

Skills live in two scopes, each with a specific directory layout:

- **Global:** `{APP_HOME}/skills/<name>/skill.md` — shared across all projects. Use for general-purpose skills (network info, backup scripts, personal preferences, cross-project tooling).
- **Project:** `{APP_HOME}/projects/<project>/skills/<name>/skill.md` — scoped to the current project. Use for project-specific build rules, architecture, deploy scripts, or skills with paths/names tied to one codebase.

### Creating or Editing a Skill

When the user asks to create or modify a skill, determine the scope:

- **100% certain it's global** (generic, no project-specific references) → write to global path.
- **100% certain it's project** (references the current project by name, paths, or conventions) → write to project path.
- **Ambiguous** → ask the user: "Should this be global (reusable across projects) or project (specific to <current project>)?"

### Generalizing a Skill for Global Use

If the user wants a project-specific skill to become global, or if you determine a skill should be global:

- Remove absolute paths — use `{SKILL_DIR}`, `{APP_HOME}`, or generic paths.
- Remove project names, branch names, or project-specific conventions.
- Replace OS-specific commands with cross-platform alternatives when possible.
- Write examples in a generic form that applies to any project.
- If after generalizing the skill loses all meaningful content, it cannot be global — keep it project.
- After editing a skill, the changes take effect on the next prompt build (no restart needed for content changes).
- If you break a skill's format (delete [DESCRIPTION] etc.), the skill simply won't appear in the available list. Delete the folder to restore.
