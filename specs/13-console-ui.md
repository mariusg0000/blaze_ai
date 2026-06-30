# Console UI

## Source Files

| File | Role |
|------|------|
| `internal/console/console.go` | Console struct, Handler implementation, rendering, REPL loop, slash commands |
| `internal/console/reader.go` | Input reading — ReadEvent, ReadHiddenInput, line editor |
| `internal/console/console_test.go` | Unit tests |

## Overview

The console is a simple CLI REPL with Markdown rendering, colored labels, visual
separators, and streaming output. It implements the Handler contract as the
primary transport. TTY-only — always applies ANSI codes, no non-TTY mode.

Prompt-specific console guidance lives in `prompts/transport.console.md` and is
loaded through `Builder.TransportName = "console"`.

## Console Struct

```go
type Console struct {
    // Handler methods carry state between calls in one turn:
    contentStarted  bool
    reasoningStarted bool
    atLineStart     bool   // tracks whether cursor is at line start for newline insertion
    contentBuffer   string // buffered streaming content pending newline
    lastPromptTokens int   // from OnUsage, for CTX display

    // UI elements
    writer      io.Writer
    reader      *Reader
    promptLabel string   // e.g. "[USER/(provider/model)]"

    // Callbacks
    onCommand   func(cmd string, args []string) bool
    onAbort     func()
    onUserInput func(text string)

    // State
    running bool
}
```

The struct carries state between handler callbacks within a single turn. State is
reset at the start of each turn.

## Color Constants

| Constant | Code | Usage |
|----------|------|-------|
| `colorBrightBlue` | `\033[0;94m` | Bracket/box borders, border chars in response separator |
| `colorBrightGreen` | `\033[0;92m` | Success badges, tool group labels |
| `colorOrange` | `\033[0;33m` | [BLAZE] label, model name in separator |
| `colorCtx` | `\033[0;96m` | CTX token count (bright cyan) |
| `colorPurple` | `\033[0;95m` | User input prefix, status labels |
| `colorRed` | `\033[0;91m` | Error messages |
| `colorYellow` | `\033[0;93m` | System messages |
| `colorDim` | `\033[2m` | Muted/less important text |
| `colorOrangeGreen` | `\033[38;5;208m` | Reasoning header |
| `colorOlive` | `\033[0;93m` | Fallback/highlight |
| `colorOff` | `\033[0m` | Reset |

## Turn Sequence (Visual Output)

```
┌─────────────────────────────────────────────────────────┐
│ [USER/(provider/model)] > user input here               │
└─────────────────────────────────────────────────────────┘
🧠 The user is asking about...                             ← reasoning streaming
tools ─────────────────────────────────────────────
💻 Search files... ✔️ CTX: 45K                           ← tool call + result inline
💻 Process results... ✔️ CTX: 46K
───────────────────────────────────────────────────
[BLAZE] Here is the result...                             ← content streaming
│ CTX: 47K o3 ────────────────────────── /home/user/prj  ← response separator
```

### Startup Splash

On session start:

```
 BlazeAI                                    ← orange bold centered
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  ← blue box border
────────────────────────────────────────     ← visual break
```

### Reasoning Streaming

- First chunk: `\n` if `!atLineStart`, then `🧠 ` in orange-green bold, set `reasoningStarted=true`
- Subsequent chunks: append text directly after previous reasoning (no new prefix)
- State reset at end of turn

### Content Streaming

- First chunk: `\n` if `!atLineStart`, then `[BLAZE] ` in orange bold, set `contentStarted=true`
- Subsequent chunks: buffer in `contentBuffer` until newline, then `renderLine()`
- Lines are rendered through the Markdown renderer
- State reset at end of turn

### Tool Call

- `OnToolCall(name, formattedArgs)`:
  - `\n` if `!atLineStart`
  - Emoji from tool emoji map + purpose text + ` … ` (dim ellipsis)
  - No newline at end (tool result appends inline)

### Tool Result

- `OnToolResult(name, result)`:
  - Append status from result text:
    - Result contains `"exit_code: 0"` or `"ok"` → ` ✔️` (green)
    - Result contains `"error:"` → ` ✖️` (red)
    - Result contains `"timeout"` → ` ⏱` (yellow)
  - If `lastPromptTokens > 0` → ` CTX: <N>K` (bright cyan)
  - Final `\n` to close the tool line

### Response Separator

At end of each turn after tool results:

```
│ CTX: 47K o3 ───────────────────────── /home/user/prj
```

- Left border `│` in bright blue
- `CTX: 47K` in bright cyan (only if `lastPromptTokens > 0`)
- Model name in orange bold
- Dim line `─` fills remaining width
- Work directory in orange bold (if different from home)

Slash commands and startup splash do not produce a separator.

### User Prompt

```
[USER/(provider/model)] > _
```

- `[USER/` prefix in purple
- Provider/model name in orange bold
- `] > ` suffix in purple
- Cursor at underscore position

## Markdown Rendering

Line-by-line rendering during content streaming. Lines are buffered if they
contain inline markers that may span multiple chunks.

### Rendered Elements

| Markdown | Terminal Output |
|----------|-----------------|
| `# Heading` | Bright cyan bold, larger visual weight |
| `## Subheading` | Bright blue bold |
| `### Sub-sub` | Bright green bold |
| `- item` / `* item` | `• ` prefix, bullet-colored |
| `1. item` | `1. ` numbered |
| `` `code` `` | Inline dim/highlighted |
| `**bold**` | ANSI bold |
| `*italic*` / `_italic_` | ANSI dim (italic unsupported, dim as fallback) |
| `[text](url)` | `text` + purple `(url)` |
| ` ``` ` fence | Dim gray lines, no interior rendering |
| Tables | Pipes stripped, separator lines skipped, cells flattened |

### Table Rendering

Tables from the LLM are handled defensively:
1. Lines matching `|---` are skipped (separator rows)
2. Lines containing `|` have pipes stripped, cells joined with spaces
3. Prompt instructions tell the LLM to avoid tables
4. No attempt at column alignment

### Buffering Rules

A line is buffered (held until newline arrives) when it contains:
- Unmatched `**` (bold delimiter)
- Unmatched `` ` `` (code delimiter)
- Unmatched `*` or `_` (italic delimiter)
- Table pipe `|`
- Opening link syntax `[text](`

Once the closing delimiter arrives in a subsequent chunk, the full line is flushed.

## Tool Emoji Map

```go
func toolEmoji(name string) string {
    switch name {
    case "shell":        return "💻"
    case "load_skill":   return "📥"
    case "unload_skill": return "📤"
    case "run_skill":    return "🚀"
    case "replace_block": return "📝"
    case "ask_a_friend": return "🧠"
    case "task_read",
         "task_write":   return "📋"
    default:             return "🔧"
    }
}
```

## Slash Commands

| Command | Action |
|---------|--------|
| `/exit` | Clean close, mark `closed_cleanly=true`, save, exit(0) |
| `/model` | Without arg: list favorite models with numbers. With arg: set model |
| `/cd` | Change work folder. Invalid path → clear error, keep current |
| `/clear` | Reset current session messages |
| `/new` | Reset + start fresh (same as clear) |
| `/show-reasoning` | Toggle reasoning display on/off |
| `:<mode-name>` | Switch to work mode (hotkey syntax) |
| Unknown | Passed to LLM as normal user message |

## Input Reading (reader.go)

Reads input from the terminal. Provides:

- `ReadEvent()` — reads one full line of input, returns the text (or empty on EOF)
- `ReadHiddenInput(prompt)` — reads a line with echo disabled (for sudo passwords, API keys)
- Terminal raw mode via `MakeRaw`/`Restore` for Ctrl-C handling

No multiline paste support (removed in TTY-only simplification).
Non-TTY paths return errors (console is TTY-only).

## User Abort (Ctrl-C)

When Ctrl-C is detected during an LLM turn:
1. The agent's context is cancelled
2. `abortCurrentTurn()` is called — prints `✖️ aborted` in red, resets turn state
3. Returns to the REPL prompt for new input
4. Partial output from the aborted turn remains visible

## State Transitions

```
resetTurnState()
  ├─ contentStarted = false
  ├─ reasoningStarted = false
  ├─ atLineStart = true
  └─ contentBuffer = ""
```

Called at the start of each `runAgentTurn()`. Ensures a clean slate for each
LLM interaction regardless of previous state.
