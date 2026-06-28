# Session Decision Summary: compaction-trigger-tool-loop

Date: 2026-06-28 22:29
Base commit: 95219b7bcffb06723f44cc1bcea5337a1ac560b2

## Context
A long tooling session reached 150k tokens without compaction at the 100k threshold. Compaction was only checked when `len(resp.ToolCalls) == 0`, so during multi-turn tool loops it was never reached.

## Changes Made
Added a second compaction check after the tool execution loop, before the next LLM call. The no-tool-calls path (final turn) keeps its existing compaction check.

## Decisions And Rationale
Compaction must fire after every LLM turn regardless of whether tool calls are present. The usage data from the response reflects the prompt size before tool results are appended, which is the correct trigger point for the next iteration.

## Implementation Approach
One block of 6 lines in `internal/runtime/runtime.go` inserted between the tool execution for-loop and the loop continuation comment.

## Files Included
- `internal/runtime/runtime.go`: compaction check after tool execution.
- `decisions/2026-06-28-2229-compaction-trigger-tool-loop.md`

## Commit Linkage
This summary is committed together with the implementation change to keep rationale linked to code history.
