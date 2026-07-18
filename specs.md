# Project: BlazeAI

## Project Directives and Rules (Mandatory)

Follow every entry below as a binding project instruction.

- Do not implement fallbacks. If something is missing or not configured, the app must stop with a relevant error message. A fallback must be considered a silent critical error.
- Never overwrite `~/.local/bin/blazeai`. It is a bash wrapper script that compiles the project from source and runs the resulting binary. Never replace it with a compiled ELF binary, never `go build -o ~/.local/bin/blazeai`, never `cp ./blazeai ~/.local/bin/`, never `install ./blazeai ~/.local/bin/`. If the binary needs updating, use `go build -o /tmp/blazeai .` only.

## Overview

BlazeAI is a cross-platform AI terminal agent for experienced users, implemented as a Go module (`go 1.25.0`, `toolchain go1.26.4`). The system is a greenfield rebuild with a shell-native runtime, console and Telegram transports, OpenAI-compatible HTTP streaming, and direct shell execution on the host OS. It favors fast interaction, low overhead, and explicit failure over silent degradation. The primary transport is a terminal REPL; Telegram is a long-polling bridge. Agent behavior is shaped by system prompt files and Markdown agent definitions, not runtime logic.

## Source Authority

Current source code, tests, and config define implemented behavior. `decisions/` is authoritative for recorded decisions and rationale. `AGENTS.md` defines working rules. No Wiki is present.

## Project Map

- `main.go` - CLI entrypoint. Parses flags, bootstraps app home, loads config or runs first-run, opens session, starts transport.
- `embed.go` - Embeds `prompts/` and `skills/` into the binary with `go:embed`.
- `firstrun.go` - Interactive first-run provider, API key, model, and role setup.
- `go.mod` - Module root, Go toolchain declaration, and dependency pins.
- `internal/runtime/` - Agent core: handler contract, RunTurn loop, prompt build, streaming, tool dispatch, persistence, compaction.
  - `runtime.go` - Handler interface, Agent struct, NewAgent, RunTurn, model/mode management.
  - `agent_orchestration.go` - One-shot child-agent execution, persistent child sessions, dual timeout, activity forwarding.
  - `mode_capabilities.go` - Mode tool deny-list and sub-agent allow-list enforcement.
- `internal/agents/` - Markdown agent definition discovery, parsing, validation, and resolution.
- `internal/console/` - Terminal REPL transport: raw input, slash commands, Markdown rendering, streaming output.
- `internal/telegram/` - Telegram bridge: long polling, single-chat enforcement, text/image handling, streaming adaptation.
- `internal/prompt/` - Rebuilds runtime prompt from disk sources on every LLM call, injects variables.
- `internal/config/` - Config and mode load/save/validate, provider/role resolution, first-run detection.
- `internal/session/` - File-based session persistence under project-scoped folders.
- `internal/compaction/` - Context compaction, pruning, summary management, reasoning stripping.
- `internal/tools/` - Native tool interface, registry, shell, skill tools, ask_a_friend, analyze_image, replace_block, task tools, read_file, write_file, run_agent, agent_done, filtered registries.
- `internal/skills/` - Skill discovery, parsing, validation, scoping, active list management.
- `internal/platform/` - OS detection, shell selection, app home bootstrap, project directory resolution.
- `internal/provider/` - OpenAI-compatible HTTP client, streaming, tool-call decoding, usage reporting.
- `internal/llmcall/` - One-shot secondary LLM calls routed by role.
- `internal/usage/` - Provider-agnostic token usage extraction and normalization.
- `internal/helpers/` - Host helper detection (rg, fd, jq, git, etc.).
- `prompts/` - Embedded system prompt and transport prompt templates.
- `skills/` - Builtin skill definitions seeded into app home on startup.
- `deploy_nas.sh` - Build linux/amd64, package self-contained installer, SCP to NAS, SSH install. Default target: `nas@192.168.0.104`. To deploy: `./deploy_nas.sh` or `./deploy_nas.sh user@host`.
- `specs/` - Detailed specification fragments for each subsystem.
- `decisions/` - Timestamped session decision summaries with rationale.

## Detailed Specs

- `specs/01-product-scope.md`: Product identity, priorities, interaction model, execution model, no-fallback rule, architecture overview, app home, configuration, sessions, skills, agents, safety, release targets, non-goals
- `specs/02-architecture.md`: Module dependency graph, layer stack, package roles, data flow, startup sequence, build constraints
- `specs/03-config-schema.md`: Config struct, JSON schema, provider model, roles, compaction, strip reasoning, modes, validation rules
- `specs/04-prompts.md`: Prompt assembly, system prompt structure, transport prompts, variable injection, skill descriptions, AGENTS section
- `specs/05-tools.md`: Tool interface, Registry, OpenAI format, tool call lifecycle, multi-call, timeout, conventions, emoji display, file tools, agent tools
- `specs/06-shell-execution.md`: Shell tool, platform shell selection, command execution, sudo pipeline, output cap, process groups
- `specs/07-file-editing.md`: replace_block tool, block matching, context lines, edit precision
- `specs/08-cross-model-delegation.md`: ask_a_friend tool, secondary model calls, role-based routing, timeout
- `specs/09-skill-system.md`: Skill format, discovery, scoping, load_skill tool, active list, body rendering
- `specs/10-sessions.md`: Session creation, resume, close, file layout, session.json structure, sanitization
- `specs/11-context-compaction.md`: Compaction triggers, pruning, summarization, reasoning stripping, summary files
- `specs/12-handler-contract.md`: Handler interface, optional extensions, StreamPhaseHandler, AgentActivityHandler, contract rules
- `specs/13-console-ui.md`: Console struct, REPL loop, rendering, slash commands, input handling, spinner, status bar
- `specs/14-telegram-bridge.md`: Telegram transport, long polling, single-chat, text/image handling, commands
- `specs/15-runtime-core.md`: Agent struct, RunTurn loop, abort handling, session sanitization, mode injection, model management, startup wiring
- `specs/16-first-run.md`: First-run detection, interactive setup flow, provider selection, API key, model, roles
- `specs/17-platform.md`: OS detection, shell chain, app home bootstrap, project directory resolution
- `specs/18-safety.md`: Sudo approval, password handling, secrets policy, Telegram safety
- `specs/19-build-deploy.md`: Build targets, cross-compilation, deployment, release artifacts
- `specs/20-agent-orchestration.md`: Agent definition format, front matter, one-shot execution, child sessions, agent_done protocol, parallel execution, cleanup
- `specs/21-mode-capabilities.md`: Mode struct, denied_tools, agents allow-list, refreshModeCapabilities, modeAllowsAgent, example configurations
- `specs/22-usage-normalization.md`: Usage struct, raw variants, provider normalization, cache status, extraction rules
