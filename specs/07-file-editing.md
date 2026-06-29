# File Editing (replace_block)

## Source Files

| File | Role |
|------|------|
| `internal/tools/replace_block.go` | ReplaceBlockTool implementation, args, FormatArgs, Execute |
| `internal/tools/replace_block_test.go` | Unit tests |

## Tool Signature

```go
type ReplaceBlockArgs struct {
    FilePath string `json:"file_path"`  // required — absolute or relative path
    OldBlock string `json:"old_block"`  // required — exact text to find
    NewBlock string `json:"new_block"`  // required — replacement text (empty = delete)
    Purpose  string `json:"purpose,omitempty"` // optional — 3-sentence explanation
}
```

All four string fields are required by the JSON schema (`required: ["purpose", "file_path", "old_block", "new_block"]`). The tool validates `file_path` and `old_block` are non-empty in `Execute()`.

## Description (LLM-Facing)

```
"file_path + old_block + new_block → replace first exact match; old_block = exact text incl whitespace + newlines"
```

## Parameters Schema

```json
{
  "type": "object",
  "properties": {
    "purpose": {
      "type": "string",
      "description": "purpose = exactly 3 user-visible sentences. Sentence 1 must name the target file and the specific code/text area being edited. Sentence 2 must explain why the edit is needed. Sentence 3 must explain what the replacement will change and how it solves or advances the task."
    },
    "file_path": {
      "type": "string",
      "description": "file_path = target file path"
    },
    "old_block": {
      "type": "string",
      "description": "old_block = exact existing text; match = exact incl whitespace + newlines"
    },
    "new_block": {
      "type": "string",
      "description": "new_block = replacement text; empty string → delete old_block"
    }
  },
  "required": ["purpose", "file_path", "old_block", "new_block"]
}
```

## Execution Logic

```
Execute(ctx, args)
  ├─ Check ctx.Err() → "aborted before execution by user"
  ├─ Parse args → ReplaceBlockArgs
  ├─ Validate file_path non-empty → "error: file_path is required"
  ├─ Validate old_block non-empty → "error: old_block is required"
  ├─ os.ReadFile(file_path)
  │    └─ Error → "error: cannot read file <path>: <err>"
  ├─ strings.Contains(content, old_block)?
  │    └─ No → "error: old_block not found in <path>"
  ├─ strings.Replace(content, old_block, new_block, 1)
  ├─ os.WriteFile(file_path, newContent, 0644)
  │    └─ Error → "error: cannot write file <path>: <err>"
  └─ Return "ok block replaced in <path>"
```

Key properties:
- **First match only** — `Replace(content, old, new, 1)` replaces the first occurrence
- **Exact match** — whitespace and newlines must match character-for-character
- **Empty new_block** — deletes old_block from the file (used for removals)
- **No backup** — the tool mutates the file directly. LLM is responsible for creating backups via shell when needed
- **Path resolution** — the `workDir` closure is used only for display (relative path in console). Execute reads the file path as-is from the `file_path` parameter

## FormatArgs

Three display modes determined by parsed arguments:

| FilePath | Purpose | Display |
|----------|---------|---------|
| present | present | `Editing: <rel-path> — <purpose>` |
| present | empty | `Editing: <rel-path>` (truncated to 50 chars) |
| empty | present | `Editing: <purpose>` |
| empty | empty | `Editing file` (fallback) |

The `displayPath()` helper converts absolute paths to working-directory-relative
paths using `filepath.Rel()`. If the path is not under the work directory, the
absolute path is shown. Purpose is never truncated.

## Differences From Shell

| Aspect | replace_block | shell |
|--------|---------------|-------|
| Primary use | Precise text edits in existing files | General command execution |
| Path handling | `file_path` arg, tool reads/writes directly | `cd` within command text |
| Output cap | File-size limited (no artificial cap) | 150kB combined cap |
| Idempotency | Fails if old_block not found | Depends on command |
| Backup | LLM responsibility | LLM responsibility |

## Error Cases

| Condition | Result |
|-----------|--------|
| `file_path` empty | `"error: file_path is required"` |
| `old_block` empty | `"error: old_block is required"` |
| File not found | `"error: cannot read file <path>: <err>"` |
| old_block not in file | `"error: old_block not found in <path>"` |
| Write permission denied | `"error: cannot write file <path>: <err>"` |
| Context cancelled | `"aborted before execution by user"` |
