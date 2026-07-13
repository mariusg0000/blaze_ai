# Session Decision Summary: Centralized usage module

Date: 2026-07-13 11:17

## Context

Token usage extraction was spread across `provider.go`, `openai_responses.go`, and `runtime.go`. Each provider had its own normalization logic for cache, reasoning, and token fields. The user requested a single boundary where raw JSON enters and standardized usage exits.

## Changes Made

- Created `internal/usage/usage.go` with one `Extract(raw []byte) (*Usage, bool)` function.
- Moved all extraction and normalization logic into the centralized module.
- Both generic SSE and ChatGPT Responses SSE now call `usagepkg.Extract()`.
- Removed `genericUsage`, `normalizeGenericUsage`, and `chatGPTUsage` local types.
- `provider.Usage` became a type alias for `usage.Usage`.
- `UsageData` struct in `session/usage.go` now includes `UncachedInputTokens`.
- Runtime handler `OnUsage` signature extended with `cachedTokens` and `uncachedTokens`.
- Console separator displays `CTX`, `CH` (cache hit), `CM` (cache miss).
- Pre-existing `AGENTS.md` and console-test changes included.

## Decisions And Rationale

- Accept raw JSON and decode once in `usage.Extract()`: eliminates field mapping duplication and handles both `usage` and `response.usage` paths naturally.
- `UncachedInputTokens` is extracted from provider `prompt_cache_miss_tokens` when available, otherwise derived from `input - cached`: prevents misleading CM values when prompt tokens include non-input components.
- The `Usage` type alias maintains source compatibility for existing callers without import changes.
- All three providers (OpenAI, DeepSeek, Mimo) now produce identical normalized output through one code path.
