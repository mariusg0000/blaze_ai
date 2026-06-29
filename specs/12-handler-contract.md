# Handler Contract

## Source Files

| File | Role |
|------|------|
| `internal/runtime/runtime.go:40-57` | Handler interface definition |
| `internal/runtime/runtime.go` | Agent struct — calls Handler methods during RunTurn |
| `internal/console/console.go` | Console — primary Handler implementation |
| `internal/telegram/handler.go` | Telegram bridge — secondary Handler implementation |

## Definition

The Handler interface is the only boundary between the agent core (runtime) and
user-facing transports. Both console and Telegram implement this interface over
the same core.

```go
type Handler interface {
    OnContent(delta string)
    OnToolCall(name string, args string)
    OnToolResult(name string, result string)
    OnUsage(promptTokens int)
    OnReasoning(delta string)
    RequestSudoApproval(command string) (approved bool, password string)
}
```

## Method Call Sequence (One Turn)

```
Agent.RunTurn()
  ├─ Build prompt
  ├─ Provider.Stream(...)
  │    ├─ OnReasoning (0+ streaming chunks) — reasoning/thinking tokens
  │    └─ OnContent (0+ streaming chunks) — final text content
  ├─ OnUsage(promptTokens) — token usage from response
  │
  ├─ For each tool call in response:
  │    ├─ OnToolCall(name, formattedArgs) — notify about to execute
  │    ├─ OnToolResult(name, result) — result returned
  │    └─ (loop back to LLM with tool results)
  │
  └─ (continue until no tool calls → end of turn)
```

### Reasoning and Content

These are streaming callbacks fired during `Provider.Stream()`:

- `OnReasoning(delta)` — called for each chunk of reasoning/thinking. May be
  empty if the model doesn't produce reasoning or if streaming hasn't started.
  Called BEFORE `OnContent` for the same response.

- `OnContent(delta)` — called for each chunk of visible text content. Called
  AFTER all reasoning chunks for the response.

Both are called with partial deltas. The transport is responsible for assembling
them if needed (console streams directly to terminal).

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

`OnUsage(promptTokens)` is called once after each LLM response with the
provider-reported prompt token count. The transport uses this for:
- Console: displays CTX in response separator and after tool results
- Telegram: currently unused (passed to transport for future display)

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
- `OnReasoning` → `🧠` prefix, streaming text (colored dim)
- `OnContent` → `[BLAZE]` label, streaming text
- `OnToolCall` → emoji + purpose on its own line (e.g., `💻 Search files...`)
- `OnToolResult` → status badge + CTX on same line (` ✔️ CTX: 45K`)
- `OnUsage` → stored for CTX display
- `RequestSudoApproval` → prompt user in terminal with hidden input

See 13-console-ui.md for full rendering details.

### Telegram

File: `internal/telegram/handler.go`

Telegram bridge is a secondary transport. It renders:
- `OnContent` → streams as Telegram message edits (single message per turn)
- `OnToolCall` → appends inline to the message
- `OnToolResult` → status badge inline
- `OnReasoning` → skipped (not shown in Telegram)
- `OnUsage` → currently no-op
- `RequestSudoApproval` → prompts user via Telegram button UI

## Design Rationale

- **Single interface** — both transports implement the same Handler, no transport
  awareness in the runtime
- **Streaming-first** — OnContent and OnReasoning are streaming for immediate
  user feedback; transports can still buffer if needed
- **Sequential tool calls** — not streamed, each tool is a discrete event with
  a clear start (OnToolCall) and end (OnToolResult)
- **Sudo in handler** — password input is inherently transport-specific (terminal
  vs Telegram button); runtime just delegates
- **No transport branching in runtime** — Handler abstracts all UI concerns
