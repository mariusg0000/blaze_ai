# Cross-Model Delegation (ask_a_friend, analyze_image)

## Source Files

| File | Role |
|------|------|
| `internal/tools/ask_friend.go` | AskFriendTool and AskFriendArgs |
| `internal/tools/analyze_image.go` | AnalyzeImageTool and AnalyzeImageArgs |
| `internal/tools/image_input.go` | Shared local image preprocessing for vision calls |
| `internal/llmcall/llmcall.go` | Caller — role resolution, prompt construction, one-shot LLM call |
| `internal/tools/ask_friend_test.go` | ask_a_friend unit tests |
| `internal/tools/analyze_image_test.go` | analyze_image unit tests |

## ask_a_friend Tool Signature

```go
type AskFriendArgs struct {
    Role         string `json:"role"`               // required — one of: advisor, summarization, vision
    Purpose      string `json:"purpose"`            // required — 3-sentence explanation
    Question     string `json:"question"`           // required — focused ask
    Context      string `json:"context"`            // required — supporting evidence, max 300000 chars
    InputFile    string `json:"input_file,omitempty"` // optional — readable text file path, max 500000 bytes
    OutputFormat string `json:"output_format"`      // required — exact expected answer shape
    Timeout      *int   `json:"timeout,omitempty"`  // optional — seconds, default 60
}
```

## ask_a_friend Description (LLM-Facing)

```
Delegate one focused text-only question to a configured secondary model role with no tools.
Use it only when an independent summarization, review, risk check, or trade-off
analysis would improve the current task. Provide all required context because the
secondary model cannot see the current conversation. For screenshots, photos,
charts, maps, and other visual inputs, use analyze_image instead.
```

## ask_a_friend Parameter Constraints

| Field | Max Length | Notes |
|-------|-----------|-------|
| `purpose` | required | 3-sentence explanation |
| `question` | required | Focused ask |
| `context` | 300000 chars | Supporting evidence |
| `output_format` | required | Exact answer shape |
| `input_file` | 500000 bytes | Readable text file content appended to context |
| Response | no local cap | Provider limits still apply |

## analyze_image Tool Signature

```go
type AnalyzeImageArgs struct {
    InputFile string `json:"input_file"` // required — local image file path
    Question  string `json:"question"`   // required — exact requested image analysis
}
```

## analyze_image Description (LLM-Facing)

```
Analyze one local image file with the configured vision model role.
Use it for screenshots, photos, diagrams, maps, charts, scans, and other visual
inputs. Put the exact task, required details, and desired answer shape directly
in question because the tool only accepts the image path and the question.
```

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

- `ask_a_friend(role="summarization")` uses the summarization model
- `ask_a_friend(role="advisor")` uses the advisor model
- `ask_a_friend(role="vision")` is allowed but still text-only
- `analyze_image` always forces `role="vision"`
- Missing role configuration returns an error with no fallback

If the summarization role is configured with a different model than default, a
separate provider client is created at agent construction time. This client is
used by both `ask_a_friend` calls with `role="summarization"` and by the
context compaction system for generating summaries.

## ask_a_friend Execution Flow

```text
AskFriendTool.Execute(ctx, args)
  -> Check ctx.Err() => "aborted before execution by user"
  -> Parse args => AskFriendArgs
  -> validateAskFriendArgs:
       - role in {advisor, summarization, vision}
       - purpose non-empty
       - question non-empty
       - context non-empty, max 300000 chars
       - output_format non-empty
  -> prepareAskFriendArgs:
       - If input_file set:
         - os.Stat => regular file check
         - Detect real MIME type from file header
         - Reject image/* inputs => "use analyze_image"
         - Size check <= 500000 bytes
         - os.ReadFile
         - Append to context as [INPUT FILE] section
  -> Create timeout context (default 60s)
  -> caller.Call(callCtx, prepared)
```

## ask_a_friend Input File Handling

The `input_file` parameter lets the LLM include a text file's content directly in the
consultation context without requiring a separate shell tool call:

1. File must be a regular file
2. MIME type is detected from file content, not only the extension
3. Image files are rejected with a clear error directing the caller to `analyze_image`
4. Non-image files must not exceed `500000` bytes
5. Content is appended to `context` as:

```text
[INPUT FILE]
path: <filepath>
size_bytes: <size>
content:
<file content>
```

## analyze_image Execution Flow

```text
AnalyzeImageTool.Execute(ctx, args)
  -> Check ctx.Err() => "aborted before execution by user"
  -> Parse args => AnalyzeImageArgs
  -> Validate input_file and question non-empty
  -> Prepare image:
       - os.Stat => regular file check
       - Detect real MIME type from file header
       - Reject non-image files
       - Reject unsupported image formats
       - image.Decode
       - Resize proportionally to max 1200px on the longest side
       - Flatten transparency to white
       - JPEG encode the processed image
       - Build data:image/jpeg;base64,... payload
  -> Create timeout context (default 60s)
  -> caller.Call(callCtx, prepared)
       - Force role = vision
       - Build text metadata/context from file path and dimensions
       - Attach the JPEG data URL as multimodal image_url
```

## analyze_image Image Handling

`analyze_image` converts supported local image files into one compact multimodal payload:

1. Supported source formats: `image/png`, `image/jpeg`, `image/gif`
2. Longest side is capped at `1200px`
3. Output is always JPEG for compact transport
4. Final provider payload uses OpenAI-compatible multimodal content:

```json
[
  {
    "type": "text",
    "text": "Purpose... Question... Context... Required output format..."
  },
  {
    "type": "image_url",
    "image_url": {
      "url": "data:image/jpeg;base64,..."
    }
  }
]
```

5. No OCR fallback, shell compression fallback, or text-file fallback is used

## Secondary Model Prompt

The secondary model receives a minimal no-tools system prompt:

```text
You are a focused expert consultant.
Return only the requested answer.
Do not call tools.
Do not invent hidden steps.
If the supplied context is insufficient, say so briefly and explain exactly what is missing.
```

The user message contains either plain text or a multimodal text+image array.
The secondary model has no access to the main conversation history beyond the
purpose, question, context, output format, and optional image payload supplied in the request.

## Return Value

- Success: full assistant response text
- Timeout: `"timeout <N>s exceeded"`
- Input file error: `"error: <description>"`
- Role not configured: `"error: <role> role is not configured"`
- Image misuse in `ask_a_friend`: `"error: input_file is an image; use analyze_image: <path>"`

## Usage Guidance (in Prompt)

The sysprompt [SECONDARY MODEL CONSULTATION] section advises:

> Use `ask_a_friend` only for focused text-only secondary-model help: `summarization` for
> summarizing, extracting, or compacting supplied content, and `advisor` for
> stronger-model review of design, risks, root causes, or trade-offs. The
> secondary model has no tools and no access to the current conversation, so
> include every required snippet, log, file excerpt, goal, constraint, and
> expected output format in `context`, or provide one readable text `input_file`
> up to 500000 bytes when direct file content is the right input. Use
> `analyze_image` for screenshots, photos, charts, maps, diagrams, scans, and
> other image files. Do not delegate routine work that the main model can handle directly.

## FormatArgs

### ask_a_friend

| Role | Purpose | Display |
|------|---------|---------|
| present | present | `Consulting <role>: <purpose>` |
| present | empty | `Consulting <role>` (truncated to 50 chars) |
| empty | present | `Consulting: <purpose>` |
| empty | empty | `Consulting secondary model` |

### analyze_image

- Valid args: `Analyzing image: <name> — <question excerpt>`
- Invalid args: `Analyzing image`
