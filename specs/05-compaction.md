# 05 - Context Compaction

## Overview
- This spec defines the BlazeAI context compaction behavior: when and how old messages are pruned, how pruned segments are summarized, how summaries are stored and injected, and how reasoning blocks are stripped from the final payload.
- Product intent is defined in `01-product-scope.md`.
- Runtime mechanics are defined in `02-core-runtime.md`.
- Interface details are defined in `03-interfaces.md`.
- Platform behavior is defined in `04-platform-ops.md`.

## Purpose
- Long sessions grow beyond the model context window.
- Compaction keeps sessions usable by pruning old messages, summarizing what was pruned, and injecting retained summaries back into context.
- The session JSON on disk is the single source of truth. There is no separate compaction state file.

## Compaction Trigger

### Provider-Reported Tokens
- The compaction trigger is based on provider-reported context tokens from the most recent assistant message usage record.
- The runtime reads `usage.prompt_tokens` from the last assistant response.
- If the provider does not report usage tokens, compaction does not trigger.
- The local token estimator is used only for choosing the prune cut point, never as a trigger.

### Thresholds
- Compaction triggers when provider-reported context tokens reach or exceed `maxContextTokens`.
- After compaction, the retained tail should be approximately `minContextTokens` of estimated tokens.
- If provider-reported context tokens exceed the hard cap (`maxContextTokens` + `maxBackoffOffsetTokens`), the runtime forces a prune to `hardMinTokens` (`minContextTokens` + `maxBackoffOffsetTokens`), even if summarization fails.

### Hard Cap
- The hard cap is `maxContextTokens` + `maxBackoffOffsetTokens`.
- If summarization fails below the hard cap, the prune is skipped and the session continues without compaction.
- If summarization fails above the hard cap, the prune is forced without a new summary. The pruned messages are lost from context with no summary replacement.

## Configuration

### Config Location
- Compaction thresholds live in `config.json` under a `compaction` section.
- These values are pre-filled at first-run setup with defaults.

### Config Fields
- `maxContextTokens`: base context threshold that triggers compaction. Default: `100000`.
- `minContextTokens`: target retained tokens after compaction. Default: `50000`.
- `summaryMaxTokens`: approximate token budget requested from the summarizer. Default: `2000`.
- `maxSummaryFiles`: maximum number of per-session summary chunks retained and injected. Default: `10`.
- `tokenCoefficient`: character-to-token divisor used by the local token estimator. Default: `3.5`.
- `maxBackoffOffsetTokens`: maximum offset applied to thresholds. Default: `25000`.

### Reasoning Strip Config
- `stripReasoning.enable`: whether reasoning parts are stripped from the final payload and summary transcript. Default: `true`.
- `stripReasoning.preserveLast`: number of newest reasoning parts kept globally across the assembled payload. Default: `5`.

## Pruning

### Cut Point Selection
- The runtime walks messages from newest to oldest, summing estimated tokens.
- The cut point is the index where the retained tail reaches `minContextTokens` (or `hardMinTokens` when above the hard cap).
- The local token estimator is used for this calculation.
- Token estimation must account for reasoning stripping: stripped reasoning parts count as 0 tokens.

### Tool Boundary Safety
- The cut point must not split a tool call from its tool result.
- If the last pruned message contains a tool call without a matching tool result in the pruned segment, the runtime extends the cut point to include the tool result.
- This prevents orphaned tool calls or orphaned tool results in the pruned segment or the retained tail.

### Prune Operation
- Messages before the cut point are removed from the session JSON.
- The removal is physical: pruned messages are deleted from the JSON on disk.
- There is no `summarizedIDs` list. Pruned messages no longer exist.

## Summarization

### Summarization Model
- Summarization uses the model assigned to the `default` role in config at initial implementation.
- The `summarization` role may be used in the future, but the initial implementation uses `default`.

### Summarization Flow
- When compaction triggers and the prune produces pruned messages:
  1. Build a transcript from the pruned messages.
  2. Include existing historical summaries as read-only context for the summarizer.
  3. Send the combined transcript to the LLM using the summarization model.
  4. Receive the summary text.
  5. Save the summary as a new chunk file in the session summaries folder.
  6. Trim older summary chunks beyond `maxSummaryFiles`.
  7. Rebuild the session JSON with the summary bundle prepended and the retained tail.

### Summary Prompt
- The summarization prompt is adapted from the auto-compress plugin.
- The prompt instructs the LLM to produce a dense, append-only technical summary of only the pruned span.
- The prompt includes existing historical summaries as read-only context.
- The prompt enforces a token budget derived from `summaryMaxTokens`.

### Transcript Construction
- The transcript is built from pruned messages.
- For each message, text parts are included.
- Reasoning parts are included as `[REASONING]...[/REASONING]` blocks only for the newest N reasoning parts (controlled by `stripReasoning.preserveLast`).
- Tool parts are included as compact tool name, input, and output text.
- System-reminder blocks are stripped from the transcript before summarization.
- Empty messages and empty lines are skipped.

### Summary Storage
- Summaries are stored as separate files in the session folder under `summaries/`.
- File names are zero-padded sequential numbers: `000001.md`, `000002.md`, etc.
- Files are ordered chronologically by sequence number.
- When the number of summary files exceeds `maxSummaryFiles`, the oldest files are deleted.
- Summary files are plain Markdown text.

### Summary Injection
- Retained summary chunks are bundled into a single synthetic summary message.
- The synthetic message is prepended to the session JSON, before the retained conversation tail.
- The synthetic message is marked as synthetic in the message metadata.
- The synthetic message contains all retained summary chunks in chronological order, with context framing that tells the LLM:
  - these are historical segment summaries of messages removed from context
  - older summaries appear first
  - newer summaries override older ones on conflicts
  - retained messages that follow are newer than all summaries

## Session Continuation

### Loading On `-c`
- When a session is continued with `-c`, the runtime loads the session JSON from disk.
- If summary chunks exist in the session summaries folder, they are loaded and the synthetic summary message is rebuilt.
- The conversation tail is loaded as-is from the session JSON.
- No re-summarization occurs on load.

## Reasoning Stripping

### Principle
- Some models emit reasoning parts alongside text parts.
- Some providers return errors if reasoning parts are missing from the payload when the model expects them.
- Therefore, reasoning parts are never removed from the session JSON on disk.
- Reasoning stripping applies only to the payload sent to the LLM and to the summary transcript.

### Session JSON On Disk
- The session JSON retains all reasoning parts exactly as received from the provider.
- No stripping, no modification, no removal.

### Payload Sent To LLM
- When `stripReasoning.enable` is `true`, reasoning parts in the payload are replaced with an empty text part.
- Only the newest N reasoning parts (controlled by `stripReasoning.preserveLast`) are kept.
- The count is global across the entire assembled payload, not per message.
- Reasoning parts are identified by type, not by content.

### Summary Transcript
- When building the transcript for summarization, reasoning parts are included as `[REASONING]...[/REASONING]` blocks.
- Only the newest N reasoning parts from the pruned segment are included.
- Older reasoning parts in the pruned segment are omitted from the transcript.

### Cut Point Estimation
- Token estimation for cut point selection must account for reasoning stripping.
- Reasoning parts that will be stripped in the final payload count as 0 tokens.
- Reasoning parts that will be preserved count as their estimated token cost.

## Token Estimation

### Local Estimator
- The runtime includes a local token estimator used only for cut point selection.
- The estimator is not used as a compaction trigger.
- The estimator uses a configurable `tokenCoefficient` as the character-to-token divisor.
- The estimator counts tokens per message part:
  - text parts: estimated from character length
  - reasoning parts: estimated from character length, or 0 if stripped
  - tool parts: estimated from a compact transcript representation
- The estimator is a best-effort approximation. Provider-reported tokens are always authoritative.

## No Cleanup
- Session folders persist on disk indefinitely.
- There is no automatic cleanup of old sessions or old summary files.
- Session cleanup is a future concern, not part of this phase.

## Edge Cases

### Provider Does Not Report Usage
- If the provider does not return `usage.prompt_tokens`, compaction does not trigger.
- The session continues without compaction until the provider rejects the request for being too large.

### Empty Pruned Segment
- If the cut point is at or before the start of the active messages, there is nothing to prune.
- The runtime skips compaction and returns the current messages.

### Summarization Returns Empty Text
- If the summarizer returns empty or whitespace-only text, the summarization is treated as failed.
- If below the hard cap, the prune is skipped.
- If above the hard cap, the prune is forced without a summary.

### No Existing Summaries
- If no summary chunks exist yet, the historical summaries context section is empty.
- The first summarization produces the first summary chunk.

### Maximum Summary Files Reached
- When the number of summary files exceeds `maxSummaryFiles`, the oldest files are deleted.
- The synthetic summary message is rebuilt from the remaining files at the next prompt build.
