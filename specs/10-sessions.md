# Sessions

## Source Files

| File | Role |
|------|------|
| `internal/session/session.go` | Message/Session types, Create, Load, Save, Append, AppendAll, AppendReadFileResult, Close, Reset, Sanitize, LastClean, Last |
| `internal/session/usage.go` | RecordUsage, UsageReport, UsageSnapshot, CacheKeyForSession — private token analytics |
| `internal/session/session_test.go` | Unit tests |
| `internal/session/doc.go` | Package docs |

## Overview

No database. Sessions are file-based — one folder per session containing
`session.json`. The session folder lives under the project's sessions
directory: `app_home/projects/<project>/sessions/<random-name>/`.

Session folders persist indefinitely. No automatic cleanup.

## Message Format

```go
type Message struct {
    Role               string      `json:"role"`                    // system, user, assistant, tool
    Content            interface{} `json:"content,omitempty"`        // message text or tool result
    Reasoning          string      `json:"reasoning_content,omitempty"` // reasoning text (kept intact on disk)
    ReasoningPresent   bool        `json:"-"`                        // forces reasoning_content to remain even when empty
    ReasoningEncrypted string      `json:"-"`                        // encrypted reasoning continuation data
    ToolCalls          interface{} `json:"tool_calls,omitempty"`     // assistant tool call array
    ToolCallID         string      `json:"tool_call_id,omitempty"`  // tool result reference
    Name               string      `json:"name,omitempty"`           // tool name for results
}
```

Follows the OpenAI chat message format exactly as sent/received. Reasoning
parts are stored intact on disk and stripped only from the LLM payload.
`ReasoningPresent` preserves the `reasoning_content` field as an empty string
when the reasoning was stripped from the payload. `ReasoningEncrypted` stores
Responses API encrypted reasoning data for providers that require it when
continuing a tool loop.

Follows the OpenAI chat message format exactly as sent/received. Reasoning
parts are stored intact on disk and stripped only from the LLM payload.

## Session Structure

```go
type Session struct {
    Messages      []Message `json:"messages"`       // full conversation array
    ClosedCleanly bool      `json:"closed_cleanly"` // true only on /exit
    Folder        string    `json:"-"`               // absolute path, excluded from JSON
}
```

- `ClosedCleanly` is set `true` only by `Close()` (`/exit` command)
- `Folder` is excluded from JSON serialization (reconstructed on Load)
- `-c` flag resumes only sessions with `closed_cleanly: true`
- `-r` flag resumes most recent session regardless of clean status

## Storage Path

Sessions are stored under `app_home/projects/<project>/sessions/`.

The project directory is resolved from the current working directory at session
creation time via `platform.EnsureProjectDir(workDir)`. This means sessions are
scoped to the project context — different projects have independent session
histories.

## Folder Naming

Format: `YYYYMMDD-HHMMSS-<4-byte-random-hex>`

```
app_home/projects/myproject/sessions/
  20260622-095500-a1b2c3d4/
    session.json
```

- Timestamp prefix enables human-readable chronological ordering
- Random hex suffix prevents collisions from simultaneous sessions
- `LastClean` and `Last` use directory entry `ModTime` for sorting, not folder
  names — more reliable after manual folder manipulation

## Session Lifecycle

### Create (default startup)

```
Create(workDir)
  ├─ EnsureProjectDir(workDir) → project sessions dir
  ├─ randomName() → "20260622-095500-a1b2c3d4"
  ├─ os.MkdirAll(folder, 0755)
  ├─ Write empty session.json (messages: [], closed_cleanly: false)
  └─ Return *Session
```

### Resume (-c flag)

```
LastClean(workDir)
  ├─ EnsureProjectDir(workDir) → project sessions dir
  ├─ List subdirectories, Load each session.json
  ├─ Filter: closed_cleanly == true
  ├─ Sort by ModTime descending
  ├─ Pick newest → Load(folder)
  └─ Return *Session or ErrNoCleanSession
```

### Resume Last (-r flag)

```
Last(workDir)
  ├─ Same as LastClean but no closed_cleanly filter
  ├─ Picks most recent session regardless of clean status
  └─ Return *Session or ErrNoSessions
```

### Close (/exit)

```
Session.Close()
  ├─ ClosedCleanly = true
  └─ save()
```

### Clear (/clear or /new)

```
Session.Reset()
  ├─ Messages = []
  ├─ ClosedCleanly = false
  └─ save()
```

### Append (per turn)

```
Session.Append(msg) or Session.AppendAll(msgs)
  ├─ Append to Messages slice
  └─ save() → immediate disk write
```

Saves to disk on every mutation. Each message or batch of messages triggers an
atomic write of the full `session.json`.

### AppendReadFileResult (file-aware append)

```
Session.AppendReadFileResult(msg)
  ├─ Append msg to Messages
  ├─ Extract file path from msg.Content envelope
  ├─ Scan all earlier tool messages for read_file results with the same path
  ├─ Clear content of older same-path results (empty string)
  └─ save() → atomic write
```

This deduplicates read_file results: only the most recent read of any given
file retains its full content in the session. Older results for the same path
are cleared to save context space while preserving the tool_call_id reference.

## Saving

```go
func (s *Session) save() error {
    // JSON encoder with:
    //   SetEscapeHTML(false) — preserve <, >, & for readability (XML in injected prompt content)
    //   SetIndent("", "  ")  — human-readable formatting
    // Trailing newline stripped before write
    // Permissions: 0644
}
```

Key decisions:
- `SetEscapeHTML(false)` — default JSON encoder escapes `<`, `>`, `&` to unicode
  sequences, making XML-style prompt content (like `<context>`) unreadable in
  the on-disk JSON. The LLM receives correct data regardless (unescape on read),
  but on-disk readability matters for debugging.
- Full file rewrite on every save. No append-log or diff approach.
- `prompt.json` is an optional debug artifact saved beside `session.json` only when `config.debugPrompt` is true.

## Session Sanitization

Session messages can become invalid when a turn is interrupted mid-tool-call
(e.g., user Ctrl-C during shell execution). The session would have an assistant
message with `tool_calls` but no matching tool results, causing a 400 error on
the next LLM call.

### Sanitize Algorithm

```
SanitizeMessages(messages)
  ├─ Scan forward from index 0
  ├─ For each assistant with tool_calls:
  │    ├─ Collect expected tool_call IDs
  │    ├─ Collect consecutive tool messages after assistant
  │    │    ├─ Drop orphan tool messages (no matching ID)
  │    │    ├─ Drop duplicate tool messages (same ID)
  │    │    └─ Keep valid tool results
  │    ├─ If all expected IDs seen → round complete, continue
  │    └─ If missing tool results → truncate FROM this assistant onward
  │         (drops broken assistant + any subsequent user messages
  │          built on broken history)
  └─ Return sanitized messages
```

Called at the start of every `RunTurn` — before every LLM call. This ensures
resume from interrupted sessions, `/cd`, or any state change always produces
valid message history.

Returns two slices: sanitized messages and removed messages (for debugging).
The removed slice includes orphan tools, truncation tails.

### The Two-Pass Evolution

1. **First version**: backward scan — only checked the tail of the message
   array. Failed when user messages followed an incomplete tool round.

2. **Current version**: forward scan — for every assistant with `tool_calls`,
   validates the following tool messages. If incomplete, truncates from that
   assistant onward. Catches all cases including user messages after broken
   tool calls.

## What Is NOT Persisted in Session

- **Active skills list** — in-memory only, starts empty per session
- **Runtime prompt part** — universal sysprompt, OS prompt, helpers, skills
  section, AGENTS.md — rebuilt from disk every LLM call
- **Summarization state** — no `summarizedIDs` or separate state file. Session
  JSON is the source of truth. Summaries are loaded from `summaries/` folder
  and injected as synthetic messages.

## Token Usage Analytics

Each session maintains a private `token-usage.json` file alongside `session.json`.
This is aggregate provider usage data for personal inspection, NOT used by the
runtime for any decision-making.

### UsageReport

```go
type UsageReport struct {
    SessionFolder string                   `json:"session_folder"`
    UpdatedAt     time.Time                `json:"updated_at"`
    Models        map[string]UsageSnapshot `json:"models"`
}
```

### UsageSnapshot

```go
type UsageSnapshot struct {
    Requests            int       `json:"requests"`
    PromptTokens        int       `json:"prompt_tokens"`
    CompletionTokens    int       `json:"completion_tokens"`
    TotalTokens         int       `json:"total_tokens"`
    CachedTokens        int       `json:"cached_tokens"`
    CacheWriteTokens    int       `json:"cache_write_tokens"`
    ReasoningTokens     int       `json:"reasoning_tokens"`
    UncachedInputTokens int       `json:"uncached_input_tokens"`
    CacheHits           int       `json:"cache_hits"`
    CacheMisses         int       `json:"cache_misses"`
    UnknownCacheStatus  int       `json:"unknown_cache_status"`
    LastUpdated         time.Time `json:"last_updated"`
}
```

### RecordUsage

Called by the runtime after each LLM response with provider usage data.
Aggregates token counters per model. Uses atomic rename for safe concurrent writes.

### CacheKeyForSession

Returns a stable, non-sensitive SHA-256 hash of the session folder path.
Used as the prompt cache identity for provider requests so the provider never
sees local filesystem details.

## Startup Flags

| Flag | Action | Source |
|------|--------|--------|
| default | Create new session | spec |
| `-c` | Resume last cleanly closed session | spec |
| `-r` | Resume most recent session (interrupted or clean) | subsequent addition |

## Compaction Impact

Context compaction (see 11-context-compaction.md) physically prunes old messages
from `session.json`. Pruned messages are removed from the array and optionally
summarized. The session file is rewritten after compaction.

## Test Helpers

Test-friendly variants accept explicit directories to avoid real app home:
- `CreateInDir(dir)` — creates session in specified directory
- `LastCleanInDir(dir)` — finds clean session in specified directory
- `LastInDir(dir)` — finds most recent session in specified directory
