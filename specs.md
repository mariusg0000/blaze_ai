# Project: BlazeAI

## Project and User Directives and Rules (Mandatory)

Follow every entry below as a binding project instruction.

- Do not implement fallbacks. If something is missing or not configured, the app must stop with a relevant error message. A fallback must be considered a silent critical error.
- Never overwrite `~/.local/bin/blazeai`. It is a bash wrapper script that compiles the project from source and runs the resulting binary. Never replace it with a compiled ELF binary, never `go build -o ~/.local/bin/blazeai`, never `cp ./blazeai ~/.local/bin/`, never `install ./blazeai ~/.local/bin/`. If the binary needs updating, use `go build -o /tmp/blazeai .` only.

## Overview

BlazeAI is a Go 1.25 terminal AI agent with console and opt-in Telegram transports. `main.go` bootstraps the platform, configuration, embedded prompts and skills, session, and selected transport; the runtime then streams OpenAI-compatible responses and dispatches native tools. Prompt content and Markdown definitions shape agent behavior, while file-backed sessions and project-scoped work directories provide continuity. The former desktop transports were removed; no active desktop implementation is part of the current repository.

## Operating Model

- `main.go` is the composition root: it detects the OS, bootstraps app home, loads or creates configuration, prepares embedded assets, selects Telegram when requested, and otherwise opens the console session.
- `internal/runtime/` owns turn execution, prompt construction, streaming, tool dispatch, session updates, compaction, modes, and child-agent orchestration. Providers supply the LLM stream; transports consume runtime callbacks.
- `internal/prompt/` rebuilds prompt input for each LLM call from embedded templates and disk-backed project/user sources. Optional project map and agent content are injected only where the prompt template provides their placeholders.
- `internal/config/`, `internal/platform/`, and `internal/session/` own startup prerequisites, host/project paths, validated configuration, and persistent session files. Required failures are surfaced rather than silently replaced.
- `internal/tools/` exposes the runtime tool registry. Shell execution remains host-native; direct `read_file` and `write_file` tools resolve relative paths through the work directory, as does `replace_block`.
- `internal/compaction/` limits context and preserves continuation state through chronological summaries, pruning, and reasoning handling. Telegram is a long-polling single-chat bridge with text/image intake and streamed activity adaptation.
- `prompts/` and builtin `skills/` are embedded at build time; user/project sources remain disk-backed. `decisions/` records accepted changes and rationale but does not override implemented behavior when code differs.

## Source Authority

Current source code, tests, schemas, and configuration define implemented behavior. `decisions/` is authoritative for recorded decisions, constraints, rationale, and committed change context. Existing specs are maintained context and must be checked against those sources; conflicts must be flagged rather than silently reconciled.

## Project Map

- `main.go` - application entrypoint and console/Telegram transport selection.
- `embed.go` - embeds builtin prompts and skills.
- `firstrun.go` - interactive initial provider, model, API key, and role setup.
- `go.mod` - Go module, toolchain, and dependency declarations.
- `internal/runtime/` - agent turn lifecycle, orchestration, modes, streaming, tools, and persistence coordination.
- `internal/agents/` - Markdown agent discovery, parsing, validation, and resolution.
- `internal/console/` - terminal REPL, commands, input, rendering, and streaming display.
- `internal/telegram/` - Telegram long polling, commands, single-chat state, images, and streaming adaptation.
- `internal/prompt/` - per-call prompt assembly and variable injection.
- `internal/config/` - configuration, provider/role/mode validation, persistence, and first-run detection.
- `internal/session/` - project-scoped file session persistence and resume lifecycle.
- `internal/compaction/` - context pruning, summarization, and reasoning cleanup.
- `internal/tools/` - native tool implementations and filtered registries.
- `internal/skills/` - skill discovery, parsing, validation, scoping, and active skills.
- `internal/platform/` - OS detection, shell selection, app-home bootstrap, and project paths.
- `internal/provider/` - OpenAI-compatible HTTP streaming and tool-call decoding.
- `internal/llmcall/` - secondary role-routed LLM calls.
- `internal/usage/` - provider-independent usage extraction and normalization.
- `internal/helpers/` - advisory host helper detection.
- `prompts/` - embedded system and transport prompt templates.
- `skills/` - embedded builtin skill definitions.
- `deploy_nas.sh` - Linux amd64 packaging and NAS deployment script.
- `specs/` - detailed subsystem specifications.
- `decisions/` - authoritative decision records and rationale.

## Task Routing

- Runtime turns, tool calls, modes, or child agents → `internal/runtime/` and the matching runtime/orchestration detailed spec.
- Prompt injection, project maps, or skill descriptions → `internal/prompt/`, `prompts/`, `internal/skills/`, and `specs/04-prompts.md` or `specs/09-skill-system.md`.
- Console interaction → `internal/console/` and `specs/13-console-ui.md`.
- Telegram behavior → `internal/telegram/` and `specs/14-telegram-bridge.md`.
- Configuration, startup, paths, or sessions → `internal/config/`, `internal/platform/`, `internal/session/`, and specs 03, 10, 16, or 17.
- Context limits and continuation summaries → `internal/compaction/` and `specs/11-context-compaction.md`.
- Accepted rationale or historical constraints → relevant files under `decisions/`.

## Detailed Specs

- `specs/01-product-scope.md`: product, interaction, execution, configuration, sessions, skills, agents, safety, release, non-goals
- `specs/02-architecture.md`: packages, layers, dependencies, startup, dataflow, boundaries, build, runtime, transport
- `specs/03-config-schema.md`: Config, JSON, providers, roles, modes, compaction, validation, defaults, persistence
- `specs/04-prompts.md`: prompt, templates, injection, variables, project-map, agents, skills, transports, system
- `specs/05-tools.md`: Tool, Registry, calls, dispatch, timeout, OpenAI, shell, files, agents, errors
- `specs/06-shell-execution.md`: shell, platform, commands, sudo, output, processes, timeout, errors, workdir
- `specs/07-file-editing.md`: replace_block, paths, workdir, matching, context, edits, precision, errors
- `specs/08-cross-model-delegation.md`: ask_a_friend, roles, secondary, models, routing, timeout, input, output
- `specs/09-skill-system.md`: skills, discovery, parsing, validation, scope, active, load_skill, builtin, project
- `specs/10-sessions.md`: sessions, resume, close, files, JSON, project, sanitization, lifecycle, persistence
- `specs/11-context-compaction.md`: compaction, pruning, summaries, chronology, reasoning, thresholds, task-switching, context
- `specs/12-handler-contract.md`: Handler, callbacks, streaming, phases, activity, extensions, contracts, runtime
- `specs/13-console-ui.md`: console, REPL, commands, readline, rendering, streaming, spinner, status, input
- `specs/14-telegram-bridge.md`: Telegram, polling, chat, commands, images, typing, streaming, retry, session
- `specs/15-runtime-core.md`: Agent, RunTurn, abort, sessions, modes, models, prompts, startup, lifecycle
- `specs/16-first-run.md`: first-run, provider, API key, model, roles, setup, config, validation, prompts
- `specs/17-platform.md`: OS, shell, app-home, paths, project, bootstrap, Windows, Linux, environment
- `specs/18-safety.md`: sudo, password, secrets, Telegram, approval, filtering, safety, errors, handling
- `specs/19-build-deploy.md`: Go, build, cross-compile, installer, NAS, linux, amd64, release, deployment
- `specs/20-agent-orchestration.md`: agents, frontmatter, child, sessions, agent_done, parallel, timeout, cleanup, activity
- `specs/21-mode-capabilities.md`: Mode, denied_tools, agents, allow-list, refresh, permissions, capabilities, configuration
- `specs/22-usage-normalization.md`: Usage, tokens, providers, normalization, cache, extraction, raw, input, output
