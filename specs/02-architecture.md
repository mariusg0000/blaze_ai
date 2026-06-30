# Architecture

## Source Files

| Package | File(s) | Role |
|---------|---------|------|
| `main.go` | `main.go`, `embed.go` | Entry point, CLI flags, wiring, embedded assets |
| `internal/runtime/` | `runtime.go` | Agent core, Handler interface, RunTurn loop, model resolution, helper detection |
| `internal/console/` | `console.go`, `reader.go` | Console transport (REPL, rendering, input) |
| `internal/telegram/` | `telegram.go`, `handler.go`, `commands.go`, `config.go`, `state.go` | Telegram bridge transport (polling, message handling) |
| `internal/tools/` | `tools.go`, `shell.go`, `skill_tools.go`, `replace_block.go`, `ask_friend.go`, `task_tools.go` | Native tool interface and implementations |
| `internal/llmcall/` | `client.go` | OpenAI-compatible streaming chat completion client |
| `internal/config/` | `config.go`, `modes.go` | Configuration load/save/validate, work mode management |
| `internal/session/` | `session.go` | File-based session persistence |
| `internal/skills/` | `skills.go` | Skill parsing, discovery, active list management |
| `internal/prompt/` | `prompt.go` | Prompt assembly from disk sources |
| `internal/compaction/` | `compaction.go` | Context compaction, pruning, summarization, reasoning stripping |
| `internal/platform/` | `platform.go` | OS detection, app home, bootstrap, shell selection |
| `internal/helpers/` | `helpers.go` | Host helper detection (rg, fd, jq, git, etc.) |
| `internal/provider/` | (provider package) | LLM provider client creation from config |
| `prompts/` | `sysprompt.md`, `sysprompt.linux.md`, `sysprompt.darwin.md`, `sysprompt.windows.md` | System prompt templates (embedded in binary) |
| `skills/` | `skill-manager.md`, `customize-me.md`, `session-retrospective.md`, `specs-manager.md` | Builtin skill templates (embedded, seeded to app_home) |

## Module Dependency Graph

```
main.go
  ├── internal/platform       (OS, app home, bootstrap)
  ├── internal/config         (config load/save, first-run detection)
  ├── internal/session        (session create/resume)
  ├── internal/skills         (skill seeding)
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
        │     └── internal/session
        ├── internal/tools
        │     ├── internal/skills
        │     └── internal/platform
        ├── internal/compaction
        │     ├── internal/session
        │     ├── internal/provider
        │     └── internal/config
        └── internal/llmcall
              └── internal/provider
```

Dependencies are acyclic. No circular imports. The dependency rule is: `main` → everything,
`runtime` depends on most internal packages, `prompt` is the leaf-most package with no
runtime dependency.

## Layer Stack

### Layer 0 — Application Entry (`main.go`)
- Single entry point (no `cmd/` tree)
- Flag parsing: `-c` (continue last clean), `-r` (resume last), `--telegram <instance>`
- Startup sequence: detect OS → bootstrap app home → load/first-run config → prepare builtin assets → create/resume session → agent → transport
- Two transport entry points: `runConsole()` or `telegram.Run()`
- Embedded assets via `//go:embed` directives in `embed.go`

### Layer 1 — Agent Core (`internal/runtime/`)
Single file `runtime.go` containing:
- `Handler` interface — the only contract between agent core and transports
- `Agent` struct — holds all runtime state: config, session, skills, tools, prompt builder, provider client, handler
- `NewAgent()` — constructs the agent with all dependencies wired. Tool registry built here with all 8 tools
- `RunTurn()` — one conversation turn: build prompt → stream LLM → execute tool calls → loop
- `SetModel()`, `SetModelLocal()`, `SetMode()` — model switching with compactor sync
- HandleXxx methods for slash commands (`/exit`, `/model`, `/cd`)

### Layer 2 — Tools (`internal/tools/`)
- `Tool` interface — 5 methods: Name, Description, Parameters, Execute, FormatArgs
- `Registry` — map[string]Tool, built at agent construction, never modified at runtime
- All 8 tools registered in `NewAgent()`:
  - `shell` — command execution via platform shell
  - `load_skill` / `unload_skill` — in-memory active skills management
  - `run_skill` — execute runnable skill [CODE] sections
  - `replace_block` — exact text replacement in files
  - `ask_a_friend` — delegate to secondary model role
  - `task_read` / `task_write` — task tracking file I/O
- Default timeout: 60s per tool call

### Layer 3 — Transports

#### Console (`internal/console/`)
- Primary transport. Implements `runtime.Handler`
- CLI REPL with streaming output, colored labels, Markdown rendering
- TTY-only (no pipe/non-TTY support)
- Raw terminal mode via `golang.org/x/term`
- Ctrl-C aborts current turn, Ctrl-D cancels input, Tab cycles modes

#### Telegram (`internal/telegram/`)
- Secondary transport. Implements `runtime.Handler`
- Long-polling bot (`getUpdates`, no webhooks)
- One instance per bridge config: `blazeai --telegram <instance>`
- Instance config in `app_home/telegram/<instance>/bridge.json`
- Buffered streaming: flushes to Telegram every 500ms, splits messages >3500 chars
- Local slash commands: `/help`, `/start`, `/model`, `/clear`, `/new`, `/exit`

### Layer 4 — LLM Client (`internal/llmcall/`)
- OpenAI-compatible chat completion API client
- Streaming support via SSE parsing
- Tool call extraction from response
- Usage token reporting
- Reasoning/thinking token extraction
- No model-specific logic — works with any OpenAI-compatible endpoint

### Layer 5 — Prompt Assembly (`internal/prompt/`)
- `Builder` struct with `Build()` and `BuildRuntimePart()` methods
- Prompt rebuilt on every LLM call from disk sources
- Build order: universal sysprompt → OS sysprompt → host helpers → skills (available + active) → specs.md → AGENTS.md → conversation history
- Variable injection: `{APP_HOME}`, `{WORK_DIR}`, `{OS_INFO}`, `{SKILLS_AVAILABLE}`, `{SKILLS_ACTIVE}`, `{RUNNABLE_SKILLS_SECTION}`, `{AGENTS_CONTENT}`, `{PROJECT_CONTENT}`, `{HOST_HELPERS_*}`, `{TRANSPORT_CONTEXT}`, `{SKILL_DIR}`

### Layer 6 — Supporting Packages

#### Config (`internal/config/`)
- `config.json` — providers, favorite_models, roles, compaction, stripReasoning, lastModel, helperSetup, showReasoning
- `modes.json` (separate file) — work modes with name/model/directive, last_mode
- Atomic saves with corruption fallback on modes
- Migration from legacy config-embedded modes

#### Session (`internal/session/`)
- File-based: `session.json` in random folder under `app_home/sessions/` or project dir
- Full message array exactly as sent to/received from LLM
- Reasoning parts preserved intact on disk
- `closed_cleanly` boolean — set true only on `/exit`
- Project-scoped sessions: `app_home/projects/<hash>/sessions/<random>/session.json`
- `Sanitize()` strips secrets from messages

#### Skills (`internal/skills/`)
- Markdown parsing: `[DESCRIPTION]`, `[BEHAVIOR]`, `[DATA]`, `[SYNTAX]`, `[CODE]`, `[CODE_ERROR]`
- Three scopes: builtin (embedded), global (app_home/skills/), project (.blazeai/skills/)
- `ActiveList` — in-memory, starts empty, not persisted
- `DiscoverAll()` — reads all three scopes, returns map[id → Skill]
- `Resolve()` — resolves name to scoped ID, errors on ambiguity

#### Compaction (`internal/compaction/`)
- Triggered on `usage.prompt_tokens >= maxContextTokens`
- Pruning with tool boundary safety (never split tool_call ↔ tool result)
- Summarization via `default` or `summarization` role model
- Summary files in `session_folder/summaries/000001.md` (chronological, trimmed to maxSummaryFiles)
- Reasoning stripping: parts replaced with empty text in LLM payload; newest N preserved; session JSON untouched

#### Platform (`internal/platform/`)
- OS detection: Linux, Darwin, Windows
- App home resolution: `$HOME/blazeai`
- Bootstrap: create standard subfolder tree
- Shell selection per OS
- `ProjectDir()` resolves project dir from work folder

#### Helpers (`internal/helpers/`)
- Live binary detection via `exec.LookPath`
- Core helpers: rg, fd, jq, git, curl, pandoc, sqlite3
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
    ├─ 1. Sanitize session (strip secrets)
    ├─ 2. Builder.Build(session, activeSkills)
    │       ├─ Read universal sysprompt (fs.FS)
    │       ├─ Read OS sysprompt (fs.FS)
    │       ├─ Detect host helpers (exec.LookPath)
    │       ├─ Discover skills (filesystem scan)
    │       ├─ Read specs.md (optional)
    │       ├─ Read AGENTS.md (optional)
    │       ├─ Inject variables → runtime prompt part
    │       └─ Prepend as system message + session messages → []Message
    │
    ├─ 3. Compactor.StripReasoningFromPayload(messages)
    │       └─ Replace old reasoning parts with empty text
    │
    ├─ 4. Write prompt.json to session folder (debug artifact)
    ├─ 5. Inject mode directive into latest user message
    │
    ├─ 6. Provider.Stream(ctx, messages, tools, onContent, onReasoning)
    │       └─ SSE stream → Handler.OnContent() / Handler.OnReasoning()
    │
    ├─ 7. Extract assistant message (content + tool_calls + reasoning)
    ├─ 8. Append assistant message to session
    ├─ 9. Handler.OnUsage(promptTokens)
    │
    ├─ 10. For each tool call:
    │       ├─ Handler.OnToolCall(name, args)
    │       ├─ tools.Registry.Execute(name, args)
    │       ├─ Handler.OnToolResult(name, result)
    │       └─ Append tool result to session
    │
    ├─ 11. Compactor.ShouldCompact(usage)?
    │       └─ If yes: prune + summarize + inject synthetic
    │
    └─ 12. Loop: if LLM produced tool_calls AND any result has content → back to step 2
         If LLM produced content (no tool_calls) → turn done
```

## Startup Sequence

```
main()
  │
  ├─ flag.Parse()  → -c, -r, --telegram
  ├─ platformOS()  → Linux / Darwin / Windows
  ├─ platform.Bootstrap()  → create app home dirs
  ├─ config.NeedsFirstRun()?
  │    ├─ yes → runFirstRun()  → interactive config
  │    └─ no  → config.Load()  → from config.json
  ├─ prepareBuiltinAssets()  → init embedded FS, seed skills
  │
  ├─ --telegram set?
  │    └─ yes → telegram.Run(ctx, cfg, osType, promptsFS, instance)
  │
  ├─ os.Getwd()  → workDir
  ├─ session.Create() / session.LastClean() / session.Last()
  ├─ runtime.NewAgent(cfg, sess, osType, promptsFS, workDir, handler=nil, transportName="console")
  │    ├─ Load modes (modes.json) or create default
  │    ├─ Resolve model: mode model > last_mode > roles.default
  │    ├─ Create provider client
  │    ├─ Create summarization provider client (if separate role)
  │    ├─ Create prompt.Builder
  │    ├─ Create compaction.Manager
  │    ├─ Build tool registry (8 tools)
  │    └─ Require a transport prompt via Builder.TransportName
  │    └─ Return Agent
  │
  ├─ Compactor.RebuildForResume() if -c or -r
  ├─ console.NewConsole(agent)
  ├─ agent.Handler = console
  └─ cons.Run()
```

## Key Design Decisions

### Single-file Runtime
The entire agent core (Agent, Handler, RunTurn, model switching, command handlers,
helper detection) is in `internal/runtime/runtime.go`. No separate agent.go,
handler.go, or turn_loop.go files. Keeps internal coupling visible in one place.

### Transport-Agnostic Core
The `Handler` interface is the only boundary. Transports know nothing about each other.
Commands with distinct transport UX (console vs Telegram vs web) are handled at the
transport level, not shared.

### Prompt-First Design
Most agent behavior is shaped by prompt templates, not runtime logic. The prompt
system prompt is rebuilt from disk every LLM call. All variables are injected at
build time. There is no cached or reused prompt state.

### No Registry or Factory Overhead
Transport selection in main.go is a plain if-branch (`--telegram` flag), not a
transport registry or factory. `main.go` startup helpers are plain functions, not
a bootstrap package. Keeps complexity low.

### Two External Dependencies
Only `golang.org/x/sys` (OS primitives) and `golang.org/x/term` (raw terminal mode).
Everything else is Go standard library.

### Skills Replace Memory
No separate memory subsystem. Skills with `[DATA]` sections provide persistent
knowledge storage. Skills with `[BEHAVIOR]` sections extend agent behavior.
Skills with `[CODE]` sections provide dynamic tool-like extensibility via
`run_skill`.

### Model Switching Paths
- `SetModel()` — transport model switch with global persistence (config.json + modes.json)
- `SetModelLocal()` — transport-local switch without persistence (Telegram only)
- `SetMode()` — switch work mode with model resolution
- All three go through `applyModel()` which syncs the compactor and provider client
