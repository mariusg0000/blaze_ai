# Project Specs

========== USER SPECS ==========

!! IMPORTANT MANDATORY !! Do not implement fallbacks. If something is missing or not configured, app must stop with a relevant error message !!! A fallback must be considered as a silent critical error !

========== AGENT GENERATED ==========

### Purpose
- BlazeAI is a fast, cross-platform AI terminal agent for experienced users.
- Optimized for direct command execution, low interaction overhead, and maximum flexibility.
- Shell-native execution model with a compiled Go backend.
- Same agent core supports a console interface now and a web terminal interface later.

### Active Scope
- Greenfield rebuild from specification. Previous implementation is considered polluted and discarded.
- Five specification files drive the rebuild: `specs/01-product-scope.md`, `specs/02-core-runtime.md`, `specs/03-interfaces.md`, `specs/04-platform-ops.md`, `specs/05-compaction.md`.
- Console transport is the first and complete implementation target. Web transport is postponed.
- Product priorities in order: speed of interaction, simplicity of execution, cross-platform behavior.

### Target Users
- Technical and experienced users familiar with terminals, shell commands, and LLM workflows.
- No beginner wizards or guided flows.

### Interaction Model
- Main interface: simple CLI REPL, not a full-screen TUI.
- Web interface: postponed, will imitate a terminal session over the same handler contract.
- Console UX: Markdown rendering, colored and bold labels, visual separators, streaming output, thinking spinner.
- Handler contract between agent core and transports: `OnContent`, `OnToolCall`, `OnToolResult`.

### Execution Model
- Main tool: `shell`.
- Inline shell for simple tasks, OS-native scripts for complex tasks.
- Python is last resort only, in a lazily-created venv under `app_home/scripts/venv`.
- Four native tools: `shell`, `load_skill`, `unload_skill`, `replace_block`.
- OpenAI-compatible tool calling, multi tool call per turn, optional `timeout` parameter per call.
- Default tool timeout: 60s. Timeout returns `timeout <N>s exceeded`.

### Provider And Model Configuration
- OpenAI-compatible providers only.
- Provider definition: `name`, `endpoint`, `api_key`.
- API keys stored in `config.json`, not in `.env`.
- Models predefined in config, canonical format `provider/model_name`.
- Three model roles: `default` (required), `vision` (optional), `summarization` (optional).
- No fallback providers or models. Errors stop the runtime with clear messages.
- Last selected model persists across sessions in config.
- Runtime command `/model` sets current model. No `/provider` command.

### Config Location
- `app_home/config/config.json` — single source of truth.
- Contains: providers, favorite models, role assignments, API keys, compaction thresholds, reasoning strip settings.
- Pre-filled at first-run setup with defaults.

### First-Run Setup
- Triggers when config missing or `default` role unassigned.
- Interactive console: curated list of max 15 known providers + custom option.
- After provider selection: API key entry.
- After API key: model retrieval from endpoint, selection by number.
- Role assignment for `default`; `vision` and `summarization` optional.
- Builtin skill `customize_me` allows LLM-assisted configuration.

### App Home Bootstrap
- Resolved from OS home directory + `blazeai` folder.
- Created at first start if missing.
- Standard subfolders: `skills`, `scripts`, `scripts/venv` (lazy), `backups`, `sessions`, `memory`, `config`.
- `{APP_HOME}` variable injected into prompts and skills at build time.

### Prompt Build
- Rebuilt on every LLM call from disk. Nothing reused.
- Runtime part order: universal sysprompt → OS sysprompt → `AGENTS.md` (if exists in work folder) → `memory.md` → skills section.
- Skills section: available skills (all `[DESCRIPTION]` blocks + file names) then active skills (`[DETAILS]` of loaded skills).
- Conversation part: persisted message history from session JSON, appended after runtime part.
- Required sources: universal prompt, OS prompt. Optional: `AGENTS.md`, `memory.md`, skills.

### Skills
- Format: Markdown with fixed sections `[DESCRIPTION]` and `[DETAILS]`. Invalid without both.
- Discovery: builtin `skills/` in project distribution + custom `app_home/skills/`. Both read every build.
- Collision: `skill-manager` forbids duplicates. If collision exists, custom wins.
- Builtin skills: `memory-manager`, `skill-manager`, `customize_me`.
- Active skills: in-memory list of names, starts empty per session, not persisted, not deduced from history.
- `load_skill` / `unload_skill` only modify the in-memory list.

### Memory
- Single file: `app_home/memory/memory.md`.
- Read fresh every prompt build.
- Updated explicitly by agent via `shell` or user action. No automatic runtime writes.

### Sessions
- No database. File-based storage in `app_home/sessions/`.
- New session: random folder name, `session.json` with full message array exactly as sent to LLM.
- `session.json` includes `closed_cleanly` boolean, set `true` only on `/exit`.
- New session by default. `-c` continues last cleanly closed session.
- Active skills list and runtime prompt part are not persisted in session.

### Slash Commands
- `/exit`: clean close, mark `closed_cleanly=true`, exit.
- `/model`: without arg prints favorite models list; with arg sets `provider/model_name`.
- `/cd`: change work folder. Invalid path → clear error, keep current.
- Unknown slash commands: passed to LLM as normal user messages.

### Context Compaction
- Trigger: provider-reported `usage.prompt_tokens` reaches `maxContextTokens`.
- Thresholds in config, pre-filled at init: `maxContextTokens=100000`, `minContextTokens=50000`, `summaryMaxTokens=2000`, `maxSummaryFiles=10`, `tokenCoefficient=3.5`, `maxBackoffOffsetTokens=25000`.
- Hard cap: `maxContextTokens` + `maxBackoffOffsetTokens`. Above hard cap, prune forced without summary if summarization fails.
- Pruning: cut point by local estimator, tool boundary safety (no split between tool call and result), physical deletion from session JSON.
- No `summarizedIDs`, no separate state file. Session JSON is the source of truth.
- Summaries: `session_folder/summaries/000001.md`, chronological, trimmed to `maxSummaryFiles`.
- Summary injected as synthetic message prepended to session JSON.
- Summarization model: `default` role. Prompt adapted from auto-compress plugin.
- On `-c`: summaries loaded automatically, synthetic message rebuilt.

### Reasoning Stripping
- Config: `stripReasoning.enable=true`, `stripReasoning.preserveLast=5` by default.
- Session JSON on disk: reasoning parts intact, never modified.
- Payload to LLM: reasoning parts replaced with empty text, only newest N kept. Count is global.
- Summary transcript: reasoning included as `[REASONING]...[/REASONING]` only for newest N from pruned segment.
- Cut point estimation: stripped reasoning parts count as 0 tokens.

### Build Toolchain
- Go `toolchain` directive in `go.mod`, `GOTOOLCHAIN=auto`.
- No `.tools/` directory in repository.
- Bootstrap minimum: Go 1.21+ on system. Toolchain auto-downloaded on first build.
- `CGO_ENABLED=0` default release strategy.
- Release targets: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`.
- Linux: conservative targets, minimal libc dependency, validate on older system.

### Platform Rules
- Linux: `bash` → `sh`.
- macOS: `bash` → `sh`. `zsh` optional, not required.
- Windows: `pwsh` → `powershell.exe` → `cmd.exe`.
- Runtime and prompt aware of quoting, paths, env vars, command availability, script formats.

### Safety Rules
- Destructive commands: extreme care, narrow targets, verify first.
- Backups: LLM decision, not runtime-enforced. Stored in `app_home/backups/`.
- Privilege elevation: `sudo` / Run as Administrator only after explicit user approval. Password interactive, never in chat or session.
- Secrets never in session JSON or prompt text.

### Dependency Rules
- Prefer Go standard library.
- Avoid CGO, platform-native libraries, broad runtime assumptions.
- Avoid newer libc than necessary for supported Linux targets.

### High-Risk Areas
- Local shell execution is the main product risk.
- Cross-platform shell behavior differs materially.
- Linux build compatibility on older systems.
- Python venv portability across platforms is best-effort only.
- Context compaction correctness: tool boundary safety, reasoning stripping, summary quality.

### Working Rules
- Keep implementations direct and incremental.
- Shared agent core between console and web transports via handler contract.
- No fallbacks on configuration or model selection errors.
- Prompt behavior is a major source of product personality and control.
- Session folders persist on disk indefinitely. No automatic cleanup in this phase.
