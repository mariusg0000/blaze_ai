# 06 — Telegram Bridge Transport

## Purpose

The Telegram bridge is a transport implementation for BlazeAI that allows the same runtime agent to operate through Telegram chat instead of the console terminal. It is not a separate product — it is a transport layer over the existing `runtime.Handler` contract, controlled by a `--telegram <instance>` flag.

## Architecture Overview

### Core Model

```
┌─────────────────────────────────────────┐
│              blazeai process            │
│                                         │
│  ┌───────────────────────────────────┐  │
│  │         runtime.Agent             │  │
│  │  ┌─────────┐  ┌────────────────┐  │  │
│  │  │ Builder  │  │  Compactor     │  │  │
│  │  │ (prompt) │  │ (context mgmt) │  │  │
│  │  └─────────┘  └────────────────┘  │  │
│  │  ┌─────────┐  ┌────────────────┐  │  │
│  │  │Provider │  │  Session       │  │  │
│  │  │ (LLM)   │  │  (persistence) │  │  │
│  │  └─────────┘  └────────────────┘  │  │
│  │  ┌──────────────────────────────┐ │  │
│  │  │      Handler (Telegram)      │ │  │
│  │  │  OnContent / OnToolCall /    │ │  │
│  │  │  OnToolResult / OnUsage      │ │  │
│  │  └──────────────────────────────┘ │  │
│  └───────────────────────────────────┘  │
│                                         │
│  ┌───────────────────────────────────┐  │
│  │      Telegram BotClient          │  │
│  │  getUpdates / sendMessage /      │  │
│  │  editMessage / sendChatAction    │  │
│  └───────────────────────────────────┘  │
└─────────────────────────────────────────┘
```

One process runs one bot instance. One bot instance accepts messages only from one configured `allowed_chat_id`. One bot instance owns one agent, one session, one workdir, one selected model.

### Key Design Decisions

- **No third-party library**: Standard library HTTP + `net/url` for Telegram Bot API calls. The bridge uses only a tiny subset of the API (getUpdates, sendMessage, editMessage, sendChatAction, setMyCommands). Adding a Go module dependency for ~5 REST endpoints is not justified.

- **One fixed session per instance**: Telegram uses `{APP_HOME}/telegram/<instance>/session/session.json` — a single persistent file, not rotating `sessions/<random>/` subfolders. Each Telegram instance has exactly one ongoing conversation.

- **SetModelLocal()**: Decouples model switching from global config persistence. The Telegram bridge calls `SetModelLocal(modelID)` which validates the model, creates a new provider client, and updates the agent in memory — without writing to `config.json` or `modes.json`. Persistence to `state.json` is the bridge's responsibility.

- **No routing/multiplexing**: No chat-to-agent map, no per-chat locking, no session multiplexing. Multiple bots require multiple processes.

- **Config via skill, not wizard**: Instance setup is driven by a builtin skill (`telegram_bridge`), not by interactive runtime wizards. The skill handles BotFather bot creation, chat ID discovery via `getUpdates`, file authoring, and startup validation.

## Storage Layout

### Directory Structure

```
{APP_HOME}/
  config/
    config.json          — global providers, API keys, compaction, favorites
  telegram/
    <instance>/
      bridge.json        — static config: bot_token, allowed_chat_id, workdir
      state.json         — mutable state: selected_model
      session/
        session.json     — single persistent conversation session (not sessions/<random>/)
  skills/
    <skill-name>/
      skill.md
  projects/
    <sanitized_workdir>/
      sessions/          — console sessions (isolated from Telegram)
      skills/
  scripts/
  backups/
```

### bridge.json

```json
{
  "bot_token": "<telegram-bot-token>",
  "allowed_chat_id": 123456789,
  "workdir": "/srv/project"
}
```

Rules:
- All three fields required.
- `workdir` must be an absolute path to an existing directory.
- `allowed_chat_id` is an int64; 0 is invalid (valid Telegram chat IDs are positive or negative integers).
- No defaults, no fallbacks.

### state.json

```json
{
  "selected_model": "openai/gpt-5.5"
}
```

Rules:
- `selected_model` is required, must be `provider/model_name` format.
- Provider must exist in global `config.json`.
- Model changes persist only to this file. No writes to `config.json` or `modes.json`.
- Atomic write: writes to `.tmp` then `os.Rename` for crash safety.

### Instance Name

- Validated by `InstanceDir()`: non-empty, not `.` or `..`, no path separators.
- Resolved to `{APP_HOME}/telegram/<instance>/`.
- Used for all three instance files: `bridge.json`, `state.json`, `session/session.json`.

## Startup Flow

```
blazeai --telegram <instance>

1. Bootstrap app home (creates telegram/ subfolder if missing).
2. Load global config.json.
3. Resolve {APP_HOME}/telegram/<instance>/ via InstanceDir().
4. Load and validate bridge.json (bot_token, allowed_chat_id, workdir).
5. Load and validate state.json (selected_model against global providers).
6. Resolve {APP_HOME}/telegram/<instance>/session/.
7. Load existing session.json or create empty one (single fixed path).
8. Create runtime.Agent with the session, workdir, prompts, OS.
9. Inject TransportContext into Agent.Builder (LLM knows it's on Telegram).
10. Apply instance model via SetModelLocal().
11. Publish bot commands via setMyCommands.
12. Drain pending updates (fetch with offset=0, compute start offset).
13. Start long polling (timeout=30s).
14. Only process updates where chat.id == allowed_chat_id.
```

### Pending Update Drain

On first boot (or after restart), Telegram may have queued old updates. The bridge calls `getUpdates(offset=0, timeout=0)` once at startup, computes `max(update_id)+1` as the starting offset, and discards those updates. This prevents stale messages ("/start", "Salut", etc. sent during chat ID discovery) from being fed to the LLM.

### Transport Context

After startup, `agent.Builder.TransportContext` is set to a string like:

```
Telegram bridge transport is active.
Telegram instance: home
Replies are sent into a Telegram chat, not an interactive terminal.
Exactly one configured chat can reach this instance.
Do not start, restart, or duplicate BlazeAI or Telegram bridge processes unless the user explicitly asks.
Do not treat generic greetings or /start as setup instructions.
Keep replies concise for chat and avoid unnecessary tool chatter.
```

This is rendered as `{TRANSPORT_CONTEXT}` in `sysprompt.md`. It prevents the LLM from:
- Treating `/start` or greetings as setup instructions
- Attempting to duplicate or restart the bridge
- Using verbose terminal-style output in chat

## Message Flow

### Inbound

1. Receive `Update` from Telegram long poll.
2. Verify `update.Message.Chat.ID == allowed_chat_id`. If not, skip.
3. Skip empty messages.
4. If message starts with `/`, parse as command via `HandleCommand()`.
5. If command is recognized (help, model, clear, new, exit), handle locally.
6. Otherwise, call `agent.RunTurn(ctx, text)`.

### Outbound (Handler)

The `Handler` struct implements `runtime.Handler` with buffered output:

```
BeginTurn(ctx)
  ├── reset content buffer, sent message tracking
  ├── sendChatAction("typing") immediately
  ├── start flushLoop goroutine (500ms ticker)
  └── start typingLoop goroutine (4s ticker)

OnContent(delta)
  └── append delta to content buffer

OnToolCall(name, args)
  └── send "[tool] <name> <args>" as separate message

OnToolResult(name, result)
  └── send "[tool result] <name> <truncated_result>" as separate message

OnUsage(promptTokens)
  └── record token count for context display

FinishTurn()
  ├── close stopFlush channel (both goroutines exit)
  ├── flush remaining content
  └── return last error

flushLoop (goroutine)
  every 500ms:
    flushNow():
      split content at 3500 chars (maxTelegramTextSize)
      for each chunk:
        - if already sent: editMessage (only if changed)
        - if new: sendMessage
        - on edit failure: fallback to sendMessage
      track sent message IDs and texts

typingLoop (goroutine)
  every 4s:
    sendChatAction("typing")
  (shared stopFlush channel with flushLoop)
```

### Message Splitting

`splitTelegramText(text, limit)` splits on newlines near the limit boundary. If no newline found in the window, splits at exact `limit`. Ensures Telegram's 4096-byte message limit is respected (safe margin at 3500 chars).

## Commands

All commands are processed locally in the bridge. Unknown commands pass through to the LLM as normal text.

| Command  | Behavior                                                    |
|----------|-------------------------------------------------------------|
| `/help`  | Show supported command list and chat restriction note.      |
| `/start` | Same as `/help`. Prevents BotFather default handling.       |
| `/model` | No arg: show current + favorites. Arg: switch model locally.|
| `/clear` | Reset conversation via `agent.ResetConversation()`.         |
| `/new`   | Same as `/clear` in v1.                                     |
| `/exit`  | Close session cleanly. Process stays alive.                 |

Bot commands are published to Telegram at startup via `setMyCommands` with:

- `/help` — show supported commands
- `/model` — show or change the instance model
- `/clear` — clear the current conversation
- `/new` — clear the current conversation
- `/exit` — close the current session cleanly

Telegram clients display these in the command menu automatically.

## UX Features

### Bot Typing Indicator

During an LLM turn, `sendChatAction("typing")` fires:
- Immediately in `BeginTurn()` after unlocking the mutex
- Every 4 seconds while the turn is active via `typingLoop` goroutine
- Stops when `FinishTurn()` closes the shared `stopFlush` channel

### Command Menu

`setMyCommands` is called once at bridge startup (not per-chat) because the bridge has exactly one allowed chat. The Telegram API expects the `commands` form parameter to be a JSON array string: `[{"command":"help","description":"..."},...]`. Serializing correctly requires marshalling `[]botCommand` directly, not wrapped in an object.

## Package Layout (internal/telegram/)

```
internal/telegram/
  telegram.go       — bridge startup, polling loop, BotClient, Update/Message types
  handler.go        — runtime.Handler implementation with buffered output
  commands.go       — local command parsing and handling
  config.go         — bridge.json loading and validation
  state.go          — state.json loading, validation, atomic saving
  handler_test.go   — handler buffering, message split, typing tests
  commands_test.go  — command dispatch and menu structure tests
  config_test.go    — config validation tests
  telegram_test.go  — drain, offset, session, command publishing tests
```

## Security Model

### Chat Isolation

- One bot instance accepts exactly one `allowed_chat_id`.
- All other chats are silently ignored at the polling level.
- No fallback chat selection.

### Instance Isolation

- Each bot has its own `{APP_HOME}/telegram/<instance>/` with `bridge.json`, `state.json`, `session/`.
- Each bot has its own `workdir`, and therefore its own project sessions and project skills.
- Two bot instances with different `workdir` values share no state.

### Config Isolation

- Providers and API keys remain in global `config.json` (shared).
- Active model is per-instance, persisted in `state.json` only.
- `SetModelLocal()` never writes to `config.json` or `modes.json`.

### Shell Risk

The bot still controls the host shell through BlazeAI tools. This is acceptable only because the bridge is restricted to one allowed chat and intended for trusted use. The LLM is explicitly instructed via `TransportContext` not to self-duplicate or launch processes without user request.

## Skills

### telegram_bridge (builtin)

The `telegram_bridge` skill is a builtin Markdown skill (not generated from Go code) that covers the full operator workflow:

1. **Bot Creation**: Guide user through BotFather to get a bot token.
2. **Chat ID Discovery**: Guide user to send a message then call `getUpdates` to find `chat.id`.
3. **Instance Folder**: Create `{APP_HOME}/telegram/<instance>/` if needed.
4. **bridge.json Authoring**: Write bot_token, allowed_chat_id, workdir.
5. **Model Selection**: Pick from global favorites, write to state.json.
6. **Startup**: `blazeai --telegram <instance>`.
7. **Verification**: Send a message, verify LLM response.
8. **Maintenance**: Token rotation, workdir change, model change, instance repair.

Constraints:
- `customize_me` defers Telegram instance work to `telegram_bridge`.
- `skill-manager` includes `telegram_bridge` in its builtin list.
- Collision rules apply: a user-created `telegram_bridge` in `app_home/skills/` overrides the builtin.

## Evolution

### Phase History (Completed)

| Phase | Description | Commit |
|-------|-------------|--------|
| Plan | Architecture document and 8-phase plan | `7265669` |
| 1-2 | Instance config/state loader, SetModelLocal() | `b921ff1`, `dd3e499` |
| 3-6 | Bridge startup, polling, handler, commands | `dd3e499` |
| 7 | Builtin telegram_bridge skill | `b940e3d` |
| Fix | Pending update drain, /start handling | `5ee54a0` |
| Fix | Session isolation from console | `d8ae970` |
| Fix | Single fixed session per instance | `249c48d` |
| UX | Bot command menu and typing indicator | `86e61ae` |
| Fix | setMyCommands JSON serialization | `351cae1` |

### Not In v1

- Multi-chat or multi-user routing.
- Mode switching.
- Workdir switching from chat.
- Provider switching from chat.
- Interactive setup wizards.
- Message media handling (photos, files, voice).
- Inline keyboards or buttons.
- Webhook mode (long polling is sufficient for v1).
- Multiple sessions per instance.

## Caveats

### 1. Mutex Ordering in Handler

`BeginTurn()` previously called `sendTypingNow()` while holding the handler mutex, causing a deadlock because `sendTypingNow()` tries to acquire the same mutex. The fix: unlock the mutex before calling `sendTypingNow()` (and before spawning the two goroutines). The struct fields needed by the goroutines (ctx, client, chatID) are read under the lock, so they are safe.

### 2. TypingLoop Termination

`typingLoop` shares the `stopFlush` channel with `flushLoop`. When `FinishTurn()` closes the channel, both goroutines must exit. They are not tracked separately — if one goroutine hangs, `FinishTurn()` waits only on `flushDone` (which `flushLoop` closes on exit). `typingLoop` has no independent done signal. This is acceptable because `sendChatAction` failures are non-fatal (the return value is discarded).

### 3. setMyCommands Payload Format

The Telegram Bot API expects the `commands` form parameter to be a **JSON array string**: `[{"command":"help","description":"..."}]`. It does **not** expect a JSON object with a `commands` key (`{"commands":[...]}`). Marshalling `[]botCommand` directly (not wrapped in a struct) is required.

### 4. Fixed Session Path

Session storage uses `session/` (singular), not `sessions/` (plural). This is a deliberate break from the console convention: Telegram has exactly one ongoing conversation per instance, so rotating subfolders are semantically wrong. The `session.Load()` function is called on the exact folder path, not `LastInDir()`.

### 5. Session Resume Behavior

Unlike console's `-c` flag (which filters for cleanly closed sessions), the bridge always resumes the existing session at the fixed path — even if it was not cleanly closed. An interrupted Telegram session (process killed, network lost) should continue from where it left off, not start fresh.

### 6. No Fallback Rule

If any required file or field is missing or invalid, startup fails with a clear error. No default values, no auto-generation, no fallback to another file or directory. This applies to `bridge.json`, `state.json`, `session/session.json`, and `workdir` existence.

### 7. Pending Update Safety

The drain call uses a short poll (`timeout=0`) to minimize startup delay. If Telegram returns no updates, the offset starts at 1. If updates exist, the offset starts at `max(update_id)+1`. This is safe because the bridge processes only one allowed chat — old updates for other chats are also drained but would be ignored anyway by the `allowed_chat_id` filter.

### 8. Token Security

The bot token is stored in `bridge.json` in plaintext under `app_home/telegram/<instance>/`. It is never logged, never exposed in session history, and never sent to the LLM. The bridge skill guides users to keep `app_home` properly permissioned.

### 9. Message Edit Rate Limits

The flush loop edits messages every 500ms. Telegram rate-limits message edits (roughly 1 edit/second per message/chat). The bridge handles edit failures by falling back to `sendMessage` (which creates a new message). In practice, the edit-during-streaming window is short enough that rate limits are rarely hit.
