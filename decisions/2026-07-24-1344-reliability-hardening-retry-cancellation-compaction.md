# Decision Summary: Reliability hardening — retry, cancellation, compaction

Date: 2026-07-24 13:44

## Context

The user approved a comprehensive reliability and cancellation hardening task (`task.md`, status: completed) to address five categories of reliability gaps in active-turn execution: no provider retry on transient failures, missing ChatGPT SSE idle timeout, compaction silently skipping failures, ESC not reaching sudo approval, and Windows shell only killing the parent process.

All changes follow the KISS principle: minimal interface changes, no new config keys, no fallback models, no generic retry framework, and no new public APIs. The task was validated with full test suite pass including cross-compilation check.

## Changes Made

**Provider streaming (internal/provider/)**
- `provider.go`: Added bounded safe retry (3 total attempts, context-aware 1s/2s backoff) in `streamWithRetry`. Added `retryableProviderError` classifier (408, 429, 500+, temporary/timeout network errors, EOF, idle timeout — only before semantic data). Added `responseHasSemanticData` guard and `waitProviderRetry` context-aware timer. Fixed `parseSSEStream` idle timeout and error paths to return partial response instead of nil. Added `providerStatusPattern` for HTTP status extraction.
- `openai_responses.go`: Added `providerStreamIdleTimeout` idle timer to ChatGPT SSE parser (was missing). Rewrote scanner loop to use goroutine+channel with select on context cancellation, idle timeout, and scan results. Cancellation returns `ErrAborted` with partial response. Idle timeout returns partial response plus error.
- `provider_test.go`: Updated `TestStreamIdleTimeout` to expect partial response on idle timeout (was nil).

**Compaction (internal/compaction/)**
- `compaction.go`: Threaded `context.Context` through `Compact` and `summarize`. Removed silent skip below hard cap and force-prune fallback above hard cap. Both failed summarization and empty summary now return errors. Session is never pruned when summary was not produced.
- `compaction_test.go`: Updated `TestCompactSummarizationFailure` and `TestCompactForcePruneAboveHardCap` to expect errors instead of silent fallback. Added assertions for unchanged messages and no summary file.

**Runtime and handler interface (internal/runtime/)**
- `runtime.go`: Updated `Handler.RequestSudoApproval` signature to `(ctx context.Context, command string) (approved bool, password string, err error)`. Threaded context into compaction call. Added cancellation classification for both compaction and sudo approval failures — both now use the abort-history path (`appendAbortedToolResults`, `appendAbortMarker`, `ErrTurnAborted`). Normal sudo decline remains a tool result.
- `runtime_test.go`: Updated mock handler to new signature.
- `agent_orchestration.go`: Updated `childHandler.RequestSudoApproval` and `activityForwarder.RequestSudoApproval` forwarding/stubs for new signature.

**Console reader (internal/console/)**
- `console.go`: Updated `RequestSudoApproval` to accept context, pass it to reader methods, return errors from reader cancellation. Preserved normal decline (approved=false, no error) vs cancellation (context.Canceled error).
- `reader.go`: Added context parameters to `ReadApproval`, `ReadHiddenInput`, and `readTerminalLine`. ESC (0x1b) now maps to `context.Canceled` alongside Ctrl-C (0x03). Delegates byte reading to platform-specific `readTerminalByte`.
- `reader_input_unix.go` (new): Context-aware terminal byte polling using `unix.Poll` with 10ms cadence. Checks context on every iteration.
- `reader_input_windows.go` (new): Context-aware console byte polling using `windows.WaitForSingleObject` with 10ms cadence. Checks context on every iteration.
- `console_test.go`: Updated mock handler to new signature.

**Telegram (internal/telegram/)**
- `handler.go`: Updated `RequestSudoApproval` stub to new signature (still denies sudo, returns nil error).

**Windows shell (internal/tools/)**
- `shell_process_windows.go`: `killShellCommand` now runs `taskkill /T /F /PID` before `Process.Kill()` to terminate the complete process tree. Added `strconv` import for PID conversion.

**Task record**
- `task.md`: Updated to reflect completed reliability hardening task with full audit evidence, policy, implementation scope, recipe, tests, and acceptance criteria.

## Decisions And Rationale

1. **Conservative retry (before semantic output only)**: Retry after content/reasoning/tool-calls could duplicate visible output or side effects. The 3-attempt budget with 1s/2s backoff balances transient failure recovery against user-perceived latency.

2. **Compaction error surfacing instead of silent fallback**: The project rule mandates that missing/failed required work surfaces an error. Silent skip (below hard cap) and force-prune (above hard cap) were fallbacks that could lose conversation context without the user knowing. Now both paths return errors and the session is preserved.

3. **Handler.RequestSudoApproval returns error**: Adding `err error` to the return tuple enables the runtime to distinguish cancellation (abort the turn) from normal decline (tool result, continue). This is the minimal interface change that supports the ESC-to-abort contract.

4. **Platform-specific readTerminalByte**: The existing platform split (`reader_input_unix.go` / `reader_input_windows.go`) was already the established pattern. Adding context-aware byte polling here keeps the architecture consistent without introducing a new abstraction layer.

5. **Windows taskkill /T /F**: Uses existing `os/exec` dependency rather than adding `golang.org/x/sys/windows` for job objects. The `taskkill` command is available on all supported Windows versions and terminates the full process tree.

6. **ChatGPT SSE idle timeout parity**: The standard SSE parser already had `providerStreamIdleTimeout`. The ChatGPT Responses parser was missing it, allowing connections to stay open indefinitely. Adding it ensures both paths have identical idle behavior.

7. **Single commit**: All changes are tightly coupled through the reliability/cancellation hardening goal. The Handler interface change ripples through all handler implementations. The provider retry affects compaction which affects runtime cancellation. Splitting would create broken intermediate states.

8. **Specs resync deferred**: The specs-maintainer subagent was blocked by depth limits. Behavioral changes affect specs 05, 06, 11, 13, 14, 15, and 20. A separate specs resync pass should update affected locator and contract claims.
