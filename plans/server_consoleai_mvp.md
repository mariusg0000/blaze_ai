# Server + ConsoleAI MVP Plan

## Goal

Build the first distributed BlazeAI runtime with a strict split:

- `BlazeAI Server` owns config, provider calls, prompt build, sessions, compaction, and the LLM tool loop.
- `ConsoleAI` owns terminal UI, local cwd, sudo approval, and local tool execution.
- Server never executes user-facing tools.
- One client per session.
- Missing config, invalid modes, disconnected client, or unavailable tool must fail explicitly. No fallback.

## Scope

Initial remote tools only:

- `console_shell`
- `console_read_file`
- `console_write_file`
- `console_replace_block`

Out of scope for the MVP:

- multi-client sessions
- Telegram tool execution
- Docker packaging
- project-skill execution redesign
- desktop, Android, or web clients

## Current Findings

- `internal/runtime/runtime.go` still registers and executes tools locally in `NewAgent()` and `RunTurn()`.
- `RunTurn()` contains local sudo approval wiring for `shell`; this must move to `ConsoleAI`.
- `internal/desktopbackend/` has a useful protocol shape, but it is stdio RPC and still keeps runtime and tools in one process.
- `internal/config/modes.go` still falls back to a default mode in some cases, which violates the no-fallback rule.
- `internal/tools/read_file.go` and `internal/tools/write_file.go` exist in code, so specs and tool publication need to reflect them.

## Architecture Target

### Server responsibilities

- load config and modes
- build prompt payloads
- stream provider responses
- persist session messages and summaries
- expose only tools supported by the connected client
- wait for remote tool results and continue the LLM loop

### ConsoleAI responsibilities

- open terminal REPL
- send manifest and user messages to the server
- render assistant deltas, reasoning, tool activity, and usage
- execute local tool calls
- handle Ctrl-C cancellation
- request sudo approval and collect hidden password locally

### Hard boundary

The server must treat tool execution as a remote capability. It can describe tools and request them, but it must not execute shell, file read, file write, or file edit code locally for user tasks.

## Protocol Plan

Use a persistent WebSocket connection with typed JSON messages.

Required message types:

- `hello`
- `session_start`
- `user_message`
- `assistant_delta`
- `reasoning_delta`
- `tool_call`
- `tool_result`
- `tool_error`
- `usage`
- `turn_done`
- `cancel`
- `heartbeat`
- `error`

Protocol rules:

- include `protocol_version`; mismatch is fatal
- one connected client per active session in MVP
- tool results must preserve exact call ordering
- client disconnect during a turn must stop the turn with an explicit persisted error

## Capability Manifest Plan

`ConsoleAI` should send a manifest at connect time with:

- `client_type=console`
- OS info
- cwd
- available tool names
- tool-specific constraints if needed later

Server behavior:

- validate manifest against known remote tool definitions
- publish only client-supported tools to the LLM
- return an explicit error if the LLM requests a tool not available on the connected client
- never fall back to a server-local tool implementation

## Runtime Refactor Plan

### 1. Split tool definition from execution

Keep schema, description, and argument formatting available on the server, but move execution to the client for user-facing tools.

Recommended naming for MVP:

- `console_shell`
- `console_read_file`
- `console_write_file`
- `console_replace_block`

`ConsoleAI` can map these names to the existing local implementations now used by:

- `internal/tools/shell.go`
- `internal/tools/read_file.go`
- `internal/tools/write_file.go`
- `internal/tools/replace_block.go`

### 2. Add a remote execution boundary

Refactor the runtime so the LLM loop depends on a tool executor boundary instead of calling local `tool.Execute()` directly for the distributed mode.

The server-side flow should become:

1. stream provider response
2. persist assistant tool call message
3. emit remote `tool_call`
4. wait for `tool_result` or `tool_error`
5. persist tool result
6. continue the loop

### 3. Remove server-local sudo handling for remote shell

The current `RunTurn()` branch that detects `shell` + `sudo` must not remain server-side for remote tools. Sudo approval and password entry must happen entirely in `ConsoleAI`.

## Implementation Steps

### Phase 1. Protocol and boundaries

- add `internal/protocol/` with typed request and event structs
- choose the WebSocket package and pin it explicitly
- define protocol versioning and error model
- define capability manifest schema

### Phase 2. Server runtime path

- add `cmd/blazeai-server/main.go`
- add `internal/server/` connection and session orchestration
- adapt runtime to use a remote tool executor path
- expose remote-tool definitions based on the active client manifest

### Phase 3. ConsoleAI client

- add `cmd/consoleai/main.go`
- reuse current console rendering behavior where possible
- connect to the server over WebSocket
- send user messages and cancellation events
- execute remote tools locally and return results

### Phase 4. No-fallback config cleanup

- remove `modes.json` fallback behavior from `internal/config/modes.go`
- make missing or invalid modes an explicit startup error outside first-run setup
- update tests that currently expect auto-default behavior

### Phase 5. Prompt and spec updates

- update prompt text to state that tools run on the connected client, not on the server
- update tool docs and architecture specs to reflect the remote-tool model
- document the MVP limits: one client, console only, no server-side user tool execution

## Likely Changed Files

New files and packages:

- `cmd/blazeai-server/main.go`
- `cmd/consoleai/main.go`
- `internal/server/`
- `internal/client/`
- `internal/protocol/`
- `internal/remotetools/`

Existing code likely to change:

- `internal/runtime/runtime.go`
- `internal/config/modes.go`
- `internal/tools/`
- `internal/console/`
- `prompts/`
- `specs/`
- `ideas/server-centric-clients.md`

## Validation Plan

- unit tests for protocol encode/decode
- unit tests for manifest validation
- runtime tests for remote tool success, explicit remote tool error, unavailable tool, disconnect, and cancellation
- config tests for no-fallback modes behavior
- integration smoke test: start server, connect `ConsoleAI`, trigger `console_shell`
- `go test ./...`
- `go build ./...`

## Risks And Open Questions

- skill tools do not fit the MVP cleanly because skill discovery and execution are currently local-process concepts
- project-scoped skills depend on local filesystem context that the server does not own yet
- remote cancellation must kill client-side shell processes reliably, not just stop server streaming
- Telegram in the new model should stay text-only unless another execution client is connected
- Docker should wait until the server + ConsoleAI flow is stable
