# Task-Focused Summarization

## Goal

Replace fixed pruning and summarization with task-aware summarization that keeps the last active task fully visible.

## Idea

- When context compaction runs, identify the latest task that is still active.
- Summarize only work that is already finished.
- Keep the active task itself uncompressed and directly visible.
- Treat completed tasks as closed history instead of always-on context.

## Why It May Help

- Reduces noise from unrelated completed work.
- Keeps the current task sharper in the prompt.
- Matches the common user mental model of one active task with summarized history behind it.
