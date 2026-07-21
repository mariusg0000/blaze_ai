# Decision Summary: Remove child-agent answer truncation

Date: 2026-07-21 12:00

## Context

Child-agent answers were truncated to a fixed 12,000-rune limit (`maxChildAnswerRunes`) before insertion into the parent context. The user requested removing this arbitrary truncation so that complete child-agent responses are preserved. The `boundAnswer` helper and its constant were the only truncation mechanism.

## Changes Made

- `internal/runtime/agent_orchestration.go`: Removed `maxChildAnswerRunes` constant, removed `boundAnswer` function, replaced both call sites with `strings.TrimSpace(answer)` to preserve whitespace-only trimming without rune-level truncation.
- `internal/runtime/agent_orchestration_test.go`: Added `TestAgentDoneCompletionPreservesCompleteTrimmedAnswer` exercising the agent_done callback path with a body exceeding the former limit. Updated `TestLastAssistantAnswerUsesFinalPlainText` to use a 15,000-rune body verifying the fallback path.
- `specs/20-agent-orchestration.md`: Replaced "Answer Bounding" section with "Child Answers" documenting trim-and-return-complete behavior; updated pseudocode diagram.
- `task.md`: Restored to no-active-task placeholder after completed implementation.

## Decisions And Rationale

- Retained `strings.TrimSpace` on both paths: surrounding whitespace removal is still desirable; only arbitrary rune truncation was harmful.
- No replacement limit was introduced: the project's no-fallback directive favors explicit failures over silent truncation, and the parent context window is managed by compaction.
- Single commit for all four files: all changes are part of one atomic topic with no independent sub-tasks.
