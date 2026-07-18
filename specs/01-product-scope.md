# Product Scope

**Source files:**
- `main.go` — entry point, CLI flags, startup sequence
- `firstrun.go` — interactive first-run setup
- `embed.go` — embedded filesystem declarations
- `internal/platform/platform.go` — app home, OS detection, shell selection
- `internal/config/config.go` — config schema, validation, first-run detection

## Product Identity

BlazeAI is a fast, cross-platform AI terminal agent for experienced users. It is a
greenfield rebuild from specification with no legacy code or backward compatibility
constraints. The interaction model is command-driven, shell-native, and optimized for
short path to execution. BlazeAI should feel like a sharp terminal assistant, not a
generic chatbot.

The product favors explicit control over hidden automation. Prompt behavior is a
major source of product personality and control — most agent behavior is shaped
by system prompt files, not runtime logic.

## Priorities (Ordered)

1. **Speed of interaction** — minimal overhead per turn, fast startup, streaming output
2. **Simplicity of execution** — direct shell execution, minimal abstractions
3. **Cross-platform behavior** — consistent on Linux, macOS, and Windows
4. **Visual clarity** — Markdown rendering, colored labels, clear visual separation

Any new feature must justify its cost in complexity.

## Target Users

Technical users familiar with terminals, shell commands, local files, and LLM
workflows. No beginner wizards, guided flows, or heavy onboarding. The UI assumes
the user can reason about shell execution, file paths, environment variables, and
Git workflows.

## Interaction Model

- Primary interface: CLI REPL (not full-screen TUI)
- Secondary transport: Telegram bridge (long-polling bot)
- Console UX: Markdown rendering, colored labels (`💻`, `🧠`, `[BLAZE]`),
  streaming output, visual separators between turns
- Console is TTY-only — no pipe or non-TTY support
- All transports implement the same `runtime.Handler` contract
- Console slash commands: `/auth`, `/model`, `/cd`, `/clear`, `/new`, `/exit`. Unknown `/...` passed to LLM as user message
- Tab cycles through work modes (defined in `modes.json`)

## Execution Model

- **Primary tool**: `shell` — executes commands via the host platform shell
- **Fast execution policy**: batch independent tool calls when supported; order dependent operations sequentially; combine tightly related shell commands with fail-fast chaining
- **Direct commands versus scripts**: use direct shell for clear, compact execution; create a task script for loops, repeated operations, structured parsing, multi-step transformations, branching, fragile quoting, or when it reduces total tool calls and tokens
- **Script language**: choose OS-native shell or Python according to the simplest, most robust, token-efficient solution
- **Task script location**: `app_home/scripts/tasks/<task-slug>/`; do not create task scripts in the user project unless the task explicitly requires a project script
- **Platform shells**:
  - Linux: `bash` → `sh`
  - macOS: `bash` → `sh`
  - Windows: `pwsh` → `powershell.exe` → `cmd.exe` (priority order)
- **Python**: task scripts run only through the lazily-created venv under `app_home/scripts/venv/`; install libraries only in that venv, with system-environment changes allowed only when explicitly requested or when the task targets the system Python environment
- **Host helpers** (rg, fd, jq, git, xh, pandoc, sqlite3): detected at startup,
  listed in prompt so the LLM knows they are available without checking
- **Native tools**: 9 base native tools plus conditional `run_agent` (`shell`, `load_skill`, `replace_block`, `ask_a_friend`, `analyze_image`, `task_read`, `task_write`, `read_file`, `write_file`)
- OpenAI-compatible tool calling with multi-tool-call per turn
- Default tool timeout: 60s. Timeout returns `"timeout <N>s exceeded"`
- Shell output capped at 150kB (combined stdout + stderr)

## No Fallbacks Rule

**Mandatory:** If something is missing or not configured, the app MUST stop with a
relevant error message. A fallback or silent degraded mode is considered a critical
error. This applies to:

- Missing or invalid config
- Missing `default` role assignment
- Invalid provider or model reference
- Missing session files
- Missing app home directory
- Missing required prompt files
- Corrupt config files
- Invalid mode definitions

There is no silent recovery, no default values when config is missing, and no
auto-provisioning beyond the explicit first-run setup flow.

## Architecture Overview

- **Single binary** built with `CGO_ENABLED=0`, Go 1.25.0+ minimum
- **Flat Go layout**: no `cmd/` tree, transports under `internal/`
- **Layer stack**:
  - Application entry: `main.go` — flag parsing, wiring, transport startup
  - Agent core: `internal/runtime/` — RunTurn, tool loop, prompt builder, compactor, orchestration
  - Transports: `internal/console/`, `internal/telegram/` — implement Handler
  - Tools: `internal/tools/` — 9 base native tool implementations plus conditional `run_agent`
  - Agent definitions: `internal/agents/` — Markdown agent discovery, parsing, validation
  - LLM client: `internal/provider/` — OpenAI-compatible streaming HTTP client
  - Secondary calls: `internal/llmcall/` — one-shot role-based LLM consultation
  - Usage normalization: `internal/usage/` — provider-agnostic token usage extraction
  - Session persistence: `internal/session/` — file-based, no database
  - Skills: `internal/skills/` — strict parsing, scoped discovery, and body loading
  - Config: `internal/config/` — load/save/validate
  - Platform: `internal/platform/` — OS detection, app home, shell selection

## App Home

Resolved from the OS home directory at `$HOME/blazeai`. Created at first start if
missing. Standard subfolders:

| Subfolder | Purpose |
|-----------|---------|
| `skills/` | Global user skills |
| `scripts/` | Script storage |
| `scripts/venv/` | Python virtual environment (lazy-created) |
| `backups/` | LLM-initiated backups |
| `projects/` | Per-project data (sessions, skills) |
| `config/` | `config.json`, `modes.json` |
| `telegram/` | Telegram bridge instance configs |
| `agents/` | Markdown agent definitions (one-shot sub-agents) |

`{APP_HOME}` is injected into prompts and skill templates at build time.

## Configuration

- Single source of truth: `app_home/config/config.json`
- Separate file for work modes: `app_home/config/modes.json`
- API keys stored in `config.json` (NOT in `.env` or environment variables)
- Provider schema: `name`, `endpoint`, `api_key` (or `auth_type: "oauth"` with OAuth credential)
- Model format: `provider/model_name`
- Four model roles: `default` (required), `vision`, `summarization`, `advisor` (optional)
- Work modes: `name`, `model`, `directive`, `denied_tools`, `agents`
- Last selected model persists per session transport
- First-run: interactive setup with provider list → API key → model selection → role assignment

## Sessions

- File-based storage in `app_home/projects/<project>/sessions/`
- No database. Plain JSON files.
- New session by default; `-c` continues last cleanly closed session; `-r` resumes last session
- `session.json` contains full message array exactly as sent to LLM
- Sessions persist indefinitely on disk (no automatic cleanup)


## Skills

- Markdown files with exactly two required sections: `[DESCRIPTION]` and `[BODY]`
- Three scopes: builtin (embedded), global (`app_home/skills/`), project (project `skills/`)
- Descriptions are rebuilt into the system prompt on every LLM call
- `load_skill` returns the expanded body as a standard persisted tool message
- No active-skill state, repeated system-prompt injection, or `unload_skill` tool

## Agent Definitions

- Markdown files with `---` front matter in `app_home/agents/`
- Required fields: `name`, `description`, `kind: "one-shot"`, `tools` (explicit allowlist)
- Optional fields: `model`, `timeout`
- The Markdown body is the behavioral prompt injected as `sysprompt.agent.md`
- One-shot children: ephemeral child sessions with bounded execution, `agent_done` protocol
- Child sessions persist under the main session folder in `agents/<id>/`

## Safety

- `sudo` / Run as Administrator only after explicit user approval
- Password entry: interactive terminal only, never in chat or session
- Secrets never in session JSON or prompt text
- No built-in command whitelist/blacklist in shell tool
- Telegram transport: sudo always denied (no password channel)

## Release Targets

| Target | OS | Arch |
|--------|----|------|
| `linux/amd64` | Linux | x86_64 |
| `linux/arm64` | Linux | ARM64 |
| `darwin/amd64` | macOS | x86_64 |
| `darwin/arm64` | macOS | Apple Silicon |
| `windows/amd64` | Windows | x86_64 |

Conservative Linux targets — minimal libc dependency, validate on older systems.

## Non-Goals

- BlazeAI is not a database-backed assistant platform
- BlazeAI is not a full-screen TUI project
- BlazeAI is not Python-first
- BlazeAI is not designed for non-technical users
- Automatic session cleanup is not in scope
- No fallback providers or models
- No separate memory subsystem — skill bodies may carry reference data in ordinary conversation history
- No native plugin system beyond skills — loaded skill bodies provide instructions and reference content

## Scope Boundary

This file defines product identity, user model, interaction model, execution
philosophy, and top-level constraints. Detailed mechanics for each subsystem
belong in their respective spec files:
- Runtime mechanics → `15-runtime-core.md`
- Handler contract → `12-handler-contract.md`
- Agent orchestration → `20-agent-orchestration.md`
- Mode capabilities → `21-mode-capabilities.md`
- Tool specifics → `05-tools.md`, `06-shell-execution.md`, `07-file-editing.md`,
  `08-cross-model-delegation.md`
- Skill system → `09-skill-system.md`
- Console transport → `13-console-ui.md`
- Telegram transport → `14-telegram-bridge.md`
- Platform-specific rules → `17-platform.md`
- Safety and secrets → `18-safety.md`
- Build and deploy → `19-build-deploy.md`
- Configuration schema → `03-config-schema.md`
- Prompts → `04-prompts.md`
- Sessions → `10-sessions.md`
- Context compaction → `11-context-compaction.md`
- First-run setup → `16-first-run.md`
- Usage normalization → `22-usage-normalization.md`
