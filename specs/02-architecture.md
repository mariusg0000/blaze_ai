# Architecture

## Sources

| Package | File(s) | Role |
|---------|---------|------|
| `main.go` | `main.go`, `embed.go` | Entry point, CLI flags, wiring, embedded assets |
| `internal/runtime/` | `runtime.go` | Agent core, Handler interface, RunTurn loop, model resolution |
| `internal/runtime/` | `agent_orchestration.go` | One-shot child-agent execution, persistent child sessions |
| `internal/runtime/` | `mode_capabilities.go` | Mode tool deny-list and sub-agent allow-list enforcement |
| `internal/console/` | `console.go`, `reader.go` | Console transport (REPL, rendering, input) |
| `internal/telegram/` | `telegram.go`, `handler.go`, `commands.go`, `config.go`, `state.go`, `image_messages.go` | Telegram bridge transport (polling, message handling) |
| `internal/tools/` | `tools.go`, `shell.go`, `skill_tools.go`, `replace_block.go`, `ask_friend.go`, `task_tools.go`, `agent_tools.go`, `read_file.go`, `write_file.go`, `analyze_image.go`, `image_input.go`, `helper_exec.go` | Native tool interface and implementations |
| `internal/agents/` | `agents.go` | Markdown agent definition discovery, parsing, validation |
| `internal/provider/provider.go` | `NewClient`, `StreamWithPhase` | Catalog resolution, protocol dispatch, HTTP streaming |
| `internal/provider/protocol.go` | `Protocol`, `Request`, `ModelReference` | Provider-neutral request contract |
| `internal/provider/openai_chat_protocol.go` | `openAIChatProtocol` | OpenAI Chat validation and lowering |
| `internal/provider/openai_responses_protocol.go` | `openAIResponsesProtocol` | Responses validation and normal/lite lowering |
| `internal/provider/openai_responses.go` | `buildChatGPTRequest`, `buildChatGPTLiteRequest` | OAuth Responses wire builders and parser |
| `internal/config/builtin_model_adapters.json` | `providers`, `adapters` | Embedded provider contracts and model-specific overrides |
| `internal/llmcall/` | `llmcall.go` | One-shot secondary LLM consultation (role-based) |
| `internal/usage/` | `usage.go` | Provider-agnostic token usage extraction and normalization |
| `internal/config/` | `config.go`, `modes.go` | Configuration load/save/validate, work mode management |
| `internal/session/` | `session.go` | File-based session persistence |
| `internal/skills/` | `skills.go` | Skill parsing, discovery, active list management |
| `internal/prompt/` | `prompt.go` | Prompt assembly from embedded templates plus disk project context |
| `internal/compaction/` | `compaction.go` | Context compaction, pruning, summarization, reasoning stripping |
| `internal/platform/` | `platform.go` | OS detection, app home, bootstrap, shell selection |
| `internal/helpers/` | `helpers.go` | Host helper detection (rg, fd, jq, git, etc.) |
| `prompts/` | `sysprompt.md`, `sysprompt.agent.md`, `sysprompt.linux.md`, `sysprompt.darwin.md`, `sysprompt.windows.md`, `transport.console.md`, `transport.telegram.md` | System prompt templates (embedded in binary) |
| `skills/` | `skill-manager.md`, `config-manager.md`, `audit-manager.md` | Immutable builtin skills embedded and read directly from the binary |

## Module Dependency Graph

```
main.go
  ├── internal/platform       (OS, app home, bootstrap)
  ├── internal/config         (config load/save, first-run detection)
  ├── internal/session        (session create/resume)
  ├── internal/skills         (skill discovery/resolution)
  ├── internal/agents         (agent definition loading)
  ├── internal/console        (OR)
  ├── internal/telegram       (OR)
  └── internal/runtime        (agent core)
        ├── internal/config
        ├── internal/session
        ├── internal/platform
        ├── internal/prompt
        │     ├── internal/skills
        │     ├── internal/helpers
        │     ├── internal/platform
        │     ├── internal/agents
        │     └── internal/session
        ├── internal/tools
        │     ├── internal/skills
        │     └── internal/platform
        ├── internal/compaction
        │     ├── internal/session
        │     ├── internal/provider
        │     └── internal/config
        ├── internal/provider
        │     ├── internal/config
        │     ├── internal/session
        │     ├── internal/tools
        │     └── internal/usage
        ├── internal/llmcall
        │     ├── internal/config
        │     ├── internal/provider
        │     ├── internal/session
        │     └── internal/tools
        └── internal/agents
              ├── internal/config
              └── internal/tools
```

Dependencies are acyclic. No circular imports. The dependency rule is: `main` → everything,
`runtime` depends on most internal packages, `prompt` depends on `skills`, `helpers`, `agents`,
and `session`. `provider` depends on `config`, `session`, `tools`, and `usage`.

## Layer Stack

### Layer 0 — Application Entry (`main.go`)
- Single entry point (no `cmd/` tree)
- Flag parsing: `-c` (continue last clean), `-r` (resume last), `--console`, `--telegram <instance>`
- Startup sequence: detect OS → bootstrap app home → load/first-run config → prepare builtin assets → load agent definitions → create/resume session → agent → transport
- Two transport entry points: `runConsole()` or `telegram.Run()`
- Embedded assets via `//go:embed` directives in `embed.go`

### Layer 1 — Agent Core (`internal/runtime/`)
Three files:
- `runtime.go` — Handler interface, Agent struct, NewAgent, RunTurn, model switching, command handlers
- `agent_orchestration.go` — One-shot child-agent execution, persistent sessions, dual timeout, activity forwarding
- `mode_capabilities.go` — Mode tool deny-list and sub-agent allow-list enforcement

### Layer 2 — Tools (`internal/tools/`)
- `Tool` interface — 5 methods: Name, Description, Parameters, Execute, FormatArgs
- `Registry` — map[string]Tool, built at agent construction, never modified at runtime
- All 9 base tools registered in `NewAgent()` plus conditional `run_agent`:
  - `shell` — command execution via platform shell
  - `load_skill` — resolves a skill and returns its expanded body as a standard tool result
  - `replace_block` — exact text replacement in files
  - `ask_a_friend` — delegate to secondary model role
  - `analyze_image` — vision role image analysis
  - `task_read` / `task_write` — task tracking file I/O
  - `read_file` / `write_file` — file reading and writing (300KB read limit)
  - `run_agent` — one-shot sub-agent delegation (added only when valid agent definitions exist)
- `agent_done` — internal completion protocol for one-shot children (not a registered base tool)
- Default timeout: 60s per tool call
- `Registry.Clone()` and `Registry.Filter()` for mode policy and child agent tool selection

### Layer 3 — Transports

#### Console (`internal/console/`)
- Primary transport. Implements `runtime.Handler`
- CLI REPL with streaming output, colored labels, Markdown rendering
- TTY-only (no pipe/non-TTY support)
- Raw terminal mode via `golang.org/x/term`
- Ctrl-C aborts current turn, Ctrl-D cancels input, Tab cycles modes
- Renders child agent activity via `AgentActivityHandler` interface

#### Telegram (`internal/telegram/`)
- Secondary transport. Implements `runtime.Handler`
- Long-polling bot (`getUpdates`, no webhooks)
- One instance per bridge config: `blazeai --telegram <instance>`
- Instance config in `app_home/telegram/<instance>/bridge.json`
- Buffered streaming: flushes to Telegram every 500ms, splits messages >3500 chars
- Local slash commands: `/help`, `/start`, `/model`, `/clear`, `/new`, `/exit`
- Sudo always denied (no password channel over chat)

### Layer 4 — LLM Client (`internal/provider/`)
- `NewClient` resolves the selected `provider/model_name` through the user adapter catalog, then builtin model/provider contracts, and selects the declared protocol; missing or unsupported metadata stops client creation.
- Resolution precedence is user exact, user longest-prefix wildcard, builtin model exact/longest-prefix wildcard, builtin exact provider identity, then explicit no-match error. Builtins do not match endpoint aliases.
- `StreamWithPhase` creates the provider-neutral `Request`, validates it, lowers it through the selected `Protocol`, and only then enters the existing HTTP transport/parser path.
- `openAIChatProtocol` lowers Chat requests, conditionally includes stream usage, and removes reasoning history fields when its catalog variant disables them.
- `openAIResponsesProtocol` selects the normal or lite existing Responses builders; OAuth credential refresh, SSE parsing, tool-call extraction, usage reporting, cache identity, and Responses identity remain in the transport layer.
- The current catalog exposes two protocol families: OpenAI Chat for non-OAuth providers and OAuth-backed OpenAI Responses. There is no model-name prefix protocol routing or automatic catalog inference.

### Layer 5 — Prompt Assembly (`internal/prompt/`)
- `Builder` struct with `PromptsFS` (immutable embedded prompt templates) and `BuiltinSkillsFS` (immutable embedded builtin skills)
- `Builder.Build()` and `BuildRuntimePart()` methods
- Prompt templates reread from the immutable embedded FS on every LLM call; disk-backed project context (specs.md, AGENTS.md) and user skills are also refreshed; nothing is cached or reused
- Build order: universal sysprompt → OS sysprompt → transport prompt → host helpers → skills → agents → specs.md → AGENTS.md → conversation history
- Variable injection: `{APP_HOME}`, `{WORK_DIR}`, `{OS_INFO}`, `{SKILLS_AVAILABLE}`, `{AGENTS_CONTENT}`, `{PROJECT_CONTENT}`, `{HOST_HELPERS_*}`, `{TRANSPORT_CONTEXT}`, `{SKILL_DIR}`
- Agent definitions inject `{AGENT_INSTRUCTIONS}` and `{AGENT_TASK}` into `sysprompt.agent.md`

### Layer 6 — Supporting Packages

#### Config (`internal/config/`)
 - `config.json` — providers, favorite_models, roles, compaction, stripReasoning, last_model, helperSetup, debugPrompt
 - `model_adapters.json` — user adapter definitions under `adapters`; builtin contracts remain embedded
- `modes.json` (separate file) — work modes with name/model/directive/denied_tools/agents, last_mode
 - Adapter catalog is loaded and saved separately; legacy `config.json.models` is migrated once when the separate file is absent, while a dual-source conflict stops loading
- Known conflict: `modes.go` falls back to `DefaultMode()` when `modes.json` is missing/invalid (violates no-fallback spec)

#### Session (`internal/session/`)
- File-based: `session.json` in random folder under `app_home/projects/<project>/sessions/`
- Full message array exactly as sent to/received from LLM
- Reasoning parts preserved intact on disk
- `closed_cleanly` boolean — set true only on `/exit`
- `Sanitize()` strips secrets from messages
- `SanitizeMessages()` enforces tool-call/result pairing

#### Skills (`internal/skills/`)
- Markdown parsing: required `[DESCRIPTION]` and `[BODY]`
- Three scopes: builtin (embedded, canonical ID `builtin/<name>`), global (app_home/skills/, canonical ID `global/<name>`), project (project skills/, canonical ID `project/<name>`)
- Reserved builtin names `skill-manager`, `config-manager`, and `audit-manager` always take priority; same-name global/project files are ignored (filtered from the discovered map)
- No active state; loaded bodies are ordinary persisted tool messages
- `DiscoverAll(workDir, builtinFS)` — discovers embedded builtins, then disk global/project skills, filters collisions, returns map[id → Skill]
- `Resolve()` — resolves name to scoped ID, errors on ambiguity

#### Agents (`internal/agents/`)
- Markdown definition files with `---` front matter
- Required: `name`, `description`, `kind: "one-shot"`, `tools` (explicit allowlist)
- Optional: `model`, `timeout`
- `Load()` discovers and validates definitions from `app_home/agents/`
- Strict validation: unknown tools rejected, empty allowlists rejected, duplicate names rejected
- `KindOneShot` is the only supported agent kind

#### Compaction (`internal/compaction/`)
- Triggered on `usage.prompt_tokens >= maxContextTokens`
- Pruning with tool boundary safety (never split tool_call ↔ tool result)
- Summarization via `default` or `summarization` role model
- Summary files in `session_folder/summaries/000001.md` (chronological, trimmed to maxSummaryFiles)
- Reasoning stripping: parts replaced with empty text in LLM payload; newest N preserved; session JSON untouched

#### Platform (`internal/platform/`)
- OS detection: Linux, Darwin, Windows
- App home resolution: `$HOME/blazeai`
- Bootstrap: create standard subfolder tree (including `agents/`)
- Shell selection per OS
- `ProjectDir()` resolves project dir from work folder
- App home README seeding into subfolders

#### Helpers (`internal/helpers/`)
- Live binary detection via `exec.LookPath`
- Core helpers: rg, fd, jq, git, xh, pandoc, sqlite3
- `Detect()` runs all lookups, returns status list
- `Available()` filters detected and project-relevant helpers
- `MissingCore()` filters helpers not on PATH

## Data Flow — One Conversation Turn

```
User Input
    │
    ▼
Transport (console/telegram)
    │  User message appended to session
    ▼
Agent.RunTurn(ctx, userInput)
    │
    ├─ 1. Append user message to session
    │
    └─ for {  (tool call loop)
         │
         ├─ 2. sanitizeSession() → drop invalid tool rounds
         ├─ 3. Builder.Build(session, activeSkills)
         │       ├─ Read universal sysprompt (fs.FS)
         │       ├─ Read OS sysprompt (fs.FS)
         │       ├─ Read transport prompt (fs.FS)
         │       ├─ Detect host helpers (exec.LookPath)
         │       ├─ Discover skills (filesystem scan)
         │       ├─ Read specs.md (optional)
         │       ├─ Read AGENTS.md (optional)
         │       ├─ Inject variables → runtime prompt part
         │       └─ Prepend as system message + session messages → []Message
         │
         ├─ 4. Compactor.StripReasoningFromPayload(messages)
         │       └─ Replace old reasoning parts with empty text
         │
         ├─ 5. Write prompt.json only when config.debugPrompt is true
         ├─ 6. injectDirective(messages, mode.Directive)  → volatile mode injection
         │
          ├─ 7. Provider.Stream(ctx, messages, tools, onContent, onReasoning)
          │       ├─ build provider-neutral Request from selected model metadata
          │       ├─ validate + lower through the selected protocol adapter
          │       └─ existing Chat or OAuth Responses SSE stream → Handler callbacks
         │
         ├─ 8. Extract assistant message (content + tool_calls + reasoning)
         ├─ 9. Append assistant message to session
         ├─ 10. Handler.OnUsage(promptTokens, cachedTokens, uncachedTokens)
         │
         ├─ 11. For each tool call:
         │       ├─ Handler.OnToolCall(name, formattedArgs)
         │       ├─ tools.Registry.Execute(name, args)
         │       ├─ Handler.OnToolResult(name, result)
         │       └─ Append tool result to session
         │
         ├─ 12. Compactor.Compact(session, usage)? → check after tool execution
         └─ }  (loop back — tool results are now in session history)
              If LLM produced no tool calls → turn done
```

## Startup Sequence

```
main()
  │
  ├─ flag.Parse()  → -c, -r, --telegram
  ├─ platform.Detect()  → Linux / Darwin / Windows
  ├─ platform.Bootstrap()  → create app home dirs (including agents/)
  ├─ config.NeedsFirstRun()?
  │    ├─ yes → runFirstRun()  → interactive config
  │    └─ no  → config.Load()  → from config.json
  ├─ prepareBuiltinAssets()  → resolve both immutable embedded filesystems with no app-home writes
  │
  ├─ --telegram set?
  │    └─ yes → telegram.Run(ctx, cfg, osType, promptsFS, builtinSkillsFS, instance)
  │
  ├─ os.Getwd()  → workDir
  ├─ session.Create() / session.LastClean() / session.Last()
  ├─ agents.Load(appHome/agents, registry)  → agent definitions
  ├─ runtime.NewAgent(cfg, sess, osType, promptsFS, builtinSkillsFS, workDir, handler=nil, transportName="console")
  │    ├─ Migrate modes from config.json if present
  │    ├─ Load modes (modes.json) or create default
  │    ├─ Resolve active mode: lastMode > firstMode
   │    ├─ Create provider client; selected user/builtin adapter metadata is required
   │    ├─ Create summarization provider client with its own metadata (if separate role)
  │    ├─ Create prompt.Builder
  │    ├─ Create compaction.Manager
  │    ├─ Build tool registry (12 base tools + conditional run_agent)
  │    ├─ Load agent definitions
  │    ├─ Apply mode capabilities (deny tools, filter agents)
  │    └─ Return Agent
  │
  ├─ Compactor.RebuildForResume() if -c or -r
  ├─ console.NewConsole(agent)
  ├─ agent.Handler = console
  └─ cons.Run()
```

## Key Design Decisions

### Multi-File Runtime
The agent core is split across three files: `runtime.go` (core loop and handler),
`agent_orchestration.go` (one-shot child execution), and `mode_capabilities.go`
(mode policy enforcement). Keeps concerns separated while maintaining visibility.

### Transport-Agnostic Core
The `Handler` interface is the only boundary. Transports know nothing about each other.
Commands with distinct transport UX (console vs Telegram) are handled at the
transport level, not shared. Optional extensions (`AgentActivityHandler`,
`StreamPhaseHandler`) allow richer transport behavior without polluting the core contract.

### Prompt-First Design
Most agent behavior is shaped by prompt templates, not runtime logic. Embedded prompt templates are reread from the immutable embedded FS on every LLM call, while `specs.md`, `AGENTS.md`, and user skills are rediscovered/read from disk. All variables are injected at
build time. There is no cached or reused prompt state.

### No Registry or Factory Overhead
Transport selection in main.go is a plain if-branch (`--telegram` flag), not a
transport registry or factory. `main.go` startup helpers are plain functions, not
a bootstrap package. Keeps complexity low.

### Minimal External Dependencies
Standard library first; direct external dependencies are `github.com/reeflective/readline`, `golang.org/x/image`, and `golang.org/x/term`; indirect dependencies are `github.com/rivo/uniseg` and `golang.org/x/sys`.

### Skills Replace Memory
No separate memory subsystem. Skill bodies may contain instructions and reference data; loaded bodies follow normal conversation-history and compaction behavior.

### Model Switching Paths
- `SetModel()` — transport model switch with global persistence (config.json + modes.json)
- `SetModelLocal()` — transport-local switch without global persistence
- `SetMode()` — switch work mode with model resolution and mode capabilities refresh
- All three go through `applyModel()` which syncs the compactor and provider client
 - `provider.NewClient()` re-resolves the user/builtin adapter and protocol on every applied model; missing or unsupported metadata is an explicit error.
 - `applyModel()` retries a missing adapter once after `Config.ReloadModelAdapters()`; a failed reload or second missing match preserves the current runtime model/provider state.
