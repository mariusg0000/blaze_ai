# Tools

## Source Files

| File | Contents |
|------|----------|
| `internal/tools/tools.go` | Tool interface, Registry, OpenAITool format, ParseToolCallArgs, truncateDisplay |
| `internal/tools/analyze_image.go` | analyze_image implementation |
| `internal/tools/task_tools.go` | task_read, task_write implementations |
| `internal/tools/tools_test.go` | FormatArgs tests, schema validation |

Individual tool files have their own specs (06–08). This file covers the shared
tool system: interface, registry, OpenAI format, conventions, and the two simple
task tools.

## Tool Interface

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() json.RawMessage  // JSON Schema
    Execute(ctx context.Context, args json.RawMessage) string
    FormatArgs(args json.RawMessage) string
}
```

- `Name()` — unique tool identifier used by the LLM to reference the tool
- `Description()` — human-readable description for the LLM (injected into the tools array)
- `Parameters()` — JSON Schema object defining the tool's arguments
- `Execute()` — runs the tool with parsed JSON arguments, returns result as string
  - Receives `context.Context` for cancellation (user abort, timeout)
  - Must check `ctx.Err()` before starting work
- `FormatArgs()` — returns a human-readable summary of the tool call for console display
  - Purpose text from LLM is NEVER truncated
  - Fallback (command/path/name when purpose missing) truncated to 50 chars

## Registry

```go
type Registry struct {
    tools map[string]Tool
}
```

- `Register(tool)` — panics on duplicate name (tools are hardcoded, never dynamic)
- `Get(name)` — returns Tool or nil
- `All()` — returns all registered tools
- `FormatArgs(name, args)` — delegates to tool's FormatArgs, falls back to raw JSON string

All 9 tools are registered at agent construction in `runtime.NewAgent()`:

```go
registry.Register(NewShellTool(os))
registry.Register(NewLoadSkillTool(active, skillResolver))
registry.Register(NewUnloadSkillTool(active, skillResolver))
registry.Register(NewRunSkillTool(os, runnableSkillResolver, workDirGetter))
registry.Register(NewAnalyzeImageTool(oneShotCaller))
registry.Register(NewAskFriendTool(oneShotCaller))
registry.Register(NewReplaceBlockTool(workDirGetter))
registry.Register(NewTaskWriteTool(workDirGetter))
registry.Register(NewTaskReadTool(workDirGetter))
```

## OpenAI Tool Calling Format

Tools are sent to the LLM as OpenAI-compatible `functions` in the chat completion
request. The runtime converts all registered tools via `AllToOpenAI()`:

```go
type OpenAITool struct {
    Type     string      `json:"type"`     // always "function"
    Function FunctionDef `json:"function"` // name, description, parameters
}
```

The LLM responds with `tool_calls[]` in the assistant message:

```go
type ToolCall struct {
    ID        string
    Name      string
    Arguments json.RawMessage
}
```

Converted to OpenAI's wire format via `ToOpenAIToolCall()` when sent back to the
API in subsequent turns.

### Tool Call Lifecycle in RunTurn

1. LLM returns `tool_calls[]` in assistant message
2. Runtime extracts each `ToolCall` (ID, name, arguments)
3. Runtime calls `Handler.OnToolCall(name, purpose)` for transport display
4. Runtime calls `registry.Get(name).Execute(ctx, args)` with optional timeout
5. Runtime calls `Handler.OnToolResult(name, result)` for transport display
6. Result appended to session as `tool`-role message (with `tool_call_id`)
7. `RunTurn` loops: feeds tool results back to LLM, LLM may produce more tool calls or final content

### Multi-Tool-Call Per Turn

The LLM may return multiple `tool_calls` in a single assistant message. They are
processed sequentially in order. Each tool result is appended to the session
before the next tool call executes.

### Default Timeout

`DefaultTimeout = 60` seconds. Passed to `Execute()` via `context.WithTimeout`.
On timeout, the tool should detect `ctx.Err()` and return `"timeout <N>s exceeded"`.

### Tool Call Arguments

Arguments are raw JSON. Each tool defines its own argument struct and processes
it via `ParseToolCallArgs[T]()`:

```go
parsed, err := ParseToolCallArgs[MyToolArgs](args)
if err != nil { return fmt.Sprintf("error: invalid arguments: %v", err) }
```

## Common Conventions

All tools share these conventions:

1. **Check context cancellation first** — `if ctx != nil && ctx.Err() != nil { return "aborted before execution by user" }`
2. **Error prefix** — errors returned by `Execute()` start with `"error: "`
3. **Success prefix** — successful results are plain strings (no prefix convention beyond "ok")
4. **Purpose field** — most tools include a `purpose: string` parameter for the LLM to describe why the tool is needed (3 sentences). Displayed in console, NEVER truncated
5. **Abort on user interrupt** — `ErrTurnAborted` is returned by the runtime when the user cancels; tool results are still appended to session

## Display Format (Tool Emoji)

Each tool has a dedicated emoji for console and Telegram display:

| Tool | Emoji | Display |
|------|-------|---------|
| `shell` | `💻` | `💻 purpose …` |
| `load_skill` | `📥` | `📥 purpose …` |
| `unload_skill` | `📤` | `📤 purpose …` |
| `run_skill` | `🚀` | `🚀 purpose …` |
| `replace_block` | `📝` | `📝 purpose …` |
| `ask_a_friend` | `🧠` | `🧠 purpose …` |
| `analyze_image` | `🖼` | `🖼 image analysis …` |
| `task_read` | `📋` | `📋 purpose …` |
| `task_write` | `📋` | `📋 purpose …` |
| Unknown | `🔧` | `🔧 name …` (generic fallback) |

Mappings are defined in both `internal/console/console.go` and
`internal/telegram/handler.go` — identical so the same tool feels the same
across transports.

## Image Analysis Tool

### analyze_image

- **Description**: `vision role → analyze one local image file`
- **Parameters**: `{ input_file: string, question: string }`
- **FormatArgs**: `"Analyzing image: <name> — <question excerpt>"`
- **Execute**:
  - Requires `input_file` and `question`
  - Detects the real image format from file headers
  - Accepts supported image inputs (`png`, `jpeg`, `gif`)
  - Resizes to max `1200px` on the longest side
  - Encodes the processed image as JPEG and sends it as a multimodal `image_url` data URL to the configured `vision` role
  - Unsupported format, decode failure, missing role, or provider rejection return clear errors with no fallback

## Task Tools

### task_read

- **Description**: `tasks.md → read current task list`
- **Parameters**: `{}` (empty — no arguments)
- **FormatArgs**: `"Loading tasks"` (static, no args to display)
- **Execute**: Reads `tasks.md` from current work directory
  - File not found → returns `"ok no tasks"`
  - Empty file → returns `"ok no tasks"`
  - Success → returns `"ok\n" + file content`

### task_write

- **Description**: `tasks.md → overwrite with full task list`
- **Parameters**: `{ tasks: string (required) }`
- **FormatArgs**: `"Saving tasks"` (static, never truncated)
- **Execute**: Writes `tasks` content to `tasks.md` in current work directory (overwrite)
  - Empty tasks string → returns `"error: tasks is required"`
  - Success → returns `"ok"`
  - Write failure → returns `"error: cannot write tasks: ..."`

### Task File Location

Both tools write/read `{workDir}/tasks.md`. The workDir is resolved at execution
time from a closure, not captured at construction — so `/cd` changes are reflected
immediately.

## FormatArgs and Purpose Display

All tools that receive a `purpose` parameter from the LLM expose it in `FormatArgs`:

```
// Example: shell with purpose
FormatArgs → "Search for config files…"
```

Rules:
- If `purpose` is present in the parsed args, it is returned as-is (never truncated)
- If `purpose` is absent or empty, a fallback string is generated from command/path/name
- Fallback strings are truncated to 50 characters with `"..."` suffix
- `FormatArgs` for tools without purpose (task_read, task_write) return a static label
