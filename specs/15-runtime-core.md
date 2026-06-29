# Runtime Core

## Source Files

| File | Role |
|------|------|
| `internal/runtime/runtime.go` | Handler interface, Agent struct, NewAgent, RunTurn, SetModel, SetModelLocal, SetWorkDir, CloseSession, ResetConversation, SetMode, NextMode |

## Overview

The runtime is the agent core — it ties together all packages (config, session,
skills, prompt, tools, provider, compaction) into a single orchestration loop.
It defines the Handler interface as the only boundary between the agent core
and transports.

## Agent Struct

```go
type Agent struct {
    Config    *config.Config
    Modes     *config.ModesConfig
    Session   *session.Session
    Active    *skills.ActiveList
    Builder   *prompt.Builder
    Tools     *tools.Registry
    Provider  *provider.Client
    Handler   Handler
    Compactor *compaction.Manager

    ModelID     string
    CurrentMode *config.Mode
    WorkDir     string
    OS          platform.OS
}
```

All fields are public for direct access from callers. No getter/setter
indirection.

## RunTurn (The Main Loop)

```
RunTurn(ctx, userInput)
  ├─ Guard: nil Handler → error
  ├─ sanitizeSession() → drop invalid tool rounds
  ├─ Append user message to session
  │
  └─ for {  (tool call loop)
       │
       ├─ sanitizeSession()
       ├─ Builder.Build(session, active)    → full prompt from disk + history
       ├─ Compactor.StripReasoningFromPayload(messages)  → keep newest N reasoning
       ├─ Write prompt.json debug file
       ├─ injectDirective(messages, mode.Directive)  → volatile mode injection
       │
       ├─ Provider.Stream(ctx, messages, tools, onContent, onReasoning)
       │    └─ onReasoning called only when Config.ShowReasoning is true
       │
       ├─ OnUsage(promptTokens)         → report token usage to transport
       ├─ Build assistant message + persist
       │
       ├─ If aborted → appendAbortedToolResults + appendAbortMarker → return ErrTurnAborted
       │
       ├─ If no tool calls:
       │    ├─ Compactor.Compact(session, usage) → check + compact
       │    └─ return nil  (end of turn)
       │
       ├─ For each tool call:
       │    ├─ Check ctx cancellation
       │    ├─ Reset BLAZE_SUDO_PASSWORD
       │    ├─ If shell + sudo → RequestSudoApproval → password or skip
       │    ├─ OnToolCall(name, formattedArgs)
       │    ├─ tool.Execute(ctx, args)
       │    ├─ OnToolResult(name, result)
       │    ├─ Persist tool result message
       │    └─ Check ctx cancellation (mid-loop)
       │
       ├─ Compactor.Compact(session, usage)  → check after tool execution
       └─ }  (loop back — tool results are now in session history)
```

### Key Properties

- **Tool call loop**: inside `RunTurn`, not a separate function. Continues until
  the LLM returns no tool calls. Each iteration sends the full session history
  (including all prior tool results) back to the LLM.

- **Persist on every append**: session.Save() is called on every message append.
  No batching for persistence.

- **Strip reasoning every iteration**: `StripReasoningFromPayload` is called on
  the built prompt before every LLM call, not once at turn start. This means
  after each tool result comes back, reasoning from previous assistant messages
  is re-evaluated for stripping on the next iteration.

- **Compaction in two places**: after no-tool-calls (final turn) and after tool
  execution (before next LLM call). This ensures compaction fires even during
  long tooling sessions.

## Sudo Pipeline

```
if tc.Name == "shell" && containsSudo(command):
  ├─ Handler.RequestSudoApproval(command)
  │    └─ approved=false → skip with "sudo command declined by user"
  ├─ os.Setenv("BLAZE_SUDO_PASSWORD", password)
  ├─ tool.Execute(ctx, args)
  │    └─ executeShell reads BLAZE_SUDO_PASSWORD → pipes to sudo --stdin
  └─ os.Unsetenv("BLAZE_SUDO_PASSWORD")  (next iteration)
```

Password is set before each sudo command and cleared before the next tool call
to prevent cross-call leaks. Password is never stored in session JSON.

## Abort Handling

When `ctx` is cancelled mid-turn (Ctrl-C):

1. If during LLM streaming → `Provider.Stream` returns `provider.ErrAborted`
2. Assistant message is saved with whatever partial content was produced
3. `appendAbortedToolResults(resp.ToolCalls, startIdx)` — writes
   `"aborted before execution by user"` for all unexecuted tool calls
4. `appendAbortMarker()` — appends a synthetic user message:

```
User requested an urgent abort. The previous assistant turn was interrupted
before completion. Tool execution may have produced partial side effects before
cancellation. Do not continue the interrupted response. Wait for the user's
next instruction.
```

5. Returns `ErrTurnAborted` — runtime does not continue the loop

## Session Sanitization

```
sanitizeSession()
  ├─ Session.Sanitize()      → remove incomplete tool rounds
  └─ Session.Save()          → persist cleaned session
```

Called twice per turn iteration: once at the start of RunTurn and once at the
top of each tool loop iteration. Ensures the session is always valid before
sending to the LLM.

## Mode Injection

`injectDirective(messages, directive)`:
- Creates a copy of the message slice
- Finds the most recent `user` message
- Appends `"\n\n[MODE DIRECTIVE]\n" + directive` to its content
- Does NOT mutate session messages (volatile — not persisted)

This means the mode directive is injected on every LLM call iteration but never
written to session.json.

## Prompt Debug File

`prompt.json` is written to `session_folder/prompt.json` before every LLM call.
Contains the full built prompt after reasoning stripping and mode injection.
Saved with `SetEscapeHTML(false)` and `SetIndent("", "  ")`. `\n` is unescaped
for readability.

Deleted by `ResetConversation()`.

## Model Management

### SetModel (Global)

```go
func (a *Agent) SetModel(modelID string) error
```

1. `applyModel(modelID)` — validates, creates new provider client, updates in-memory
2. If in a mode → updates `CurrentMode.Model` + persists `modes.json`
3. If legacy (no modes) → updates `Config.LastModel` + persists `config.json`

### SetModelLocal (Transport-Local)

```go
func (a *Agent) SetModelLocal(modelID string) error
```

Calls `applyModel()` only — no persistence. Used by Telegram bridge to persist
model in its own `state.json`.

### SetMode

```go
func (a *Agent) SetMode(name string) error
```

1. Finds mode by name in `Modes.Modes`
2. Applies the mode's model
3. Sets `CurrentMode`, updates `Modes.LastMode`
4. Persists `modes.json`

## Work Directory

```go
func (a *Agent) SetWorkDir(dir string) error
```

Validates path exists and is a directory. Updates `WorkDir` and `Builder.WorkDir`.
On failure: returns error, keeps current directory. Used by `/cd` command.

## Conversation Reset

```go
func (a *Agent) ResetConversation() error
```

1. `Compactor.ClearSummaries(sessionFolder)` — deletes summaries/ folder
2. Remove `prompt.json` if present
3. `Active.Clear()` — unload all skills
4. `Session.Reset()` — clear messages, set closed_cleanly=false, persist

## Helper Predicates

### shouldPersistAssistantMessage

Persists if content, reasoning, or tool calls are non-empty. An empty string
from a model that produced only tool calls IS persisted (has ToolCalls).

### containsSudo

Checks for `sudo ` at command start or after `|`, `;`, `&&`, `||` with space or
tab separator. Used before `RequestSudoApproval` to determine if password is
needed.

### validateModelInConfig

Validates `provider/model_name` format and ensures the provider exists in config.
Does not call the provider endpoint — only config-level validation.

## Startup Wiring (NewAgent)

```
NewAgent(cfg, sess, os, promptsFS, workDir, handler)
  ├─ config.MigrateFromConfig()     → migrate modes from config.json if present
  ├─ config.LoadModes(modelID)      → load modes.json with fallback
  ├─ Auto-create default mode if empty
  ├─ Resolve active mode: lastMode > firstMode
  ├─ provider.NewClient(cfg, modelID)  → primary LLM client
  ├─ provider.NewClient(cfg, summarizationModel)  → secondary (if different model)
  ├─ skills.NewActiveList()         → empty active skills list
  ├─ prompt.Builder{...}            → prompt assembler
  ├─ compaction.NewManager(...)     → compaction orchestrator
  ├─ skills.DiscoverAll(workDir)    → skill resolvers
  ├─ tools.NewRegistry()
  │    ├─ shell, load_skill, unload_skill, run_skill
  │    ├─ ask_a_friend (with oneShotCaller via llmcall)
  │    ├─ replace_block, task_write, task_read
  └─ Return agent
```
