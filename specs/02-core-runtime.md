# 02 - Core Runtime

## Overview
- This spec defines the BlazeAI runtime mechanics: provider and model configuration, prompt construction, session persistence, tool contract, skills, memory, and first-run setup.
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
  3. `AGENTS.md` from the current work folder, if it exists
  4. `memory.md` from the memory folder
  5. skills section

### Skills Section
- The skills section has two distinct parts:
  1. **Available skills**: the `[DESCRIPTION]` of every discovered skill, concatenated, including the skill file name.
  2. **Active skills**: the `[DETAILS]` of every skill currently in the active skills list, concatenated, with a context description so the LLM understands what this section is.

- The available skills block tells the LLM which skills can be loaded.
- The active skills block injects the full details of only the skills the user or agent has activated.
- Both blocks are part of the runtime prompt and are rebuilt on every call.

### Source Files
- Universal prompt: `prompts/sysprompt.md`
- OS prompt: `prompts/sysprompt.<os>.md` where `<os>` is `linux`, `darwin`, or `windows`
- `AGENTS.md`: read from the current work folder only; not recursive; optional
- Memory: `app_home/memory/memory.md`
- Builtin skills: `skills/` in the project distribution
- Custom skills: `app_home/skills/`

### Missing Source Handling
- Universal prompt: required; if missing, the runtime stops with a clear error.
- OS prompt: required; if missing, the runtime stops with a clear error.
- `AGENTS.md`: optional; if missing, it is omitted silently.
- `memory.md`: optional; if missing, it is omitted silently.
- Skills: optional; if no skills are found, the skills section is omitted.

### Variable Injection
- Prompt files and skill files support variable placeholders in the form `{VARIABLE_NAME}`.
- The runtime replaces placeholders with resolved values at prompt build time.
- Guaranteed variable in this phase:
  - `{APP_HOME}`: the resolved application home path
- Additional variables may be added in future phases.
- Unknown or unresolvable variables are left as-is in the text.

### Conversation Part
- The conversation part is the message history persisted in the session folder.
- It contains user messages, assistant messages, and tool call / tool result messages.
- The conversation part is loaded from the session JSON file and appended after the runtime part.
- The conversation part is not rebuilt; it is the accumulated history of the session.

## Active Skills

### Active Skills List
- The runtime keeps an in-memory list of active skill names (for example `memory.md`).
- The list starts empty at the beginning of every session.
- The list is not persisted in the session JSON.
- The list is not deduced from conversation history.

### Load And Unload
- `load_skill` tool: adds a skill name to the active skills list.
- `unload_skill` tool: removes a skill name from the active skills list.
- These tools only modify the in-memory list. They do not touch disk.
- At the next prompt build, the runtime reads the `[DETAILS]` of every skill in the active list and injects them.

## Skills

### Skill Format
- Every skill file is Markdown.
- Every skill must contain two fixed sections:
  - `[DESCRIPTION]`: short summary shown in the available skills block.
  - `[DETAILS]`: full content injected when the skill is active.
- Skills without these sections are invalid and are reported as errors.

### Skill Discovery
- Skills are discovered from two locations:
  1. builtin skills: `skills/` in the project distribution
  2. custom skills: `app_home/skills/`
- Both locations are read on every prompt build.
- The skill file name is the skill identifier.

### Skill Collision
- `create_skill` validates and forbids creating a skill with the same name as an existing one.
- If despite that a custom skill has the same name as a builtin skill, the custom skill wins.
- The builtin skill with the same name is ignored when a custom one exists.

### Builtin Skills
- At least three builtin skills ship with the product:
  1. `memory`: explains how and where persistent memory is stored and updated.
  2. `create_skill`: explains how and where to create a new custom skill, including naming, format, and validation rules.
  3. `customize_me`: explains how to configure providers, models, and roles in `config.json`, enabling the LLM to assist with auto-configuration.

## Memory

### Location
- Memory lives at `app_home/memory/memory.md`.
- A single memory file is used in this phase.

### Role
- Memory content is injected into the runtime part of every prompt build.
- Memory is read fresh from disk on every prompt build.

### Updates
- Memory updates are performed explicitly by the agent using the `shell` tool or by user action.
- The runtime does not automatically write to memory.

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
