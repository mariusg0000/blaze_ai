# Session Decision Summary: Minimal Markdown Renderer

Date: 2026-06-22 18:40
Base commit: 91db12c

## Context
LLM output contained raw Markdown markers (tables, **bold**) that rendered poorly in the terminal. The console had no Markdown rendering.

## Changes Made
- Added line-based Markdown renderer in console: headings (`#`/`##`/`###`), bullets (`-`/`*`), numbered lists, code fences, inline `**bold**`, inline `` `code` ``
- Table lines are detected and flattened to plain text (pipes removed, separator lines skipped)
- Lines with inline markers (`**`, `` ` ``, `|`) are buffered until newline to avoid partial rendering
- `flushPendingContent()` ensures buffered content is rendered before tool blocks and turn end
- Universal prompt updated: NEVER use tables, prefer lists, keep layouts simple

## Decisions And Rationale
- Line-by-line buffering instead of full response buffering: preserves streaming feel while ensuring complete lines render correctly
- Table handling is defensive (strip pipes, skip separators) rather than relying on the LLM to obey prompt rules
- `shouldBufferMarkdownLine` extended to also match `**`, `` ` ``, and `|` so partial lines with these markers are not flushed before the closing marker arrives

## Implementation Approach
- `renderLine()` dispatches by line type (heading, bullet, numbered, code fence, table, plain)
- `renderInline()` processes `**` and `` ` `` pairs with `toggleDelimited()`
- `isTableSeparator()` / `isTableRow()` / `splitTableRow()` handle Markdown tables defensively
- Prompt rules hardened with imperative wording

## Files Included
- internal/console/console.go: renderLine, renderInline, toggleDelimited, table helpers, shouldBufferMarkdownLine extended
- internal/console/console_test.go: 5 new tests (bold, bold-split, table, table-separator, table-cells)
- prompts/sysprompt.md: harder rules against tables, simpler format preference
