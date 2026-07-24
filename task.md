# Task: Harden active-turn reliability and cancellation

status: completed
created: 2026-07-24

## User outcome

Audit and harden BlazeAI so that transient LLM failures are retried only when
safe, terminal failures stop promptly, agent and tool timeouts have explicit
semantics, and ESC interrupts every operation inside an active console turn,
including sudo approval and shell processes.

Implementation is not authorized until the user answers `proceed` or `go`.

## Audit evidence

Supporting reports:

- `.agents/explorer/260724-llm-retry-timeouts.md`
- `.agents/explorer/260724-agents-tools-cancel.md`
- `.agents/explorer/260724-console-esc.md`
- `.agents/explorer/260724-timeout-coverage.md`
- `.agents/explorer/260724-approval-contract.md`
- `.agents/explorer/260724-compaction-contract.md`
- `.agents/explorer/260724-windows-shell-contract.md`

### Critical findings

1. Provider streaming has no retry. A transient 429, 5xx, network failure, or
   stream interruption ends the turn immediately. No status-code policy exists.
2. The ChatGPT Responses SSE parser has no idle timeout. A connection can stay
   open indefinitely after the last event.
3. Retry must not replay a response that already emitted content, reasoning, or
   tool calls; replay could duplicate visible output or side effects.
4. Compaction calls the summarizer with `context.Background()`. ESC cannot
   cancel a summarization already in progress.
5. Compaction silently skips summarization below the hard cap and force-prunes
   above it. This is a fallback and conflicts with the project rule that
   missing/failed required work must surface an error.
6. Tool timeouts are local to shell, helper, `ask_a_friend`, and image calls;
   they return a result string to the LLM. They are not automatic retries.
7. Child agents already have a 2-minute inactivity timeout and a 20-minute
   overall timeout, with optional positive `timeout:` front matter. Parent
   cancellation propagates, but actual timer paths have little test coverage.
8. ESC already cancels model streaming, normal tool execution, child agents,
   and Unix shell process groups. It is not reliable during sudo approval:
   approval reads `/dev/tty` synchronously, drops ESC, and receives no context.
9. Windows shell cancellation kills only the shell parent; background children
   can survive. Unix process-group cancellation already kills the full group.
10. Telegram polling has its own infinite transient-error retry and remains
    outside this active-turn provider policy.

## Resolved behavior and policy

### Provider retry policy

Apply one policy to provider streaming calls, which also covers the secondary
LLM and compaction calls because they use the provider client:

- Three total attempts: the initial attempt plus at most two retries.
- Wait one second before retry one and two seconds before retry two. Each wait
  must select on the caller context; cancellation stops the wait immediately.
- Retry only when no semantic response data has been received yet and the
  failure is one of:
  - HTTP 408, 429, or 500-599;
  - a temporary/timeout network error;
  - `io.EOF` or `io.ErrUnexpectedEOF` before semantic data;
  - the provider stream idle timeout before semantic data.
- Never retry after content, reasoning, or a tool call has been accumulated.
  This includes a partial stream that later fails or idles.
- Never retry `context.Canceled`, `context.DeadlineExceeded`, user abort,
  authentication/authorization errors, invalid requests, other 4xx statuses,
  provider `response.failed`/error events, malformed response errors, or a
  retry budget exhaustion.
- After the budget is exhausted, return the provider error to the turn. Do not
  switch models, invent a response, replay tools, or silently continue.
- Preserve existing partial-response and user-abort persistence semantics.

The retry loop stays in `Client.StreamWithPhase`; no new exported provider API
or user configuration key is introduced. The existing
`providerStreamIdleTimeout` remains the single idle limit for both SSE paths.

### Abandon and timeout policy

- User ESC/Ctrl-C: abort immediately, never retry, persist already received
  assistant content, mark unexecuted tool calls as aborted, append the existing
  abort marker, and return `runtime.ErrTurnAborted`.
- Provider terminal error or exhausted safe retries: fail the turn with the
  provider error; do not invoke another LLM or replay a tool call.
- A shell/helper/secondary/image tool timeout: end only that tool invocation
  and return its explicit timeout result to the current LLM; do not retry the
  tool automatically. The LLM may decide the next step.
- Child-agent inactivity or overall timeout: terminate that child, return its
  existing formatted timeout result to the parent tool call, and do not
  automatically restart the child. Parent execution may continue unless the
  user cancels the parent turn.
- Parent user cancellation: propagate to every child, provider call, tool, shell
  process, approval reader, and compaction call; classify it as cancellation,
  not as a child timeout or provider retryable error.
- Compaction summarization failure, empty summary, or cancellation is surfaced;
  no silent skip and no force-prune fallback is allowed. The session must not
  be pruned when the required summary was not produced.
- Existing 60-second tool defaults, 2-minute child inactivity timeout,
  20-minute child overall default, per-agent positive `timeout:` override, and
  150 KB shell output cap remain unchanged in this task.

### ESC contract

The scope is every blocking operation nested in an active console turn. Idle
readline input, startup setup, Telegram polling, and process exit shortcuts are
not changed.

- Keep the existing turn abort watcher for streaming, tools, and shells.
- Make `RequestSudoApproval` context-aware and error-returning. ESC or Ctrl-C
  in approval input must return `context.Canceled`; cancellation from the turn
  context must terminate the approval read and restore terminal state.
- Make approval and hidden-password readers context-aware. Their byte loop must
  recognize ESC instead of dropping it, and must preserve raw-mode restoration
  and file cleanup on every return path.
- Runtime must distinguish approval cancellation from a normal sudo decline:
  cancellation uses the standard turn-abort path; decline remains a tool error
  and does not abort the turn; non-cancellation reader errors fail the turn.
- Unix shell cancellation continues to kill the complete process group and wait
  for `cmd.Wait()` before returning partial output.
- Windows shell cancellation must terminate the shell process tree as well as
  the root process. Keep the existing unexported helper signatures and use the
  native Windows tree-termination operation before returning from the helper.

## Exact implementation scope

Only the following repository paths may be created or modified during
implementation. `task.md` is owned by Architect; all other writes belong to
Coder.

- `internal/provider/provider.go` — modify provider stream retry classification,
  bounded context-aware backoff, and shared idle-timeout behavior.
- `internal/provider/openai_responses.go` — modify ChatGPT SSE parsing to use
  the standard idle timeout and preserve partial-response cancellation rules.
- `internal/provider/provider_test.go` — add retry classification, retry budget,
  partial-response no-retry, and cancellation-during-backoff tests.
- `internal/provider/openai_responses_test.go` — add ChatGPT idle-timeout and
  no-retry-after-data tests.
- `internal/compaction/compaction.go` — thread context through `Compact`,
  `summarize`, and the provider call; surface summary errors and empty output
  instead of skipping or force-pruning.
- `internal/compaction/compaction_test.go` — update fallback expectations and
  add cancellation/no-prune assertions.
- `internal/runtime/runtime.go` — pass turn context into compaction, handle
  cancelled compaction with the existing abort persistence path, and consume
  the context-aware approval result.
- `internal/runtime/runtime_test.go` — add cancelled-compaction and cancelled-
  sudo-approval assertions; update Handler test doubles.
- `internal/runtime/agent_orchestration.go` — update Handler forwarding and
  stubs for the approval signature only; keep child limits, ordering, and
  cleanup unchanged.
- `internal/runtime/agent_orchestration_test.go` — add parent-cancellation and
  configured-overall-timeout assertions without changing production limits.
- `internal/console/console.go` — pass context into approval, map approval
  cancellation to the turn-abort contract, and preserve normal decline/error
  behavior.
- `internal/console/reader.go` — add context parameters to approval/password
  reads, recognize ESC as cancellation, and preserve raw terminal cleanup.
- `internal/console/reader_test.go` — add input cancellation assertions if the
  existing reader seams permit them; do not add an unrelated input abstraction.
- `internal/console/reader_input_unix.go` — create only if required by the
  existing platform split for nonblocking context-aware terminal-byte polling.
- `internal/console/reader_input_windows.go` — create only if required by the
  existing platform split for context-aware console-byte polling.
- `internal/console/console_test.go` — update handler doubles and add approval
  cancellation/decline rendering assertions where existing seams permit.
- `internal/telegram/handler.go` — update the stub to the new Handler method
  signature; Telegram continues to deny sudo.
- `internal/tools/shell_process_windows.go` — modify tree termination while
  preserving `prepareShellCommand(*exec.Cmd)` and
  `killShellCommand(*exec.Cmd)` signatures.
- `internal/tools/shell_test.go` — retain Unix timeout/abort assertions and add
  only platform-appropriate process-tree coverage.

Do not modify prompts, configuration schemas, public provider APIs, timeout
values, Telegram retry behavior, session file formats, or unrelated tools.
Do not add fallback models, generic retry frameworks, new global state, or
unrequested abstractions.

## Implementation recipe

1. **Provider: bounded safe retries**
   - Preserve `Client.Stream` and `Client.StreamWithPhase` signatures.
   - Wrap the existing request/parse attempt in `StreamWithPhase` with the
     exact three-attempt policy above.
   - Classify HTTP status before returning the existing status error. Classify
     transport/EOF/idle errors only when the accumulated response has empty
     content, reasoning, and tool calls.
   - Use a context-aware timer for each fixed backoff. Return `ErrAborted` when
     the caller cancels during an attempt or backoff.
   - Reset per-attempt phase reporting and do not emit duplicate semantic
     callbacks from a retry because retries are allowed only before data.
   - Make `parseChatGPTSSE` use `providerStreamIdleTimeout`, reset it for each
     received line, close the response body on idle/cancel, and return partial
     data plus the established abort sentinel on cancellation.

2. **Compaction: context and fail-fast semantics**
   - Change signatures exactly to:
     - `func (m *Manager) Compact(ctx context.Context, sess *session.Session, usage *provider.Usage) (bool, error)`
     - `func (m *Manager) summarize(ctx context.Context, sessionFolder string, pruned []session.Message) (string, error)`
     - `func (a *Agent) compact(ctx context.Context, usage *provider.Usage) (bool, error)`
   - Pass the `RunTurn` context at the existing compaction call site and use it
     for the summarizer provider stream.
   - If summarization returns an error, return it even below the hard cap. If
     the response content is empty, return an explicit error. Do not save a
     pruned session in either case.
   - If compaction returns a context cancellation/deadline, reuse the existing
     `appendAbortedToolResults`, `appendAbortMarker`, and `ErrTurnAborted` path;
     do not report it as an ordinary maintenance failure.

3. **Approval and ESC cancellation**
   - Change the Handler method exactly to:
     `RequestSudoApproval(ctx context.Context, command string) (approved bool, password string, err error)`.
   - Update `Console`, Telegram and child stubs, `activityForwarder`, and both
     test mocks. Pass the existing turn context from the runtime tool loop.
   - Change `ReadApproval`, `ReadHiddenInput`, and the private terminal-line
     routine to accept context. Preserve `/dev/tty` ownership, raw-mode
     restoration, and close defers.
   - Use a small platform-specific byte-read helper only if needed: Unix polls
     the raw terminal with a 10 ms wait and Windows waits for console input with
     the same 10 ms cadence. Both check context on every poll. The line reader
     maps byte `0x1b` and Ctrl-C to `context.Canceled`.
   - At the runtime call site, context cancellation or `context.Canceled` from
     approval invokes the same abort-history path as other user aborts. A normal
     `approved == false` result remains `error: sudo command declined by user`.
     Any other approval error is returned as a terminal `sudo approval failed`
     error and never executes the shell.

4. **Shell and agent boundaries**
   - Leave the existing turn-context-to-tool flow and Unix process-group kill
     intact; add regression assertions that ESC/parent cancellation reaches a
     shell and that `cmd.Wait()` completes before the tool result returns.
   - Preserve child-agent first-error sibling cancellation, ordered results,
     inactivity reset on tool activity, and overall deadline behavior.
   - Add tests for parent cancellation classification and a short positive
     agent `timeout:` value; do not add automatic child retries.
   - On Windows, preserve both helper signatures and terminate the complete
     process tree before the shell wait is released.

## Required tests and exact assertions

- Provider transient status: first response 503, second response success;
  assert exactly two requests and the successful response.
- Provider permanent status: 401; assert exactly one request and the original
  status error.
- Provider retry exhaustion: three transient failures; assert exactly three
  requests and the final provider error.
- Provider partial stream: emit content/tool data, then fail; assert one
  request, partial response preservation, and no retry.
- Provider cancel during backoff: cancel after the first transient failure;
  assert `errors.Is(err, provider.ErrAborted)` and no second attempt.
- ChatGPT idle stream: shorten `providerStreamIdleTimeout` in the test, leave
  the stream open without events, and assert an idle-timeout error.
- Compaction failure below and above hard cap: assert an error, no summary file,
  no pruning, and unchanged session messages.
- Compaction cancellation: cancel the supplied context; assert the provider
  receives cancellation and the compaction result is classified as cancellation.
- Runtime cancellation during compaction: assert `ErrTurnAborted`, one abort
  marker, and no unexecuted tool call is run.
- Runtime sudo approval cancellation: fake approval returns
  `context.Canceled`; assert `ErrTurnAborted`, no shell execution, and the
  existing abort marker/history behavior. A normal decline must remain a tool
  result and must not return `ErrTurnAborted`.
- Reader cancellation: assert ESC and Ctrl-C map to `context.Canceled`, a
  cancelled context exits promptly, and terminal cleanup is retained.
- Child overall timeout and parent cancellation: assert the existing formatted
  timeout/cancelled distinctions and no automatic second child run.
- Existing shell timeout, user-abort partial output, output cap, and Unix
  background-child-kill tests must continue to pass.
- Windows tools package must compile with:
  `GOOS=windows GOARCH=amd64 go test -c -o /tmp/blazeai-tools.test.exe ./internal/tools`.

## Verification commands

1. `go test ./internal/provider ./internal/compaction ./internal/runtime ./internal/console ./internal/tools ./internal/telegram`
2. `go test ./...`
3. `GOOS=windows GOARCH=amd64 go test -c -o /tmp/blazeai-tools.test.exe ./internal/tools`

## Acceptance criteria

- A transient provider failure is retried only before semantic output, at most
  twice, with context-aware 1s/2s waits; permanent and partial-stream errors
  are not retried.
- Both provider SSE implementations have the same 180-second idle behavior.
- No provider/model fallback or tool/agent replay exists.
- Compaction uses the turn context and never silently skips or force-prunes on a
  failed/empty summary.
- ESC/Ctrl-C aborts provider streaming, compaction, every nested tool and child
  agent, sudo approval, and Unix/Windows shell process trees in an active turn.
- Sudo decline remains distinct from cancellation and does not abort the turn.
- Agent timeout values and existing tool timeout values remain unchanged, with
  explicit tested classification and no automatic retries for side-effecting
  tools or agents.
- All requested verification commands pass after the final write.
- No path outside the allowlist is changed.

## Unresolved questions

None for the defined active-turn scope. Idle readline/startup ESC behavior and
Telegram polling retry are explicitly out of scope.

## Stop conditions

Stop before editing if implementation requires a new public API, a new config
key, a timeout-value change, a session-format change, a fallback, or a write
path outside this task. Resolve that scope change with the user first.

## Completion

Accepted outcome: transient provider failures now use bounded safe retries;
both SSE paths have idle cancellation; compaction propagates turn context and
fails without pruning when summarization fails; active-turn ESC/Ctrl-C reaches
approval, tools, agents, compaction, and shell cancellation; and Windows shell
termination targets the process tree.

Changed paths:

- `internal/provider/provider.go`
- `internal/provider/openai_responses.go`
- `internal/provider/provider_test.go`
- `internal/provider/openai_responses_test.go`
- `internal/compaction/compaction.go`
- `internal/compaction/compaction_test.go`
- `internal/runtime/runtime.go`
- `internal/runtime/runtime_test.go`
- `internal/runtime/agent_orchestration.go`
- `internal/runtime/agent_orchestration_test.go`
- `internal/console/console.go`
- `internal/console/reader.go`
- `internal/console/reader_input_unix.go`
- `internal/console/reader_input_windows.go`
- `internal/console/console_test.go`
- `internal/telegram/handler.go`
- `internal/tools/shell_process_windows.go`

Verification results:

- `go test ./internal/provider` — PASS
- `go test ./internal/provider -run 'Test(Stream|Client|NewClient)'` — PASS
- `go test ./internal/compaction` — PASS
- `go test ./internal/runtime` — PASS
- `go test ./internal/console` — PASS
- `go test ./internal/telegram` — PASS
- `go test ./internal/tools` — PASS
- `go test ./internal/provider ./internal/compaction ./internal/runtime ./internal/console ./internal/tools ./internal/telegram` — PASS
- `go test ./...` — PASS
- `GOOS=windows GOARCH=amd64 go test -c -o /tmp/blazeai-tools.test.exe ./internal/tools` — PASS

Remaining issues: None within the approved active-turn scope.
