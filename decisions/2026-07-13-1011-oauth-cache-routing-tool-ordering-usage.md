# Session Decision Summary: OAuth prompt-cache routing and tool ordering

Date: 2026-07-13 10:11

## Context

ChatGPT OAuth Responses sessions reported zero prompt-cache hits despite stable prompt-cache keys. Raw Responses analysis showed that the GPT-5.6 Lite tool list was serialized in different orders between turns, invalidating the exact reusable prefix. The ChatGPT subscription backend also rejected the public API-only `prompt_cache_options` parameter, so the implementation had to remain compatible with the OAuth Codex route.

## Changes Made

- Added a stable SHA-256 session cache identity, limited to OpenAI's 64-character key maximum.
- Sent `prompt_cache_key` on ChatGPT OAuth Responses requests.
- Added Codex-compatible client metadata and session, thread, conversation, window, installation, and experimental Responses headers.
- Parsed and privately aggregated `cached_tokens`, cache status, and `cache_write_tokens` per model in `token-usage.json`.
- Sorted Responses tools lexicographically by name before serialization, including GPT-5.6 Lite `additional_tools` input.
- Added provider and session tests for cache accounting, OAuth metadata, headers, and deterministic tool ordering.
- Removed temporary raw LLM and cache-diagnostic capture code and deleted its generated session files after analysis.
- Included the pre-existing `AGENTS.md` commit-workflow wording update.

## Decisions And Rationale

- Keep `prompt_cache_key` stable per session and reuse it after model switches so the backend can route matching prefixes consistently.
- Use the SHA-256 digest without a prefix because the ChatGPT OAuth backend rejects keys longer than 64 characters.
- Sort tools before serialization because OpenAI prompt caching requires an exact prefix; changing only the position of `shell` invalidated the cache prefix.
- Do not send `prompt_cache_options` or explicit breakpoints because the ChatGPT subscription OAuth endpoint returned HTTP 400 for that public API parameter.
- Retain token usage aggregation but remove raw prompt/response diagnostics after the root cause was identified.
