# Cross-Model Delegation (ask_a_friend)

## Source Files

| File | Role |
|------|------|
| `internal/tools/ask_friend.go` | AskFriendTool and AskFriendArgs |
| `internal/llmcall/llmcall.go` | Caller — role resolution, prompt construction, one-shot LLM call |
| `internal/tools/ask_friend_test.go` | Unit tests |

## Tool Signature

```go
type AskFriendArgs struct {
    Role         string `json:"role"`          // required — one of: advisor, summarization, vision
    Purpose      string `json:"purpose"`       // required — 3-sentence explanation, max 500 chars
    Question     string `json:"question"`      // required — focused ask, max 4000 chars
    Context      string `json:"context"`       // required — supporting evidence, max 24000 chars
    InputFile    string `json:"input_file,omitempty"` // optional — file path, max 150000 bytes
    OutputFormat string `json:"output_format"` // required — exact expected answer shape, max 1000 chars
    Timeout      *int   `json:"timeout,omitempty"`   // optional — seconds, default 60
}
```

## Description (LLM-Facing)

```
Delegate one focused question to a configured secondary model role with no tools.
Use it only when an independent summarization, review, risk check, or trade-off
analysis would improve the current task. Provide all required context because the
secondary model cannot see the current conversation.
```

## Parameter Constraints

| Field | Max Length | Notes |
|-------|-----------|-------|
| `purpose` | 500 chars | 3-sentence explanation |
| `question` | 4000 chars | Focused ask |
| `context` | 24000 chars | Supporting evidence (can expand via input_file) |
| `output_format` | 1000 chars | Exact answer shape |
| `input_file` | 150000 bytes | File content appended to context |
| Response | 12000 chars | Hard cap on returned answer |

## Role Resolution

Roles are mapped to models in `config.json`:

```json
{
  "roles": {
    "advisor": "openai/o3",
    "summarization": "openai/gpt-4o-mini",
    "vision": "openai/gpt-4o"
  }
}
```

The `llmcall.Caller` resolves the role name to a model ID via `cfg.ModelForRole(role)`:

- If `role` is `"summarization"` and configured → uses the summarization model
- If `role` is `"vision"` and configured → uses the vision model
- If `role` is `"advisor"` and configured → uses the advisor model
- If any role is not configured → returns an error (no fallback)

If the summarization role is configured with a different model than default, a
separate provider client is created at agent construction time. This client is
used by both `ask_a_friend` calls with `role="summarization"` and by the
context compaction system for generating summaries.

## Execution Flow

```
AskFriendTool.Execute(ctx, args)
  ├─ Check ctx.Err() → "aborted before execution by user"
  ├─ Parse args → AskFriendArgs
  ├─ validateAskFriendArgs:
  │    ├─ role ∈ {advisor, summarization, vision}
  │    ├─ purpose non-empty, max 500 chars
  │    ├─ question non-empty, max 4000 chars
  │    ├─ context non-empty, max 24000 chars
  │    └─ output_format non-empty, max 1000 chars
  ├─ prepareAskFriendArgs:
  │    └─ If input_file set:
  │         ├─ os.Stat → regular file check
  │         ├─ Size check ≤ 150000 bytes
  │         ├─ os.ReadFile
  │         └─ Append to context as [INPUT FILE] section
  ├─ Create timeout context (default 60s)
  └─ caller.Call(callCtx, prepared):
       ├─ cfg.ModelForRole(role) → modelID
       ├─ provider.NewClient(cfg, modelID) → streamClient
       ├─ Build messages:
       │    ├─ system: "You are a consultant. Answer only the asked question. Strict output_format. No tools."
       │    ├─ user: purpose + question + context + output_format
       └─ streamClient.Stream(messages, no tools, no-op onContent)
            └─ Return full assistant text
```

## Input File Handling

The `input_file` parameter lets the LLM include a file's content directly in the
consultation context without requiring a separate shell tool call:

1. File must be a regular file (not directory, symlink, etc.)
2. File must not exceed 150000 bytes
3. Content is appended to the `context` field as:
   ```
   [INPUT FILE]
   path: <filepath>
   size_bytes: <size>
   content:
   <file content>
   ```
4. The original `input_file` path is preserved in the prepared args but not sent
   to the secondary model (the content is already in context)

## Secondary Model Prompt

The secondary model receives a minimal no-tools prompt:

```
You are a consultant. Answer only the asked question. Strict output_format.
No tools.
```

With the user message containing:
```
Purpose: <purpose>

Question: <question>

Context:
<context>

Output format:
<output_format>
```

The secondary model has no access to the conversation history, no tools, and no
awareness of the main agent's context beyond what's provided in the arguments.

## Return Value

- Success: the full assistant response text (capped at 12000 chars)
- Timeout: `"timeout <N>s exceeded"`
- Input file error: `"error: <description>"`
- Role not configured: `"error: <role> role is not configured"`
- Response too long: `"error: ask_a_friend response exceeded 12000 characters"`

## Usage Guidance (in Prompt)

The sysprompt.md [SECONDARY MODEL CONSULTATION] section advises:

> Use `ask_a_friend` only for focused secondary-model help: `summarization` for
> summarizing, extracting, or compacting supplied content, and `advisor` for
> stronger-model review of design, risks, root causes, or trade-offs. The
> secondary model has no tools and no access to the current conversation, so
> include every required snippet, log, file excerpt, goal, constraint, and
> expected output format in `context`, or provide one `input_file` up to 150000
> bytes when direct file content is the right input. Do not delegate routine
> work that the main model can handle directly.

## FormatArgs

| Role | Purpose | Display |
|------|---------|---------|
| present | present | `Consulting <role>: <purpose>` |
| present | empty | `Consulting <role>` (truncated to 50 chars) |
| empty | present | `Consulting: <purpose>` |
| empty | empty | `Consulting secondary model` (fallback) |
