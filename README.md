# BlazeAI

BlazeAI is a fast, shell-native AI agent for experienced terminal users. It
streams responses from OpenAI-compatible providers, executes local tools, keeps
file-based sessions, and can also run as a Telegram bridge.

## Features

- Console REPL with streaming Markdown output
- OpenAI-compatible HTTP streaming
- Shell execution through the native host shell
- File tools, exact block replacement, task tracking, and image analysis
- Markdown skills loaded from builtin, global, and project scopes
- One-shot Markdown sub-agents with explicit tool allowlists
- Work modes that restrict direct tools and available sub-agents
- File-based sessions with resume and context compaction
- Optional Telegram transport with one configured chat per instance
- Explicit startup errors when required configuration is missing or invalid

## Requirements

- Go 1.25 or newer
- A configured OpenAI-compatible provider and model
- A TTY for the console transport

The module declares Go toolchain `go1.26.4` and uses `CGO_ENABLED=0` for
portable builds.

## Run From Source

```bash
go run .
```

The first run opens an interactive setup flow. It asks for a provider, API key
or OAuth authentication where supported, a model, and optional role models.
Configuration is stored under:

```text
~/blazeai/config/config.json
```

BlazeAI also creates editable prompt templates and builtin skills under
`~/blazeai/` without overwriting existing user files.

## Console Usage

```bash
# Start a new session
go run .

# Continue the last cleanly closed session
go run . -c

# Resume the most recent session, clean or interrupted
go run . -r

# Explicitly select the console transport
go run . --console
```

Available console commands:

| Command | Purpose |
|---------|---------|
| `/auth openai` | Connect ChatGPT through browser OAuth |
| `/model` | Select a provider and model interactively |
| `/model provider/model` | Switch to a specific configured model |
| `/cd <path>` | Change the working directory |
| `/clear` | Clear the current conversation |
| `/new` | Start a clean session |
| `/exit` | Close the session cleanly |

Press `Tab` to cycle work modes. Press `Ctrl-C` to abort an active turn.

## Telegram Transport

Create an instance configuration at:

```text
~/blazeai/telegram/<instance>/bridge.json
```

Example:

```json
{
  "bot_token": "123456:replace-me",
  "allowed_chat_id": 123456789,
  "workdir": "/absolute/path/to/project"
}
```

The bot token, chat ID, and absolute working directory are all required.
Start the bridge with:

```bash
go run . --telegram <instance>
```

The bridge accepts messages only from the configured chat. Sudo commands are
not approved over Telegram because the transport has no secure password input.

## Runtime Data

BlazeAI stores machine-local data under `~/blazeai/`:

```text
~/blazeai/
├── agents/       # Markdown one-shot agent definitions
├── backups/      # Safety copies created by tools
├── config/       # config.json and modes.json
├── projects/     # Project-scoped sessions and skills
├── prompts/      # Editable universal, OS, and transport prompts
├── scripts/      # Helper scripts and optional Python environment
├── skills/       # Global and seeded builtin skills
└── telegram/     # Telegram bridge instances
```

Project sessions are stored under:

```text
~/blazeai/projects/<project>/sessions/<session>/
```

Each session contains `session.json`; token analytics are stored separately in
`token-usage.json`. Active skills are session-memory state and are not persisted
in the session file.

## Skills

Skills are Markdown files with exactly two required sections: `[DESCRIPTION]` for system-prompt discovery and `[BODY]` for content returned by `load_skill`.

Supported locations are:

- Builtin skills embedded in the binary and seeded to `~/blazeai/skills/`
- Global skills in `~/blazeai/skills/`
- Project skills in the current project's `skills/` directory

Use `load_skill` to add a skill body to the conversation as a standard tool message.

## One-Shot Agents

Agent definitions live in `~/blazeai/agents/*.md` and use YAML-like front
matter:

```markdown
---
name: reviewer
description: Review source code for defects
kind: one-shot
tools:
  - read_file
  - shell
---

Describe the agent's behavior here.
```

Each agent must declare an explicit non-empty tool list. Child agents complete
through the internal `agent_done` protocol and may use persistent child session
IDs for later resumption.

## Configuration Concepts

`config.json` contains:

- Providers and credentials
- Favorite models
- `default`, `vision`, `summarization`, and `advisor` model roles
- Context compaction settings
- Reasoning payload-stripping settings
- Optional prompt debugging (`debugPrompt`)

`modes.json` contains work modes with:

- `name`
- `model`
- Optional `directive`
- Optional `denied_tools`
- Optional `agents` allowlist

Missing or invalid required configuration stops startup with an error. BlazeAI
does not silently select fallback providers, models, or modes.

## Development

Run the test suite:

```bash
go test ./...
```

Build all packages:

```bash
go build ./...
```

Build a local executable without replacing any user-installed wrapper:

```bash
go build -o /tmp/blazeai .
```

For a Linux AMD64 deployment, use the repository deployment script:

```bash
./deploy_nas.sh [user@host]
```

The script builds a static binary, packages an installer, and deploys it to the
remote user's `~/.local/bin/blazeai`.

## Project Layout

```text
main.go                 # Application entry point
firstrun.go             # Interactive first-run setup
internal/runtime/       # Agent loop and child-agent orchestration
internal/provider/      # OpenAI-compatible streaming client
internal/tools/         # Native tool implementations
internal/prompt/        # Prompt assembly
internal/session/       # File-based persistence
internal/skills/        # Skill parsing, discovery, and body loading
internal/agents/        # Markdown agent definitions
internal/console/       # Terminal transport
internal/telegram/      # Telegram transport
internal/config/        # Configuration and work modes
internal/compaction/    # Context pruning and summaries
prompts/                # Embedded default prompt templates
skills/                 # Embedded builtin skill templates
specs/                  # Detailed subsystem specifications
```

See `specs/` for detailed behavioral and subsystem documentation.
