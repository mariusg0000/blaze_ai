# Shell Execution

## Source Files

| File | Role |
|------|------|
| `internal/tools/shell.go` | ShellTool, ShellArgs, executeShell, output limiter, sudo handling |
| `internal/tools/shell_process_unix.go` | Unix process group isolation (Setpgid + SIGKILL) |
| `internal/tools/shell_process_windows.go` | Windows process kill fallback |
| `internal/tools/shell_test.go` | Unit tests |
| `internal/platform/platform.go` | Shell selection per OS |

## Tool Signature

```go
type ShellArgs struct {
    Command string `json:"command"`          // required — shell input
    Timeout *int   `json:"timeout,omitempty"` // optional — seconds, default 60
    Purpose string `json:"purpose,omitempty"` // optional — 3-sentence explanation
}
```

- `Command` is required (tool returns error if empty)
- `Timeout` overrides the default 60s when set and > 0
- `Purpose` is the LLM's 3-sentence explanation displayed in console (never truncated)

## Shell Selection

Resolved per platform by `platform.SelectShell(os)`:

| OS | Primary | Fallback |
|----|---------|----------|
| Linux | `bash` | `sh` |
| macOS | `bash` | `sh` |
| Windows | `pwsh` | `powershell.exe` → `cmd.exe` |

On Unix: runs as `bash -c "<command>"`. On Windows: runs as `pwsh -Command "<command>"`.

## Execution Flow

```
ShellTool.Execute(ctx, args)
  ├─ Parse args: command, timeout, purpose
  ├─ Resolve timeout (default 60s)
  └─ executeShell(ctx, os, command, workDir, extraEnv, timeoutSec)
       ├─ Validate command non-empty
       ├─ Select shell path via platform.SelectShell
       ├─ Create timeout context (ctx + timeoutSec)
       ├─ Handle sudo: if BLAZE_SUDO_PASSWORD env var set
       │    ├─ Inject `-S` flag into sudo command
       │    ├─ Pipe password to stdin
       │    └─ Unset BLAZE_SUDO_PASSWORD from env
       ├─ exec.Command(shellPath, flag, command)
       ├─ prepareShellCommand(cmd) — Setpgid on Unix
       ├─ Start streaming stdout/stderr through output limiter
       ├─ Wait goroutine
       ├─ Select:
       │    ├─ cmd.Wait() done → format output
       │    ├─ ctx.Done() → killShellCommand() → timeout or abort
       └─ Return formatted string
```

## Output Format

Returned as a single string to the LLM:

```
exit_code: 0
stdout:
<output content>

stderr:
<error content>
```

- `exit_code: N` always included
- `stdout:` and `stderr:` sections only present if they have content
- `stderr:` has `[sudo] password for ...:` lines stripped (noise from cached sudo credentials)
- On abort: `"aborted by user"` with partial stdout/stderr
- On timeout: `"timeout <N>s exceeded"`

## Output Size Limit

`MaxShellOutputBytes = 150 * 1024` (150kB) combined stdout + stderr.

The limiter (`shellOutputLimiter`) tracks total bytes across both streams. When
the limit is reached:
1. The command process is killed (kills parent + children on Unix)
2. Subsequent bytes are discarded
3. Returned message:
   ```
   error: shell output exceeded the 150 kB limit and was stopped.
   stdout_bytes=<N> stderr_bytes=<N> limit_bytes=153600.
   Refine the command to a narrower path, pattern, or depth,
   or read the target in sequential chunks below 150 kB.
   ```

The LLM is expected to retry with narrower scope.

## Timeout Handling

Default: 60 seconds. Adjustable per call via `timeout` parameter.

When timeout fires:
1. `killShellCommand(cmd)` is called
2. On Unix: sends `SIGKILL` to the full process group (parent + all children)
3. On Windows: sends `Process.Kill()` to the shell parent
4. The Wait goroutine is drained before returning the timeout message
5. Returns: `"timeout <N>s exceeded"`

Process group isolation (`Setpgid: true` on Unix) ensures background children
(`&`, `nohup`) are killed on timeout, preventing orphaned processes from holding
stdout/stderr pipes open.

## Sudo Handling

Sudo execution flows through the `RequestSudoApproval` handler:

1. Runtime detects `sudo` in the command string before execution
2. Transport prompts user: approved + password, or declined
3. If approved: password is set as `BLAZE_SUDO_PASSWORD` env var
4. Shell tool reads the env var, injects `-S` (stdin password) flag, pipes password
5. Env var is unset immediately after read
6. Password never enters session JSON or prompt text
7. On decline: tool call skipped, returns `"aborted: user declined sudo approval"`

The `stripSudoPasswordPrompt()` function removes `[sudo] password for <user>:`
lines from stderr to prevent the LLM from misinterpreting cached-credential
prompts as errors.

## User Abort (Ctrl-C)

User abort via Ctrl-C:
1. Context cancelled by transport
2. `executeShell` detects `ctx.Done()`
3. Child process killed via process group (Unix) or Process.Kill (Windows)
4. Returns: `"aborted by user"` with any partial stdout/stderr

## Work Directory

The shell tool does not track a work directory internally. Work directory is
injected via the `workDir` parameter to `executeShell()`. Currently unused
by the tool — commands run in the process's current working directory. The
prompt tells the LLM to use `cd` within shell commands.

## Extra Environment

`executeShell` accepts `extraEnv` (map of key=value), merged with `os.Environ()`
before execution. Used for injecting sudo password and potentially other
transport-specific environment variables.

## Shell Description (LLM-Facing)

```
"command → execute via host shell; output = stdout + stderr + exit_code"
```

Schema requires both `purpose` and `command`. The `purpose` description
mandates exactly 3 user-visible sentences explaining what, how, and why.
