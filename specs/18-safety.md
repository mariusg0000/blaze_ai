# Safety

## Source Files

| File | Role |
|------|------|
| `internal/runtime/runtime.go` | Sudo pipeline, containsSudo, abort handling |
| `internal/tools/shell.go` | MaxShellOutputBytes, shellOutputLimiter, executeShell, stripSudoPasswordPrompt, formatAbortedToolOutput |
| `internal/tools/shell_process_unix.go` | Process group isolation, SIGKILL on timeout |
| `internal/tools/shell_process_windows.go` | Windows fallback (no-op process group) |
| `internal/console/reader.go` | ReadHiddenInput (no-echo) |
| `internal/telegram/handler.go` | No-sudo policy |

## Overview

Safety mechanisms protect the user from destructive operations and inadvertent
data leaks. They exist in three categories: shell execution safety, privilege
elevation, and secret handling.

## Shell Output Cap

All shell commands have a hard output cap of `150 kB` combined stdout + stderr.

```go
const MaxShellOutputBytes = 150 * 1024
```

### Mechanism

1. `shellOutputLimiter` is created with `maxBytes = 150 * 1024`
2. Both stdout and stderr are written through `limitedStreamWriter` instances
   that share the same limiter
3. The limiter tracks total bytes across both streams in a thread-safe counter
4. When either stream reaches the limit:
   - `onExceeded` callback calls `killShellCommand()` (SIGKILL to process group)
   - Subsequent bytes from both streams are silently discarded
5. After `cmd.Wait()`, if the limiter was exceeded, returns:

```
error: shell output exceeded the 150 kB limit and was stopped.
stdout_bytes=<N> stderr_bytes=<N> limit_bytes=153600.
Refine the command to a narrower path, pattern, or depth, or read the
target in sequential chunks below 150 kB.
```

### Purpose

- Prevents runaway commands from flooding session history
- Protects against binary/file output that would waste context window
- Gives the LLM explicit guidance to narrow its approach

## Timeout

Default timeout: 60 seconds. Configurable per tool call via `timeout` parameter.

### Mechanism

```go
ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
defer cancel()
```

1. When `ctx.Done()` fires due to `DeadlineExceeded`:
   - `killShellCommand()` sends SIGKILL to the process group
   - Waits for `cmd.Wait()` to complete
   - Returns `"timeout <N>s exceeded"`

### User Abort

When the user cancels (Ctrl-C), the agent's context is cancelled:
1. `ctx.Done()` fires (not `DeadlineExceeded`)
2. `killShellCommand()` sends SIGKILL
3. Returns partial output captured before cancellation with `"aborted by user"`

### Process Group Isolation (Unix)

```go
func prepareShellCommand(cmd *exec.Cmd) {
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killShellCommand(cmd *exec.Cmd) {
    _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
    _ = cmd.Process.Kill()
}
```

- `Setpgid: true` puts the shell command in its own process group
- `syscall.Kill(-pid, SIGKILL)` sends to the entire process group (not just the parent)
- This kills background children (`sleep 30 &`) that would otherwise keep pipes open
- Without this, timeout could hang because `cmd.Wait()` blocks on pipe EOF

## Sudo Pipeline

### Detection

`containsSudo(command)` checks for `sudo ` at:
- Start of string
- After pipe `| sudo `
- After semicolon `; sudo `
- After `&&` or `||`

Space and tab separators are handled.

### Flow

```
if tc.Name == "shell" && containsSudo(command):
  ├─ Handler.RequestSudoApproval(command)
  │    ├─ Console: show command + "[y/N]", read hidden password
  │    └─ Telegram: return false (no-sudo policy)
  │
  ├─ approved=false → skip with "sudo command declined by user"
  │
  ├─ os.Setenv("BLAZE_SUDO_PASSWORD", password)
  │
  ├─ executeShell():
  │    ├─ Read BLAZE_SUDO_PASSWORD, Unsetenv immediately
  │    ├─ Insert "-S" flag if not present: "sudo " → "sudo -S "
  │    ├─ Pipe password + newline through cmd.Stdin
  │    └─ Execute
  │
  └─ Cross-call cleanup:
       ├─ os.Unsetenv("BLAZE_SUDO_PASSWORD") before each tool call
       └─ executeShell unsets after reading (belt and suspenders)
```

### Stderr Filtering

`stripSudoPasswordPrompt()` removes `[sudo] password for <user>:` lines from
stderr output. When sudo uses cached credentials, this prompt still appears on
stderr even though the command succeeds. The LLM would otherwise interpret any
`password` text in stderr as an error condition.

## Secrets

| Secret | Storage | In Session JSON | In Prompt |
|--------|---------|-----------------|-----------|
| API keys | `config.json` (0600) | Never | Never |
| Sudo password | env var per tool call | Never | Never |

### API Keys

- Stored in `config.json` with 0600 permissions (owner read/write)
- Used only at provider client creation time
- Never appear in session JSON or prompt text
- Transmitted over HTTPS to the configured endpoint

### Sudo Password

- Set as `BLAZE_SUDO_PASSWORD` env var immediately before the tool call
- Cleared immediately after executeShell reads it (`os.Unsetenv`)
- Re-cleared at the start of each tool call iteration (`os.Unsetenv`)
- Never stored in session JSON
- Entered via hidden input (no echo on terminal)
- Telegram bridge: always returns `approved=false` (no sudo over chat)

## Backups

Backups are an LLM decision, not a runtime-enforced rule. The agent may choose
to create backups of files before modifying them. Backups are stored in
`app_home/backups/`.

No automatic backup retention or cleanup in v1.

## Transport-Specific Safety

| Transport | Sudo Policy | Password Input |
|-----------|-------------|----------------|
| Console | Allowed, user prompted | Hidden terminal input (raw mode, no echo) |
| Telegram | Denied (always returns false) | N/A (no channel available) |
