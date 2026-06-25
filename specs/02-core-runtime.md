# 02 - Core Runtime

## Overview
- This spec defines the BlazeAI runtime mechanics: provider and model configuration, prompt construction, session persistence, tool contract, skills, and first-run setup.
- Product intent and interface shape are defined in `01-product-scope.md`.
- Interface implementation details are defined in `03-interfaces.md`.
- Platform and operations behavior is defined in `04-platform-ops.md`.

## Provider And Model Configuration

### Provider Definition
- A provider is an OpenAI-compatible endpoint.
- Each provider is defined by exactly three fields:
  - `name`: unique provider identifier
  - `endpoint`: base URL for the OpenAI-compatible API
  - `api_key`: secret key for that endpoint, stored in `config.json`
- Only OpenAI-compatible providers are supported.
- No fallback providers. No automatic provider switching on failure.

### Model Definition
- Models are predefined in configuration, not discovered at runtime from the endpoint.
- The canonical model identifier format is `provider/model_name`.
- The runtime never works with a bare model name separate from its provider.
- The current model selection is always a full `provider/model_name` identifier.
- Each provider can define multiple models.

### Model Roles
- Three roles are predefined:
  - `default`: the model used for normal agent interaction
  - `vision`: a model intended for vision tasks
  - `summarization`: a model intended for summarization tasks
- Each role maps to one `provider/model_name` entry in configuration.
- The `default` role is required for the runtime to start.
- `vision` and `summarization` roles are optional but recommended.

### Config Location
- The runtime configuration lives in `app_home/config/config.json`.
- The config file contains:
  - the list of providers, each with `name`, `endpoint`, `api_key`
  - the list of favorite models per provider, each in `provider/model_name` form
  - the role assignments mapping `default`, `vision`, and `summarization` to specific `provider/model_name` entries
- `config.json` lives in `app_home/config/` and is not committed to version control.

### Config Validation
- At startup, the runtime validates the config.
- If the config is missing or the `default` role is not assigned, the runtime starts first-run setup.
- If a referenced provider or model is malformed, the runtime stops with a clear error message.
- No fallback model is ever selected automatically.

### Provider And Model Selection
- `/model` is a runtime command, not a config editor.
- The exact behavior of `/model` without arguments is defined in `03-interfaces.md`.
- Changing the current model at runtime always sets a full `provider/model_name` identifier.
- The last selected model persists across sessions in config.

## First-Run Setup

### Trigger
- First-run setup triggers when the config file is missing or the `default` model role is not assigned.

### Goal
- The user must define at least one provider and assign the `default` model role before the runtime can start a session.
- `vision` and `summarization` roles can be configured during setup or left for later.

### Auto-Configuration Skill
- A builtin skill named `customize_me` exists to let the LLM assist the user with configuration.
- The `customize_me` skill explains how to define providers, models, and roles in `config.json`.
- The skill can be loaded by the user or by the agent when configuration help is needed.

## Prompt Build

### Build Timing
- The prompt is rebuilt on every LLM call.
- Every prompt build reads all sources fresh from disk at the moment of the call.
- Nothing from a previous prompt build is reused.

### Prompt Structure
- The prompt has two major parts:
  1. **Runtime part**: rebuilt from files and variables on every call.
  2. **Conversation part**: the persisted user/assistant/tool message history from the session folder.

### Runtime Part Order
- The runtime part is assembled in this exact order:
  1. universal system prompt (`prompts/sysprompt.md`)
  2. OS-specific system prompt (`prompts/sysprompt.<os>.md`)
  3. host helpers section (optional)
  4. skills section
  5. `AGENTS.md` from the current work folder, if it exists

### Skills Section
- The skills section has two distinct parts:
  1. **Available skills**: the `[DESCRIPTION]` of every discovered skill, concatenated, including the skill file name.
  2. **Active skills**: the `[BEHAVIOR]` (procedural guidance) and `[DATA]` (persistent facts) sections of every skill currently active, concatenated.

- The available skills block tells the LLM which skills can be loaded.
- The active skills block injects the full details of only the skills the user or agent has activated.
- Both blocks are part of the runtime prompt and are rebuilt on every call.

### Source Files
- Universal prompt: `prompts/sysprompt.md`
- OS prompt: `prompts/sysprompt.<os>.md` where `<os>` is `linux`, `darwin`, or `windows`
- `AGENTS.md`: read from the current work folder only; not recursive; optional
- Builtin skills: `skills/` in the project distribution
- Custom global skills: `app_home/skills/`
- Project skills: `<workdir>/.blazeai/skills/`

### Missing Source Handling
- Universal prompt: required; if missing, the runtime stops with a clear error.
- OS prompt: required; if missing, the runtime stops with a clear error.
- `AGENTS.md`: optional; if missing, it is omitted silently.
- Skills: optional; if no skills are found, the skills section is omitted.

### Variable Injection
- Prompt files and skill files support variable placeholders in the form `{VARIABLE_NAME}`.
- The runtime replaces placeholders with resolved values at prompt build time.
- Guaranteed variables:
  - `{APP_HOME}`: the resolved application home path
  - `{WORK_DIR}`: the current working directory
  - `{SKILL_DIR}`: the skill's directory path (only in skill content)
- Additional variables may be added in future phases.
- Unknown or unresolvable variables are left as-is in the text.

### Conversation Part
- The conversation part is the message history persisted in the session folder.
- It contains user messages, assistant messages, and tool call / tool result messages.
- The conversation part is loaded from the session JSON file and appended after the runtime part.
- The conversation part is not rebuilt; it is the accumulated history of the session.

## Active Skills

### Active Skills List
- The runtime keeps an in-memory list of active skill IDs.
- The list starts empty at the beginning of every session.
- The list is not persisted in the session JSON.
- The list is not deduced from conversation history.

### Load And Unload
- `load_skill` tool: resolves the skill name (handling scope prefixes) and adds the canonical ID to the active list.
- `unload_skill` tool: removes a skill ID from the active list.
- These tools only modify the in-memory list. They do not touch disk.
- At the next prompt build, the runtime reads the `[BEHAVIOR]` and `[DATA]` of every skill in the active list and injects them.
- If a loaded skill no longer exists on disk, it is silently omitted from the prompt.

## Skills

### Skill Format
- Every skill file is Markdown.
- Every skill must contain three sections:
  - `[DESCRIPTION]`: short summary shown in the available skills block (required).
  - `[BEHAVIOR]`: procedural guidance, workflow rules, decision logic (optional).
  - `[DATA]`: persistent facts in compact key=value format (optional).
- At least one of `[BEHAVIOR]` or `[DATA]` must be present.
- Skills without `[DESCRIPTION]` or without at least one content section are invalid and skipped.

### Skill Scopes
- Three scopes with resolution rules:
  1. **builtin**: skills embedded in the binary; bare name resolution
  2. **global**: custom skills in `app_home/skills/<name>/skill.md`; bare name, overrides builtin
  3. **project**: skills in `<workdir>/.blazeai/skills/<name>/skill.md`; uses `project/` prefix

- Unqualified bare name resolves to a unique match across scopes.
- Ambiguous bare name (exists both global and project) → error listing candidates.
- Scoped name (`project/foo`) → exact lookup in that scope.

### Skill Discovery
- Skills are discovered from three locations on every prompt build:
  1. builtin skills: `skills/` embedded in the project distribution
  2. custom global skills: `app_home/skills/<name>/skill.md`
  3. project skills: `<workdir>/.blazeai/skills/<name>/skill.md`
- Each skill is a subfolder containing `skill.md`.

### Skill Collision
- `skill-manager` validates and forbids creating a skill with the same name as an existing one.
- If a custom skill has the same name as a builtin skill, the custom skill wins.
- The builtin skill is ignored when a custom override exists.
- Project skills use `project/` prefix to avoid collision with global/builtin.

### Builtin Skills
- At least two builtin skills ship with the product:
  1. `skill-manager`: explains how to create, modify, review, and repair skills. Includes skill format, data authoring rules, and the behavior/data distinction.
  2. `customize_me`: explains how to configure providers, models, and roles in `config.json`, enabling the LLM to assist with auto-configuration.

## Memory

Memory is handled through skills. Persistent facts are stored in the `[DATA]` section of skills, eliminating the need for a separate memory subsystem. The old `app_home/memory/memory.md` file is no longer used or created by the runtime.

## Sessions

### Session Creation
- Every new session creates a folder with a random name under `app_home/sessions/`.
- The session folder stores the full message JSON for that session.

### Session Persistence
- The session JSON contains the complete message array exactly as sent to the LLM, including tool calls and tool results.
- The session JSON also contains a `closed_cleanly` boolean field, set to `false` by default and to `true` only when the session is closed via `/exit`.
- The session JSON is updated as the conversation progresses.
- The conversation part of the prompt is loaded from this JSON.

### Session Lifecycle
- A new session starts by default.
- The `-c` flag continues the last session that was closed cleanly with `/exit`.
- A session is considered cleanly closed only if it has an explicit clean-close marker in its metadata.
- If no cleanly closed session exists, `-c` stops with a clear error.

### What Is Not Persisted In Session
- The active skills list is not persisted in the session.
- The runtime part of the prompt is not persisted in the session.
- Only the conversation messages are persisted.

## Tools

### Tool Contract
- Tools follow the standard OpenAI tool-calling format.
- Multiple tool calls per turn are supported.
- Each tool call can include an optional `timeout` parameter.

### Tool Timeout
- The default tool timeout is 60 seconds.
- If a tool call exceeds its timeout, the tool returns exactly:
  - `timeout <N>s exceeded`
  - where `<N>` is the actual timeout value used.
- This format lets the LLM understand the failure and either retry with a larger timeout or abandon the task.
- If `timeout` is not provided in the tool call, the default value is used.

### Tool Error Handling
- Non-timeout tool errors return raw `stdout`, `stderr`, and `exit_code`.
- The runtime does not normalize non-timeout error output.
- The raw output is persisted in the session JSON as the tool result.

### Native Tools
- The runtime exposes a small fixed set of native tools:
  1. `shell`: execute a shell command on the host.
  2. `load_skill`: add a skill to the active skills list.
  3. `unload_skill`: remove a skill from the active skills list.
  4. `replace_block`: replace a block of text in a file by matching an old block and writing a new block.
- No script-based or dynamically discovered tools are supported in this phase.
- The tool set is intentionally minimal and hardcoded in the runtime.

## Work Folder

### Initial Work Folder
- The initial work folder is the directory from which the application is started.

### Changing Work Folder
- The `/cd` command changes the current work folder during a session.
- If `/cd` receives an invalid path, the runtime reports a clear error and keeps the current work folder.
- Changing the work folder affects both tool execution and the `AGENTS.md` source in prompt build.

## Error Handling Policy

- The runtime never falls back silently.
- Missing required files or invalid configuration stop the runtime with a clear error message.
- Optional missing sources are omitted silently.
- Tool errors are returned as raw output, not normalized.
- Timeout errors use the fixed `timeout <N>s exceeded` format.
