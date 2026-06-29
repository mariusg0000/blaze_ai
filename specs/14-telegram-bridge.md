# Telegram Bridge

## Source Files

| File | Role |
|------|------|
| `internal/telegram/telegram.go` | Run(), polling loop, openTelegramSession, bot client, retry logic |
| `internal/telegram/handler.go` | Handler — runtime.Handler implementation with buffer/flush |
| `internal/telegram/commands.go` | Bridge-local slash command handling (/help, /model, /clear, /exit) |
| `internal/telegram/config.go` | BridgeConfig struct, LoadBridgeConfig, InstanceDir |
| `internal/telegram/state.go` | State struct (SelectedModel), LoadState, SaveTo |
| `internal/telegram/doc.go` | Package docs |

## Overview

Telegram is a secondary transport over the shared `runtime.Handler` contract.
Each bot instance is one process, one chat, one agent.

Started via `--telegram <instance>` CLI flag in `main.go`.

## Architecture

```
BlazeAI (one process)
  ├── main.go
  │    └── --telegram <instance>
  │         └── telegram.Run()
  │              ├── LoadBridgeConfig(instance)  → bridge.json
  │              ├── LoadState(instance, cfg)     → state.json + resolved model
  │              ├── openTelegramSession(sessDir) → fixed session/ folder
  │              ├── runtime.NewAgent(...)        → shared agent core
  │              ├── agent.Builder.TransportContext = "Telegram bridge..."
  │              ├── agent.Compactor.RebuildForResume(sess)
  │              ├── agent.SetModelLocal(state.SelectedModel)
  │              ├── NewBotClient(token)          → stdlib HTTP Bot API client
  │              ├── publishTelegramCommands(...)  → SetCommands API
  │              ├── NewHandler(client, chatID)    → output handler
  │              ├── agent.Handler = handler
  │              └── runPolling(ctx, ...)          → long poll loop
```

## Instance Storage

```
app_home/telegram/<instance>/
  bridge.json     — static config (bot token, allowed chat ID, workdir)
  state.json      — mutable state (selected_model, resolved)
  session/
    session.json  — single fixed session (not a collection of rotating folders)
```

### bridge.json

```json
{
  "bot_token": "123456:ABC-DEF...",
  "allowed_chat_id": -1001234567890,
  "workdir": "/home/user/projects"
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `bot_token` | Yes | Telegram Bot API token |
| `allowed_chat_id` | Yes | Single allowed chat (positive = user, negative = group) |
| `workdir` | No | Working directory for tool execution (defaults to instance dir) |

### state.json

```json
{
  "selected_model": "openai/gpt-4o"
}
```

Persisted only by `/model` command. Loaded at startup; if empty, resolved from
the global config's active model.

## Session

Unlike the console (rotating `sessions/` folder with random names), Telegram
uses a **single fixed session** at `app_home/telegram/<instance>/session/session.json`.

- On startup: `session.Load(sessDir)` — if `ErrSessionNotFound`, creates `sessDir`
  and initializes a new Session with that folder.
- On resume: calls `agent.Compactor.RebuildForResume(sess)` to rebuild synthetic
  summary messages from summary files.
- No clean-close filtering — `session.Last()` or `LastClean()` are not used;
  there is only one session folder.

## Polling

### Long Poll

`getUpdates` with `timeout=30s`. Standard Telegram Bot API long polling.

### Retry

Transient polling failures (EOF, timeout, connection reset) are retried
in-process with a 2-second fixed backoff. This prevents network glitches from
terminating the bridge.

Non-retryable errors (auth failure, invalid response) fail fast and terminate
the process.

### Start-up Drain

On startup, the polling loop drains any pending updates before processing.
This prevents stale commands from being replayed after a restart.

### Serialization

Only one message is processed at a time. The polling loop blocks on
`agent.RunTurn()` and resumes polling after the turn completes. Commands are
handled bridge-locally and skip the LLM.

## Handler (Output)

The Telegram Handler implements `runtime.Handler` with buffered output:

```
BeginTurn(ctx)
  ├─ Reset content buffer, sent messages, typing state
  ├─ h.sendTypingNow() — immediate typing indicator
  ├─ go h.flushLoop()   — 500ms periodic flush
  └─ go h.typingLoop()  — 4s periodic typing indicator
```

### Flush Loop

Every 500ms, the handler sends the current buffered content to Telegram:
- If no message exists yet → `SendMessage` (creates first message)
- If message exists → `EditMessage` (edits in place)
- Text split at 3500 chars → multiple messages if needed

### Message Splitting

Content beyond `maxTelegramTextSize` (3500) is split into a sequence of messages.
Each message's text and ID are tracked in `sentIDs[]` / `sentTexts[]` for
subsequent edits.

### Typing Indicator

A typing action (`SendChatAction "typing"`) is sent every 4 seconds while
content is being streamed. The initial onTurnStart typing is sent immediately.

### Handler Methods

| Handler Method | Telegram Behavior |
|----------------|-------------------|
| `OnContent(delta)` | Append to content buffer (flushed periodically) |
| `OnReasoning(delta)` | No-op (not shown in Telegram) |
| `OnToolCall(name, args)` | Append emoji + purpose line to buffer |
| `OnToolResult(name, result)` | Append status badge line to buffer |
| `OnUsage(promptTokens)` | Store for potential display |
| `RequestSudoApproval(command)` | Prompt user via reply (approved boolean) |

### Turn End

```
FinishTurn()
  ├─ Close flush ticker, wait for final flush
  ├─ h.flushNow() — send remaining buffered content
  └─ Return last error (if any)
```

## Commands

Telegram commands are handled bridge-locally and do not reach the LLM.

| Command | Behavior |
|---------|----------|
| `/start` | Show help text |
| `/help` | Show help text |
| `/model` | Without arg: show current model. With arg: `agent.SetModelLocal()` + persist to state.json |
| `/clear` or `/new` | `agent.ResetConversation()` |
| `/exit` | `agent.CloseSession()` — bridge stays online, only session is closed |

Bot commands are registered on startup via `SetCommands` so Telegram shows
them in the chat UI.

`@botname` suffix in commands is stripped (e.g. `/model@MyBot` → `/model`).

## Transport Context

A `TransportContext` string is injected into the prompt builder when the
Telegram bridge starts:

```
Telegram bridge transport is active.
Telegram instance: <name>
Replies are sent into a Telegram chat, not an interactive terminal.
Exactly one configured chat can reach this instance.
Do not start, restart, or duplicate BlazeAI or Telegram bridge processes
unless the user explicitly asks.
Do not treat generic greetings or /start as setup instructions.
Keep replies concise for chat and avoid unnecessary tool chatter.
```

Injected via `{TRANSPORT_CONTEXT}` placeholder in `sysprompt.md`.

## Model Management

Model switching in Telegram is **instance-local**:
- `agent.SetModelLocal(name)` — sets the model on the agent without touching
  global `config.json` or `modes.json`
- `/model` persists to `state.json` in the instance folder
- Startup reads `state.json` and calls `SetModelLocal()` to restore

This means each Telegram instance can have its own model independent of the
console session.

## Startup Sequence

```
Run(ctx, cfg, osType, promptsFS, instance)
  ├─ LoadBridgeConfig(instance)      → bridge.json
  ├─ LoadState(instance, cfg)         → state.json
  ├─ openTelegramSession(sessDir)    → fixed session/ folder
  ├─ runtime.NewAgent(...)           → agent with shared core
  ├─ agent.Builder.TransportContext   → Telegram context
  ├─ agent.Compactor.RebuildForResume → summaries
  ├─ agent.SetModelLocal(...)         → instance model
  ├─ NewBotClient(token)             → HTTP Bot API client
  ├─ publishTelegramCommands(...)     → SetCommands
  ├─ NewHandler(client, chatID)      → output handler
  ├─ agent.Handler = handler
  └─ runPolling(ctx, ...)            → long poll (blocks)
```

## Process Model

- One process per bot instance
- Started via `blazeai --telegram <instance>` (flag, not subcommand)
- Runs until context cancelled or fatal error
- Systemd: `Restart=always` for resilience
- No mode switching in v1

## Safety

- `RequestSudoApproval` prompts user via Telegram (no hidden input channel available).
- Bot token stored in `bridge.json` (0600 perms if created by skill).
- Secrets never in session JSON or prompt text.
