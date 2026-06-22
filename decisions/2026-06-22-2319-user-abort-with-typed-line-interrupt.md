# Session Decision Summary: User Abort With Typed-Line Interrupt

Date: 2026-06-22 23:19
Base commit: be6d9f6

## Context
User required a full turn-abort mechanism usable at any point: during LLM streaming, tool execution, or idle. The abort must preserve partial history in the session so the next turn sees what happened before interruption. Additionally, typing a new message during an active turn should abort the current turn and automatically start a new turn with the typed message.

## Changes Made
1. **Context propagation** — `runtime.RunTurn`, `provider.Stream`, and all `tool.Execute` methods now accept `context.Context`. The cancellation chain is: console → runtime → provider HTTP request → tool subprocess.
2. **Provider abort** — `Client.Stream` uses `http.NewRequestWithContext`. On cancel it returns partial content/tool-calls and `provider.ErrAborted`. Incomplete tool-call fragments are discarded by `finalizeToolCalls`.
3. **Runtime abort** — `RunTurn` returns `ErrTurnAborted` when context is cancelled. The session is preserved at the point of interruption: partial assistant content + complete tool calls + tool results (real or "aborted by user") + a `user` abort marker. Aborted tools that were not yet executing get "aborted before execution by user".
4. **Shell cancellation** — `ShellTool.Execute` derives timeout from the turn context. On user cancel it kills the process group and returns partial stdout/stderr wrapped in "aborted by user".
5. **Console async input** — `Run()` starts a background goroutine reading lines continuously. During an active turn, `runAgentTurn` selects on: agent completion, `Ctrl+C`, or a new typed line. A typed line triggers abort, persists the marker, and stores the line as `nextInput` for the REPL loop to process immediately.
6. **Compaction** — `summarize()` adapted to pass `context.Background()` to `Stream`.
7. **Tests**: provider partial abort, runtime stream abort persistence, runtime active tool abort persistence, shell user-cancel, console typed-line interrupt.

## Decisions And Rationale
- Abort preserves history: the interrupted turn (user message, partial assistant, tool calls/results) stays in session. This lets the next turn know what side-effects may have occurred.
- Abort appends a `user`-role marker: "User requested an urgent abort..." but does **not** auto-trigger an LLM call. The user or the next typed line drives the continuation.
- Typed-line interrupt captures the new message before abort and runs it immediately after the interrupted turn closes, creating a seamless "replace my last input" experience.
- `Ctrl+C` only aborts; it doesn't capture a replacement message. `Ctrl+C` during an idle prompt behaves as normal OS interrupt (closes the app).
- Shell timeout and user-cancel are differentiated: timeout returns "timeout Ns exceeded"; user-cancel returns "aborted by user" with partial output.

## Implementation Approach
- Added `context.Context` parameter to `Agent.RunTurn`, `Client.Stream`, and all `Tool.Execute` implementations.
- Added `ErrTurnAborted` (runtime) and `ErrAborted` (provider) sentinel errors.
- Added `shouldPersistAssistantMessage`, `appendAbortMarker`, `appendAbortedToolResults` helpers in runtime.
- Replaced REPL synchronous `ReadLine()` loop with async goroutine feeding an `inputEvent` channel.
- `runAgentTurn` selects over agent completion, interrupt signal, and input channel. On typed line: cancels context, stores `nextInput`, returns `ErrTurnAborted`.
- REPL loop checks `nextInput` after the turn and processes it as the next iteration.
- Added `turnAborting` atomic guard on Console to suppress late callbacks from the cancelled turn.
- Shell's `formatAbortedToolOutput` captures partial stdout/stderr before kill.

## Alternatives Considered
- Discard interrupted turn entirely: rejected because user wants tool side-effects visible in history.
- Auto-trigger LLM after abort: rejected because user should control the next step.
- `Esc` key for abort: rejected because it requires raw terminal mode; `Ctrl+C` is simpler and reliable.

## Files Included
- `internal/tools/tools.go`: Tool interface `Execute(ctx, args)` signature
- `internal/tools/shell.go`: context-derived timeout, process group kill on cancel, partial output
- `internal/tools/skill_tools.go`: `Execute(ctx, args)` signature + pre-check
- `internal/tools/replace_block.go`: `Execute(ctx, args)` signature + pre-check
- `internal/provider/provider.go`: `Stream(ctx, ...)`, `ErrAborted`, `finalizeToolCalls`
- `internal/runtime/runtime.go`: `RunTurn(ctx, input)`, `ErrTurnAborted`, abort persistence helpers
- `internal/console/console.go`: async input reader, typed-line interrupt, `runAgentTurn`, `abortCurrentTurn`
- `internal/compaction/compaction.go`: adapt `summarize` to new `Stream` signature
- Test files: all Execute/Stream/RunTurn call sites updated to pass context; new abort-specific tests

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
