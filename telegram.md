# Telegram Bridge Plan

## Goal

Implement a Telegram bridge for BlazeAI with the smallest architecture that fits the approved use case:

- one Telegram bot per running process
- one allowed Telegram chat per bot instance
- one BlazeAI `runtime.Agent` per bot instance
- one working folder per bot instance
- one persistent selected model per bot instance
- no Telegram mode switching in v1
- bridge and instance configuration managed by a skill, not by an interactive runtime wizard

This plan intentionally avoids multi-chat and multi-user routing. If another bot is needed later, it runs as another process with its own instance folder and its own working folder.

## Approved Architecture

### Core Model

The Telegram bridge is a new transport over the existing `runtime.Handler` contract.

The runtime remains unchanged in its main shape:

- one `runtime.Agent`
- one `session.Session`
- one `Handler`
- one `workDir`

For Telegram, that maps to:

- one process runs one bot instance
- one bot instance accepts messages only from one configured `allowed_chat_id`
- one bot instance owns one agent and one active session context
- one bot instance persists its own selected model outside the global config files

### Why This Is The Simplest Correct Model

- It matches the current BlazeAI runtime shape, which is already 1:1.
- It avoids a chat-to-agent map, per-chat locking, and session multiplexing.
- It keeps project session storage and project skills tied to the configured `workDir`.
- It allows multiple bots later by running multiple bridge processes, each with a different instance name and `workDir`.

## Storage Layout

### Global App Home Layout

```text
app_home/
  config/
    config.json
    modes.json
  telegram/
    <instance>/
      bridge.json
      state.json
  projects/
    <sanitized_workdir>/
      sessions/
      skills/
```

### Responsibility Of Each Location

`app_home/config/config.json`
- global provider definitions
- API keys
- favorite models
- compaction configuration
- other global runtime settings

`app_home/config/modes.json`
- console and other global mode configuration
- not used by Telegram for active model selection

`app_home/telegram/<instance>/bridge.json`
- static Telegram instance configuration
- bot token
- allowed chat id
- working folder

`app_home/telegram/<instance>/state.json`
- mutable Telegram instance state
- selected model for that instance

`app_home/projects/<sanitized_workdir>/sessions/`
- normal BlazeAI conversation sessions
- still managed by the existing session system

`app_home/projects/<sanitized_workdir>/skills/`
- project-scoped skills for that bot's working folder

## Telegram Instance Files

### `bridge.json`

Purpose: static configuration for one Telegram bot instance.

Example:

```json
{
  "bot_token": "<telegram-bot-token>",
  "allowed_chat_id": 123456789,
  "workdir": "/srv/hermes-agent"
}
```

Rules:

- `bot_token` is required
- `allowed_chat_id` is required
- `workdir` is required
- missing or invalid values must fail startup with a clear error
- no fallback to another chat, another folder, or the global current directory

### `state.json`

Purpose: mutable persistent state for one Telegram bot instance.

Example:

```json
{
  "selected_model": "openai/gpt-5.5"
}
```

Rules:

- `selected_model` is required
- it must be in `provider/model_name` format
- its provider must exist in the global `config.json`
- Telegram model changes update only this file
- Telegram model changes must not write to global `config.json`
- Telegram model changes must not write to global `modes.json`

## Transport Behavior

### Startup Flow

For `blazeai --telegram <instance>`:

1. Bootstrap app home.
2. Load global `config.json`.
3. Resolve `app_home/telegram/<instance>/`.
4. Load `bridge.json`.
5. Load `state.json`.
6. Validate `bot_token`, `allowed_chat_id`, `workdir`, and `selected_model`.
7. Resolve builtin prompts and builtin skills as usual.
8. Create or resume the BlazeAI session based on the configured `workdir`.
9. Create one `runtime.Agent` for this instance.
10. Attach a Telegram handler implementing `runtime.Handler`.
11. Start Telegram long polling.
12. Retry transient `getUpdates` transport errors with a short backoff instead of stopping the bridge.
13. Ignore every update whose `chat_id` does not match `allowed_chat_id`.

### Runtime Message Flow

For a normal Telegram message:

1. Receive update from Telegram.
2. Verify the update belongs to `allowed_chat_id`.
3. Reject unsupported update types.
4. Parse Telegram command if the message starts with `/`.
5. If it is a supported local command, handle it in the bridge.
6. Otherwise, forward the text to `agent.RunTurn(ctx, text)`.
7. The Telegram handler receives `OnContent`, `OnToolCall`, `OnToolResult`, and `OnUsage` callbacks.
8. The handler sends or edits Telegram messages to reflect the response.

### Telegram Output Strategy

Telegram is not a terminal and has message/edit limits. The bridge should not stream token-by-token.

The polling loop must also tolerate transient network failures such as `EOF`, timeouts, and `connection reset by peer`. Those failures should retry after a short delay instead of terminating the process.

Recommended v1 strategy:

- buffer assistant output in memory
- flush updates at a fixed interval such as 300-700 ms
- edit the active Telegram message while it remains within Telegram limits
- split into a new message when the current message would exceed Telegram size limits
- send tool call and tool result notices as short separate messages

This keeps the runtime streaming behavior while adapting it to Telegram safely.

## Runtime Integration

### Existing Runtime Constraints

The current runtime persists model changes globally:

- `SetModel()` writes to `modes.json` when `CurrentMode` is present
- otherwise it writes to `config.json`

That behavior is correct for console mode but wrong for Telegram instance-local model persistence.

### Required Runtime Rule For Telegram

Telegram must use a model-switch path that:

- validates the requested model against the global provider config
- recreates the provider client
- updates the agent's in-memory `ModelID`
- does not write to `config.json`
- does not write to `modes.json`

### Recommended Runtime Change

Add a runtime method dedicated to local model switching, for example:

- `SetModelLocal(modelID string)`

Behavior:

1. validate format
2. validate provider existence in global config
3. create a new provider client
4. assign `a.Provider`
5. assign `a.ModelID`
6. do not touch `a.Config.Save()`
7. do not touch `a.Modes.Save()`

The Telegram bridge then persists the chosen model in `state.json` after `SetModelLocal()` succeeds.

### Telegram And Modes

Telegram does not expose mode switching in v1.

Rules:

- Telegram does not cycle modes
- Telegram does not persist `LastMode`
- Telegram does not use mode directives as an exposed user feature
- the active model comes from `state.json`, not from `modes.json`

Implementation note:

- the runtime may still internally load modes today
- but Telegram startup must override the active model with `state.json.selected_model`
- later cleanup can make Telegram bypass modes more explicitly if needed

## Commands Exposed In Telegram

### Allowed In v1

- `/model`
- `/clear`
- `/new`
- `/exit`
- `/help`

### Not Exposed In v1

- mode switching
- workdir switching
- provider switching

### Command Behavior

#### `/model`

Without argument:

- show the current selected model for the instance
- optionally list favorite models from global config

With argument:

- validate the requested model
- switch the runtime locally using the Telegram-safe model setter
- persist the model to `state.json`
- confirm the change in chat

#### `/clear`

- clear the current conversation history
- keep the same Telegram instance
- keep the same selected model
- clear compaction summaries if present

#### `/new`

For v1, this should behave the same as `/clear` unless a separate semantic need appears.

Rationale:

- the current runtime reset behavior already provides a clean prompt state
- the bridge is intentionally simple

#### `/exit`

- close the session cleanly
- keep the process behavior explicit

Open design choice for implementation:

- either stop only the current session and keep the process alive
- or stop the bridge process entirely after sending a final message

Recommended v1 behavior:

- mark the session cleanly closed
- send a confirmation message
- keep the process alive

This avoids accidental bot shutdown from chat.

#### `/help`

- show the supported Telegram bridge commands
- explain that only one chat is accepted by this instance

## Security And Isolation Rules

### Chat Isolation

- each bot instance accepts one configured `allowed_chat_id`
- all other chats are ignored or rejected with no bridge action
- no fallback chat selection is allowed

### Instance Isolation

- each bot instance has its own `app_home/telegram/<instance>/`
- each bot instance has its own `workdir`
- each bot instance therefore gets separate project sessions and project skills

### Config Isolation

- provider definitions remain global
- selected Telegram model is local per instance
- Telegram never persists active model into global config files

### Skill-Based Configuration

Bridge configuration is created and maintained by a skill.

The skill is responsible for:

- creating the instance folder
- writing `bridge.json`
- writing `state.json`
- validating model names against the global config
- listing or updating configured Telegram instances

The bridge runtime is not responsible for interactive setup.

## Minimal Package Layout

Recommended new files:

```text
internal/telegram/
  telegram.go
  handler.go
  commands.go
  config.go
  state.go
```

Possible responsibilities:

`internal/telegram/config.go`
- load and validate `bridge.json`

`internal/telegram/state.go`
- load and save `state.json`

`internal/telegram/telegram.go`
- bridge startup
- Telegram polling loop
- allowed chat enforcement
- agent wiring

`internal/telegram/handler.go`
- `runtime.Handler` implementation for Telegram output

`internal/telegram/commands.go`
- Telegram command parsing and local command handling

Possible entrypoint options:

- extend `main.go` with `--telegram <instance>`
- or add a dedicated `cmd/telegram/main.go`

Recommended v1 choice:

- extend `main.go` with a Telegram flag if the CLI remains simple

## Implementation Plan

### Phase 1 - Define Telegram Instance Storage

1. Add a Telegram instance directory resolver under app home.
2. Define the Go structs for `bridge.json`.
3. Define the Go structs for `state.json`.
4. Add strict load and validation functions for both files.
5. Add tests for missing files, malformed JSON, invalid chat id, invalid workdir, and invalid model.

Deliverable:

- deterministic instance config/state loading with clear startup errors

### Phase 2 - Add Telegram-Safe Model Persistence

1. Add a runtime method for local model switching without global persistence.
2. Validate it uses the same provider/model format checks as console switching.
3. Ensure it recreates the provider client correctly.
4. Add tests proving the local model setter does not change global `config.json` or `modes.json`.

Deliverable:

- runtime support for per-instance Telegram model changes

### Phase 3 - Bootstrap One Telegram Agent

1. Add startup path for `--telegram <instance>`.
2. Load global config.
3. Load Telegram instance config/state.
4. Create or resume the normal BlazeAI session using the configured `workdir`.
5. Create one `runtime.Agent`.
6. Apply the instance-local selected model.
7. Attach a Telegram handler.

Deliverable:

- one Telegram bot instance can start and own one BlazeAI runtime agent

### Phase 4 - Implement Telegram Polling Loop

1. Add long polling with the chosen Telegram library.
2. Accept only text messages in v1.
3. Reject or ignore updates from any chat other than `allowed_chat_id`.
4. Retry transient polling transport failures with a short backoff.
5. Serialize processing so only one turn runs at a time for the instance.
6. Surface clear runtime errors to the allowed chat when possible.

Deliverable:

- the bridge receives and forwards messages safely

### Phase 5 - Implement Telegram Output Handler

1. Implement `runtime.Handler` for Telegram.
2. Buffer content from `OnContent`.
3. Flush content on a safe interval rather than per token.
4. Split large output into multiple Telegram messages.
5. Send short tool call and tool result markers.
6. Handle message edit failures gracefully and explicitly.

Deliverable:

- readable assistant output in Telegram within platform limits

### Phase 6 - Implement Telegram Commands

1. Implement `/help`.
2. Implement `/model` read and write behavior.
3. Persist `/model` changes to `state.json` only.
4. Implement `/clear` using runtime reset behavior.
5. Implement `/new` as the same reset behavior in v1.
6. Implement `/exit` as clean session close without shutting down the process.

Deliverable:

- minimal but complete Telegram command surface for the approved scope

### Phase 7 - Add Skill-Based Configuration Support

1. Add or extend a skill that creates Telegram instance folders.
2. Make the skill write `bridge.json` and `state.json`.
3. Make the skill validate the configured model against global providers.
4. Make the skill support editing an existing instance cleanly.

Deliverable:

- Telegram bridge instances can be configured without manual JSON authoring

### Phase 8 - Validation

Validation should cover:

1. startup succeeds only with valid `bridge.json` and `state.json`
2. startup fails clearly when required files or fields are missing
3. only `allowed_chat_id` is accepted
4. message text reaches `agent.RunTurn()`
5. output is delivered through the Telegram handler
6. `/model` changes the model and persists only to `state.json`
7. `/clear` and `/new` reset the conversation without changing the model
8. restart resumes the session and preserves the selected model
9. two different bot instances with different `workdir` values do not affect each other's sessions

## Risks And Decisions To Keep Explicit

### Telegram Library Choice

Need one concrete Telegram client library. Keep it minimal and stable.

### Message Limits

Telegram message length and edit rate limits require buffered output. Do not map terminal-style token streaming directly.

### Shell Tool Risk

The bot still controls the host shell through BlazeAI tools. This is acceptable only because the bridge is restricted to one allowed chat and intended for trusted use.

### Global Config Still Shared

Providers and API keys stay global. This is acceptable. The selected model must remain per instance.

### No Fallback Rule

If any required Telegram instance file is missing or invalid, startup must fail. No auto-generated defaults at runtime.

## Final Summary

The approved Telegram bridge architecture is:

- one process per bot instance
- one allowed chat per bot instance
- one agent per bot instance
- one workdir per bot instance
- one persistent selected model per bot instance in `state.json`
- no mode switching in Telegram
- configuration managed by a skill

This is the smallest architecture that fits the real use case while leaving a clean path to multiple bots later through separate instances instead of transport multiplexing.
