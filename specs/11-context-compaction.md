# Context Compaction

## Source Files

| File | Role |
|------|------|
| `internal/compaction/compaction.go` | Manager — ShouldCompact, findCutPoint, Compact, summarize, buildTranscript, save/load/trim summaries, buildSyntheticMessage, StripReasoningFromPayload, RebuildForResume |
| `internal/compaction/compaction_test.go` | Unit tests (cut point, reasoning strip, summarization integration) |
| `internal/runtime/runtime.go` | Calls Compact after each LLM turn, sanitizes before each call, creates summarization client |

## Overview

Context compaction keeps session token usage within the model's context window.
It is triggered by the provider-reported `usage.prompt_tokens` reaching the
`maxContextTokens` threshold. Old messages are pruned from the session and
optionally summarized.

Session JSON is the source of truth — no separate `summarizedIDs` or state file.

## Configuration

```json
{
  "compaction": {
    "maxContextTokens": 100000,
    "minContextTokens": 50000,
    "summaryMaxTokens": 2000,
    "maxSummaryFiles": 10,
    "tokenCoefficient": 3.5,
    "maxBackoffOffsetTokens": 25000
  },
  "stripReasoning": {
    "enable": true,
    "preserveLast": 5
  }
}
```

### Thresholds

| Field | Default | Meaning |
|-------|---------|---------|
| `maxContextTokens` | 100000 | Trigger point — compact when `prompt_tokens >= this` |
| `minContextTokens` | 50000 | Target — retain approximately this many tokens after pruning |
| `maxBackoffOffsetTokens` | 25000 | Headroom above base — hard cap = `maxContextTokens + this` |
| `summaryMaxTokens` | 2000 | Token budget for summary generation |
| `maxSummaryFiles` | 10 | Maximum summary chunks to retain per session |

### Hard Cap

`hardCap = maxContextTokens + maxBackoffOffsetTokens = 125000`

Above hard cap:
- Force prune even if summarization fails (summary skipped)
- Retained target = `minContextTokens + maxBackoffOffsetTokens = 75000`

## Compaction Flow

```
Compact(session, usage)
  ├─ ShouldCompact(usage)
  │    └─ usage.PromptTokens >= maxContextTokens? → false = skip
  ├─ isAboveHardCap(usage) → choose minTokens target
  ├─ findCutPoint(messages, minTokens):
  │    ├─ Walk messages newest→oldest
  │    ├─ sum estimateTokens() for each
  │    ├─ reasoning-stripped messages count for estimateTokens (0 if to be stripped)
  │    └─ Return index where accumulated >= minTokens
  ├─ Split: pruned = messages[:cutIndex], retained = messages[cutIndex:]
  ├─ session.SanitizeMessages(retained):
  │    ├─ Orphan tool results in retained → move to pruned
  │    └─ cleanRetained + removed (appended to pruned for summary)
  ├─ summarize(sessionFolder, pruned):
  │    ├─ buildTranscript(pruned) → text with [REASONING]/[TOOL_CALLS]/[TOOL_RESULT] markup
  │    ├─ loadSummaries → existing summary context
  │    ├─ buildSummaryPrompt(transcript, existing, maxTokens)
  │    ├─ Stream to summarization model (or default if not configured)
  │    └─ Return summary text
  ├─ If summarization fails:
  │    ├─ Above hard cap → force prune without summary, save
  │    └─ Below hard cap → skip prune, leave session intact
  ├─ saveSummary(sessionFolder, summary) → 000001.md, 000002.md, ...
  ├─ trimSummaries(sessionFolder) → delete oldest beyond maxSummaryFiles
  ├─ buildSyntheticMessage(sessionFolder) → system message with all summaries
  ├─ Prepend synthetic message before retained
  └─ Save session
```

### Trigger Point

Compaction checks `usage.prompt_tokens` from the last LLM response. This is the
correct trigger point because it reflects the prompt size before tool results
are appended for the next iteration. Local token estimates are NOT used for the
trigger decision — only provider-reported tokens.

### Trigger Timing

Compaction fires after EVERY LLM turn — both after final content and after tool
execution loops. The original implementation only checked when `len(ToolCalls) == 0`,
which meant long tooling sessions could pass the 100k threshold without any
compaction.

### Cut Point

The cut point is purely token-based. No tool-boundary logic exists in
`findCutPoint`. Tool validity is enforced AFTER the split by calling
`session.SanitizeMessages` on the retained tail. Any orphan tool results in the
retained segment are moved back into the pruned segment before summarization.

This two-step design (cut → sanitize) means:
1. `findCutPoint` is simple and correct (just token estimation)
2. `SanitizeMessages` is the single source of truth for tool validity
3. No information loss — orphan tool results from the cut region are captured

### Summarization

Summarization uses the configured `summarization` model role when available.
If the role is not configured or uses the same model as default, the default
provider is used.

A dedicated provider client is created at agent construction time when the
summarization role differs from the default model, avoiding coupling compaction
to the active agent model.

#### Summarization Prompt

```
You are a conversation summarizer. Produce a dense, append-only technical summary
of the conversation segment below. Focus on facts, decisions, actions taken, and
their outcomes. Omit pleasantries. Keep the summary under approximately <maxTokens>
tokens.

Existing historical summaries (read-only context):
<previous summary text>

Conversation segment to summarize:
<transcript>
```

The secondary model receives:
- A system prompt describing the summarization task
- Any previously saved summaries as read-only context
- The pruned transcript
- No tools, no conversation history

#### Transcript Format

```
[user] What is the purpose of this function?
[assistant] [REASONING]The user wants to know what the function does...[/REASONING] It processes data.
[assistant] [TOOL_CALLS] {"id":"call_1","name":"shell","arguments":"...")
[tool] [TOOL_RESULT shell] exit_code: 0\nstdout:\n...
```

Reasoning is included as `[REASONING]...[/REASONING]` only for the newest N
reasoning messages from the pruned segment. Older reasoning is excluded.

## Summary Storage

Summaries are stored as numbered `.md` files in `session_folder/summaries/`:

```
session_folder/
  session.json
  summaries/
    000001.md
    000002.md
    ...
```

- File names are zero-padded sequential numbers (`000001.md`, `000002.md`)
- Ordered by number, not timestamp — deterministic chronological order
- Maximum `maxSummaryFiles` (default 10) — oldest files are deleted on each compaction
- No separate state or index file — enumeration of `.md` files IS the state

## Synthetic Message

After compaction, a synthetic `system` message is prepended to the session:

```
These are historical segment summaries of messages removed from context.
Older summaries appear first. Newer summaries override older ones on conflicts.
Retained messages that follow are newer than all summaries.

<all summary chunks concatenated in order>
```

Identified by the prefix `"These are historical segment summaries"`.
On resume (`-c`), `RebuildForResume` removes the old synthetic message and
rebuilds it from the summary files on disk.

## Resume Behavior

When continuing a session with `-c`:

1. `RebuildForResume` is called on the loaded session
2. Existing synthetic message (if any) is removed by prefix detection
3. Summary files are reloaded from `summaries/` folder
4. Fresh synthetic message is prepended
5. If no summary files exist, the session is used as-is

This ensures:
- Summary content matches what's on disk (not stale from saved session.json)
- Missing/incomplete summary files don't cause errors
- Clean sessions without compaction are unchanged

## Reasoning Stripping

### What It Does

Reasoning parts are stored intact in session.json on disk. Before sending
messages to the LLM, `StripReasoningFromPayload` creates a copy of the message
array and clears `Reasoning` on older messages:

```
StripReasoningFromPayload(messages)
  ├─ If not enabled → return messages as-is
  ├─ Count reasoning messages from end (global count)
  ├─ If count <= preserveLast → keep all
  ├─ Walk from newest:
  │    ├─ Keep reasoning for newest N
  │    └─ Clear reasoning (set "") for older
  └─ Return new message array
```

### Impact on Token Estimation

When estimating token counts for cut point selection, messages whose reasoning
will be stripped count as 0 tokens for their reasoning content. The
`buildReasoningStripSet` method pre-computes which messages will be stripped
and passes this information to `estimateTokens`.

### In Summaries

Only the newest N reasoning messages from the pruned segment appear in the
summary transcript (wrapped in `[REASONING]...[/REASONING]`). Older reasoning
is excluded from the transcript entirely.

## Session Sanitization Integration

Compaction calls `session.SanitizeMessages` on the retained tail after the cut.
This handles:

1. **Orphan tool results** in the retained segment (because the assistant with
   `tool_calls` was in the pruned segment) → moved to pruned for summarization
2. **Incomplete tool rounds** from the cut boundary → truncated
3. **Duplicate tool results** → removed

The removed messages from sanitization are appended to the pruned segment,
ensuring they are included in the summary even though they were technically in
the retained side of the cut.

## Debug and Observability

- A `prompt.json` file is saved in the session folder after compaction and
  reasoning stripping, showing the exact payload the LLM receives
- Error messages from summarization failures include the underlying error
- Success/failure of each compaction is visible through the `Compact()` return
  value (bool — true if compaction occurred)
