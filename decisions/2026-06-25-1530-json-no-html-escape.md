# Session Decision Summary: json-no-html-escape

Date: 2026-06-25 15:30
Base commit: 09aa382

## Context
After switching injected prompt content to XML, the on-disk JSON files (session.json, prompt.json) showed `<` and `>` as `\u003c` / `\u003e` Unicode escapes. This made debugging XML structure in the prompt unreadable. The LLM receives correct data (Unmarshal decodes escapes), but on-disk clarity matters for inspection.

## Changes Made
- `internal/session/session.go`: `save()` switched from `json.MarshalIndent` to `json.Encoder` with `SetEscapeHTML(false)`. This writes raw `<`, `>`, `&` to session.json instead of Unicode escapes.
- `internal/runtime/runtime.go`: `prompt.json` debug dump switched from `json.MarshalIndent` to `json.Encoder` with `SetEscapeHTML(false)`. Added `"bytes"` import.
- `internal/compaction/compaction.go`: Added `serializeToolCalls()` helper using `json.Encoder` + `SetEscapeHTML(false)`. Replaced two `json.Marshal(msg.ToolCalls)` call sites: token estimation (line 80) and summary construction (line 321). The summary prompt is sent to the summarization LLM, so escaped `<` / `>` would degrade summary quality. Added `"bytes"` import.

## Decisions And Rationale
- `SetEscapeHTML(false)` everywhere JSON is written for on-disk or LLM consumption. The exception is `provider.go` which sends JSON over HTTP to the LLM API — there HTML escaping is the correct security default and is left intact.
- Helper function in compaction: extracted to avoid duplicate encoder setup code and ensure consistency.

## Files Included
- `internal/session/session.go`: Encoder-based save
- `internal/runtime/runtime.go`: Encoder-based prompt.json
- `internal/compaction/compaction.go`: serializeToolCalls helper + two call sites
- `decisions/2026-06-25-1530-json-no-html-escape.md`: This summary

## Commit Linkage
This summary is committed together with the implementation changes.
