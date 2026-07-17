# Project Specs

========== USER SPECS ==========

!! IMPORTANT MANDATORY !! Do not implement fallbacks. If something is missing or not configured, app must stop with a relevant error message !!! A fallback must be considered as a silent critical error !

!! IMPORTANT MANDATORY !! NEVER overwrite ~/.local/bin/blazeai. It is a bash wrapper script that compiles the project from source and runs the resulting binary. Never replace it with a compiled ELF binary, never `go build -o ~/.local/bin/blazeai`, never `cp ./blazeai ~/.local/bin/`, never `install ./blazeai ~/.local/bin/`. If the binary needs updating, use `go build -o /tmp/blazeai .` only. The wrapper script is the single source of truth for local execution.

========== AGENT GENERATED ==========

### Purpose
- BlazeAI is a cross-platform AI terminal agent for experienced users.
- The current implementation is a Go shell-native runtime with console and Telegram transports.
- The system favors fast interaction, low overhead, and explicit failure over silent degradation.

### Technology Stack
- Go module with `go 1.25.0` and `toolchain go1.26.4`.
- Standard library first; current external deps are `golang.org/x/image`, `golang.org/x/term`, and `golang.org/x/sys`.
- Runtime integration is OpenAI-compatible HTTP streaming with shell execution on the host OS.

### Active Scope
- Greenfield rebuild driven by the spec fragments in `specs/`.
- Console is the primary transport; Telegram transport is implemented.
- User-facing behavior should stop on missing configuration or model selection errors rather than degrade silently.

### Architecture And Runtime
- `main.go` bootstraps app home, loads config or runs first-run setup, opens or resumes a session, and starts the selected transport.
- `internal/runtime/` owns the handler contract and the agent loop: prompt build, streaming, mode tool policy, delegated tool calls, persistence, and compaction.
- `internal/agents/` discovers and strictly validates Markdown agent definitions under `app_home/agents/`.
- `internal/prompt/` rebuilds the runtime prompt on every LLM call from `prompts/`, skills, agent definitions, `specs.md`, and `AGENTS.md`.
- `internal/platform/` handles OS detection, shell chain selection, app-home bootstrap, and project directory resolution.
- `internal/skills/` discovers builtin, global, and project skills; active skills live only in memory for the current session.
- `internal/compaction/` prunes long sessions, writes summaries, and strips reasoning from the payload while preserving on-disk session JSON.
- `internal/provider/` talks to OpenAI-compatible endpoints, streams responses, parses tool calls, and reports usage.
- `internal/tools/` implements shell execution, skill tools, ask_a_friend, analyze_image, replace_block, task tools, and filtered registries.
- `internal/console/` is a terminal-only REPL transport with raw input, slash commands, Markdown rendering, and streaming output.
- `internal/telegram/` is a long-polling bridge that enforces one chat, accepts text and images, and adapts runtime streaming into Telegram messages.
- `internal/llmcall/` provides one-shot secondary model calls for role-based delegation.
- `internal/memory/` reads persistent memory text into prompt builds without automatic writes.

### Configuration
- Runtime config lives in `app_home/config/config.json`.
- Work modes live in `app_home/config/modes.json`.
- `config.json` stores providers, favorite models, role assignments, API keys, compaction thresholds, reasoning strip settings, helper preferences, and the last selected model.
- Providers are OpenAI-compatible and use `name`, `endpoint`, and `api_key`.
- Model roles are `default`, `vision`, `summarization`, and `advisor`.
- First-run setup triggers when config is missing or the default role is unassigned, then asks for provider, API key, model, and optional roles.

### Persistence And Protocol Rules
- Sessions are file-based under `app_home/projects/<project>/sessions/`.
- Session JSON is the source of truth for message history and is written exactly as sent to the LLM.
- `closed_cleanly` is set only on `/exit`.
- Summary files live under `summaries/` inside each session folder.
- The transport boundary is `runtime.Handler` with `OnContent`, `OnToolCall`, `OnToolResult`, `OnUsage`, `OnReasoning`, and `RequestSudoApproval`.
- Tool calls follow the OpenAI-compatible tool-calling format with multi-call support and per-call timeouts.
- Markdown agents use `---` front matter with `name`, required `description`, `kind`, optional `model`, and explicit `tools`; the Markdown body is the behavioral prompt. One-shot children complete only through `agent_done`, persist under the main session, and store the current task in `agent_task.md` for system-prompt injection.
- The runtime prompt contains a separate `AGENTS` section with descriptions and run/completion instructions for agents discovered from `app_home/agents/`; work modes remain defined only by `config/modes.json`.
- Modes define `denied_tools` for the main runtime and `agents` for explicitly callable Markdown one-shot sub-agents. Mode restrictions apply only to direct main-runtime tools; child agents use their own explicit tool allowlists. `run_agent` requires a three-sentence `purpose`, accepts an optional persistent child-session `id`, displays that purpose in activity output, and falls back to the task truncated to 80 characters when missing. A resumed child preserves `agent_task.md` and sends the new task as a resume message before its next context build. Child execution is bounded and ordered.

### Sensitive Areas
- `internal/config/` and `internal/config/modes.go` control startup config, role resolution, and mode persistence.
- `internal/session/` and `internal/compaction/` affect persisted conversation state and summary files.
- `internal/provider/` affects streaming protocol, tool-call parsing, and usage accounting.
- `internal/platform/` affects OS detection, shell selection, and app-home layout.
- `internal/tools/` executes shell commands and file edits against the local machine.

### Build And Validation
- `go test ./...`
- `go build ./...`
- `go.mod` pins `GOTOOLCHAIN=auto`, so a Go 1.21+ host can download the pinned toolchain on demand.

### Project Map
- `main.go` - CLI entrypoint. Parses `-c`, `-r`, `--console`, and `--telegram`, bootstraps app home, loads config or first-run setup, opens a session, and starts the selected transport over the agent core. Keywords: entrypoint, flags, bootstrap, session, console, telegram
- `embed.go` - Embeds `prompts/` and `skills/` into the binary with `go:embed`. Keywords: embed, assets, prompts, skills, binary, startup
- `firstrun.go` - Interactive first-run provider, API key, model, and role setup. Keywords: first-run, config, providers, API keys, models, roles
- `go.mod` - Module root and Go toolchain declaration. Keywords: module, toolchain, dependencies, build, Go
- `internal/runtime/` - Agent core orchestration loop and transport handler contract. Builds prompts, calls providers, handles tool calls, persists session messages, triggers compaction, and validates app-home agent definitions. Keywords: runtime, loop, provider, tools, agents, compaction, handler
- `internal/agents/` - Strict Markdown one-shot sub-agent discovery and validation. Supports descriptions, explicit model syntax, and explicit tool allowlists. Keywords: agents, markdown, frontmatter, validation, permissions
- `internal/runtime/agent_orchestration.go` - Persistent one-shot execution with model inheritance, strict `agent_done`, resumable child sessions, agent_task.md replacement, bounded parallelism, cancellation, timeout, ordered results, and immediately emitted scoped tool activity. Parallel activity is displayed in arrival order without timestamps or reordering. Keywords: orchestration, one-shot, parallel, persistence, resume, completion
- `internal/runtime/mode_capabilities.go` - Applies config/modes.json denied_tools to direct runtime tools and limits run_agent to explicitly allowed one-shot agents. Keywords: modes, permissions, delegation, denied-tools, agents
- `internal/console/` - Terminal REPL transport implementing `OnContent`, `OnToolCall`, and `OnToolResult`. Handles raw input, slash commands, and Markdown rendering. Keywords: console, REPL, ANSI, raw mode, streaming, slash-commands
- `internal/telegram/` - Telegram bridge transport with long polling, single-chat enforcement, text/image handling, and streaming output adaptation. Keywords: telegram, bridge, polling, images, handler, transport
- `internal/prompt/` - Rebuilds the runtime prompt from disk sources on every LLM call and injects variables. Keywords: prompt, sysprompt, variables, skills, AGENTS, specs
- `internal/config/` - Loads and validates runtime config and work modes, including providers, roles, compaction settings, and first-run conditions. Keywords: config, validation, modes, roles, providers, compaction, first-run
- `internal/session/` - File-based session persistence under project-scoped session folders. Keywords: sessions, JSON, persistence, resume, clean-close
- `internal/compaction/` - Context compaction, pruning, summary file management, and reasoning stripping. Keywords: compaction, summaries, pruning, reasoning, token budget
- `internal/tools/` - Native tools: shell, skill tools, ask_a_friend, analyze_image, replace_block, task tools, run_agent, agent_done, and filtered registries. Keywords: tools, shell, editing, image, delegation, agents, timeout
- `internal/skills/` - Skill discovery, parsing, validation, scoping, and active list management. Keywords: skills, discovery, parsing, scopes, active-list
- `internal/platform/` - OS detection, shell selection, app home bootstrap (including `agents/`), and project directory resolution. Keywords: platform, OS, app-home, agents, shell, paths, bootstrap
- `internal/provider/` - OpenAI-compatible HTTP client, streaming response parsing, tool-call decoding, and usage reporting. Keywords: provider, HTTP, SSE, OpenAI-compatible, usage, tool-calls
- `internal/llmcall/` - One-shot secondary LLM calls routed by role. Keywords: delegation, advisor, summarization, secondary-call
- `internal/memory/` - Reads persistent memory text into prompt builds without automatic writes. Keywords: memory, prompt-input, read-only, persistence
- `prompts/` - Embedded universal, OS, and transport prompt templates. Keywords: prompts, sysprompt, transport, embedded, runtime
- `skills/` - Builtin skill definitions seeded into app home on startup. Keywords: builtin-skills, seed, prompt-content
- `specs/` - Specification fragments that describe the rebuilt product scope and runtime behavior. Keywords: specs, requirements, runtime, architecture
- `decisions/` - Timestamped session decision summaries used to keep rationale attached to changes. Keywords: decisions, rationale, history, change-log

### Working Rules
- Keep implementations direct and incremental.
- Preserve explicit erroring over silent degradation.
- Treat prompt text and skill content as runtime inputs rebuilt from disk.
- Session folders persist indefinitely in this phase.

### Potential Conflict
- `internal/config/modes.go` currently falls back to a default mode config when `modes.json` is missing or invalid, which conflicts with the user spec's no-fallback rule.
- Current code stores sessions and project skills under `app_home/projects/<project>/...`, which differs from the older user-owned paths in this file.
