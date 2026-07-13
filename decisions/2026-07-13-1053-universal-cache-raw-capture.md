# Session Decision Summary: Universal cache normalization and raw capture

Date: 2026-07-13 10:53

## Context

DeepSeek and Mimo providers reported zero cached tokens in `token-usage.json` despite raw JSON confirming cache hits. Analysis showed each provider uses slightly different field names for the same data. A stable universal raw-response capture was also needed for future debugging without accumulating per-turn files.

## Changes Made

- Added normalized generic usage parser that maps provider-specific field names to shared counters (cache tokens, reasoning tokens, input/output aliases).
- Moved deterministic tool ordering from OpenAI provider to the shared tool registry.
- Added `llm-raw.json` single-file per-response raw capture (overwritten before each new request).
- Added `reasoning_tokens` to provider usage, session aggregation, and `token-usage.json`.
- Added `ResetRawJSON` and `AppendRawJSON` helpers in `internal/session/raw_capture.go`.
- Included the pre-existing `AGENTS.md` KISS emphasis update.

## Decisions And Rationale

- Prefer `prompt_tokens_details.cached_tokens` first, then `input_tokens_details.cached_tokens`, then `prompt_cache_hit_tokens`: covers OpenAI, DeepSeek, and Mimo without field-specific branches.
- Use `llm-raw.json` single-file reset per turn instead of append: keeps only the latest response for debugging without accumulating files.
- Derive `uncached_input_tokens` as `input_tokens - cached_tokens`: all providers report one of the two token counts.
- Move tool sorting from OpenAI to registry root: all providers receive the same deterministic order.
