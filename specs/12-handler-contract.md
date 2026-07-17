# Handler Contract

## Source Files

| File | Role |
|------|------|
| `internal/runtime/runtime.go:37-63` | Handler interface definition, AgentActivity, AgentActivityHandler, StreamPhaseHandler |
| `internal/runtime/runtime.go` | Agent struct — calls Handler methods during RunTurn |
| `internal/console/console.go` | Console — primary Handler implementation |
| `internal/telegram/handler.go` | Telegram bridge — secondary Handler implementation |

## Definition

The Handler interface is the only boundary between the agent core (runtime) and
user-facing transports. Console and Telegram implement this interface over the
same core.

```go
type Handler interface {
    OnContent(delta string)
    OnToolCall(name string, args string)
    OnToolResult(name string, result string)
    OnUsage(promptTokens, cachedTokens, uncachedTokens int)
    OnSystem(message string)
    OnMaintenanceCall(name string, args string)
    OnMaintenanceResult(name string, result string)
    RequestSudoApproval(command string) (approved bool, password string)
}
```

### Optional Extensions

```go
// StreamPhaseHandler is an optional transport extension for richer provider wait-state updates.
type StreamPhaseHandler interface {
    OnStreamPhase(phase provider.StreamPhase)
}

// AgentActivity describes one ephemeral child-agent lifecycle event.
type AgentActivity struct {
    Agent            string
    Kind             string   // "started", "completed", "failed", "timed out", "cancelled", "tool_call", "tool_result", "system"
    Tool             string
    Status           string   // "running", "ok", "error", "info"
    Text             string
    LastPromptTokens int
}

// AgentActivityHandler is an optional transport extension for child activity.
type AgentActivityHandler interface {
    OnAgentActivity(activity AgentActivity)
}
```

## Method Call Sequence (One Turn)

```
Agent.RunTurn()
  ├─ Append user message to session
  ├─ Build prompt
  ├─ Provider.Stream(...)
  │    ├─ OnSystem (streaming phase updates) — optional via StreamPhaseHandler
  │    └─ OnContent (0+ streaming chunks) — final text content
  ├─ OnUsage(promptTokens, cachedTokens, uncachedTokens) — token usage from response
  │
  ├─ For each tool call in response:
  │    ├─ OnToolCall(name, formattedArgs) — notify about to execute
  │    ├─ OnToolResult(name, result) — result returned
  │    └─ (loop back to LLM with tool results)
  │
  └─ (continue until no tool calls → end of turn)
```

### Content Streaming

`OnContent(delta)` is called for each chunk of visible text content during
`Provider.Stream()`. The transport is responsible for assembling streaming
chunks (console streams directly to terminal).

### Tool Lifecycle

Each tool call in the LLM response follows this sequence:

1. **OnToolCall(name, formattedArgs)** — called before execution with the tool
   name and a display-purpose string from FormatArgs. The transport should show
   this as "tool is running" (console: emoji + purpose). The runtime exits the
   streaming phase for this — tool calls are discrete, not streamed.

2. **Tool execution** — `registry.Get(name).Execute(ctx, args)`. May involve
   `RequestSudoApproval` for sudo commands.

3. **OnToolResult(name, result)** — called after execution with the full result
   string. The transport shows completion status. Console appends `✔️/✖️/⏱`
   badge and CTX tokens.

After ALL tool results are collected, they are appended to the session and fed
back to the LLM for the next turn.

### Usage Reporting

`OnUsage(promptTokens, cachedTokens, uncachedTokens)` is called once after each
LLM response with the provider-reported token counts. Three parameters:
- `promptTokens` — total input tokens
- `cachedTokens` — tokens served from prompt cache
- `uncachedTokens` — tokens not cached (promptTokens - cachedTokens)

The transport uses this for:
- Console: displays CTX in response separator and after tool results
- Telegram: currently unused (passed to transport for future display)

### System Notifications

`OnSystem(message)` is called when the runtime needs to display a system-level
notification. Used for streaming phase transitions and status updates that are
not part of the LLM content stream.

### Maintenance Operations

`OnMaintenanceCall(name, args)` and `OnMaintenanceResult(name, result)` are
user-visible internal operations rendered like tool calls. They represent
background work (e.g., OAuth token refresh) that the user should see as activity
but that are not LLM-initiated tool calls.

### Sudo Approval

`RequestSudoApproval(command)` is called before executing a shell command
containing `sudo`. The runtime parses the command text to detect `sudo` presence
before calling this method.

Return values:
- `approved=true, password=<string>` — user approved, password to pipe to sudo
- `approved=false, password=""` — user declined, tool call skipped with
  `"aborted: user declined sudo approval"`

The password is passed via `BLAZE_SUDO_PASSWORD` env var to the shell tool and
never stored in session JSON or prompt text.

## Transport Implementations

### Console

File: `internal/console/console.go`

Console is the primary transport. It renders:
- `OnContent` → `[BLAZE]` label, streaming text
- `OnToolCall` → emoji + purpose on its own line (e.g., `💻 Search files...`)
- `OnToolResult` → status badge + CTX on same line (` ✔️ CTX: 45K`)
- `OnUsage` → stored for CTX display
- `OnSystem` → phase indicator during provider connect/wait
- `OnMaintenanceCall` → displayed like a tool call with wrench emoji
- `OnMaintenanceResult` → completion badge
- `RequestSudoApproval` → prompt user in terminal with hidden input
- `OnAgentActivity` → child agent activity display (implements AgentActivityHandler)

See 13-console-ui.md for full rendering details.

### Telegram

File: `internal/telegram/handler.go`

Telegram bridge is a secondary transport. It renders:
- `OnContent` → streams as Telegram message edits (single message per turn)
- `OnToolCall` → appends inline to the message
- `OnToolResult` → status badge inline
- `OnUsage` → currently no-op
- `OnSystem` → currently no-op
- `OnMaintenanceCall`/`OnMaintenanceResult` → currently no-op
- `RequestSudoApproval` → always returns false (no sudo over chat)

## Design Rationale

- **Single interface** — both transports implement the same Handler, no transport
  awareness in the runtime
- **Streaming-first** — OnContent is streaming for immediate user feedback;
  transports can still buffer if needed
- **Sequential tool calls** — not streamed, each tool is a discrete event with
  a clear start (OnToolCall) and end (OnToolResult)
- **Sudo in handler** — password input is inherently transport-specific (terminal
  vs Telegram button); runtime just delegates
- **No transport branching in runtime** — Handler abstracts all UI concerns
- **Optional interfaces** — StreamPhaseHandler and AgentActivityHandler are
  checked via type assertion, keeping the core Handler minimal
