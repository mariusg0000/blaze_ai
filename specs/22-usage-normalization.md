# Usage Normalization

## Source Files

| File | Role |
|------|------|
| `internal/usage/usage.go` | Usage struct, rawUsage, Extract function |

## Overview

Provider-agnostic token usage extraction and normalization. Converts raw
provider JSON usage objects from SSE events into one standard `Usage` record
that the rest of the runtime uses for compaction triggers and CTX display.

## Usage Struct

```go
type Usage struct {
    PromptTokens        int    // total input tokens
    CompletionTokens    int    // total output tokens
    TotalTokens         int    // provider-reported total
    CachedTokens        int    // tokens served from prompt cache
    UncachedInputTokens int    // promptTokens - cachedTokens
    CacheWriteTokens    int    // tokens written to cache
    ReasoningTokens     int    // reasoning/thinking tokens in completion
    CacheStatus         string // "hit", "miss", or "unknown"
}
```

### Field Descriptions

| Field | Source | Meaning |
|-------|--------|---------|
| `PromptTokens` | `prompt_tokens` or `input_tokens` | Total input tokens sent to the model |
| `CompletionTokens` | `completion_tokens` or `output_tokens` | Total output tokens generated |
| `TotalTokens` | `total_tokens` | Provider-reported total (may be 0) |
| `CachedTokens` | `cached_tokens` or `prompt_cache_hit_tokens` | Tokens served from prompt cache |
| `UncachedInputTokens` | `prompt_cache_miss_tokens` or computed | Input tokens not served from cache |
| `CacheWriteTokens` | `cache_write_tokens` | Tokens written to cache during this request |
| `ReasoningTokens` | `reasoning_tokens` | Tokens used for reasoning/thinking |
| `CacheStatus` | computed | `"hit"` if cached > 0, `"miss"` if cache detected but 0, `"unknown"` otherwise |

## Raw Usage Variants

The `rawUsage` struct handles multiple OpenAI-compatible response formats:

```go
type rawUsage struct {
    PromptTokens            int               `json:"prompt_tokens"`
    InputTokens             int               `json:"input_tokens"`
    CompletionTokens        int               `json:"completion_tokens"`
    OutputTokens            int               `json:"output_tokens"`
    TotalTokens             int               `json:"total_tokens"`
    InputTokensDetails      *tokenDetails     `json:"input_tokens_details"`
    PromptTokensDetails     *tokenDetails     `json:"prompt_tokens_details"`
    PromptCacheHitTokens    int               `json:"prompt_cache_hit_tokens"`
    PromptCacheMissTokens   int               `json:"prompt_cache_miss_tokens"`
    CompletionTokensDetails *reasoningDetails `json:"completion_tokens_details"`
    OutputTokensDetails     *reasoningDetails `json:"output_tokens_details"`
}
```

### Supported Provider Formats

| Provider Pattern | Input Tokens | Output Tokens | Cache Details |
|-----------------|--------------|---------------|---------------|
| OpenAI standard | `prompt_tokens` | `completion_tokens` | `prompt_tokens_details.cached_tokens` |
| Anthropic-style | `input_tokens` | `output_tokens` | `input_tokens_details.cached_tokens` |
| Cache hit/miss | `prompt_tokens` | `completion_tokens` | `prompt_cache_hit_tokens` / `prompt_cache_miss_tokens` |
| Responses API | `response.usage.*` | same | same |

## Extract Function

```go
func Extract(raw []byte) (*Usage, bool)
```

Reads one raw provider SSE event and returns usage when the event contains it.

### Extraction Logic

```
Extract(raw)
  ├─ Unmarshal JSON into { Usage, Response.Usage }
  ├─ Try top-level usage, then response.usage (Responses API)
  ├─ If neither found → return nil, false
  ├─ Normalize input tokens: prompt_tokens || input_tokens
  ├─ Normalize output tokens: completion_tokens || output_tokens
  ├─ Normalize cache:
  │    ├─ Try input_tokens_details.cached_tokens
  │    ├─ Try prompt_tokens_details.cached_tokens
  │    └─ Try prompt_cache_hit_tokens / prompt_cache_miss_tokens
  ├─ Compute uncached: promptTokens - cachedTokens
  ├─ Compute cache status: "hit" / "miss" / "unknown"
  ├─ Extract reasoning tokens from completion_tokens_details or output_tokens_details
  └─ Return Usage struct
```

### Return Values

- `(*Usage, true)` — usage found and normalized
- `(nil, false)` — event does not contain usage data (normal for content deltas)

## Usage in the Runtime

### Compaction Trigger

The primary consumer of usage data is the compaction system:

```go
// In RunTurn, after each LLM response:
OnUsage(resp.Usage.PromptTokens, resp.Usage.CachedTokens, resp.Usage.UncachedInputTokens)

// After tool execution loop:
Compactor.Compact(session, resp.Usage)
```

The compactor checks `usage.PromptTokens >= maxContextTokens` to decide
whether to trigger context compaction.

### CTX Display

The console transport displays token usage in tool result lines:
```
💻 Search files... ✔️ CTX: 45K
```

The `CachedTokens` and `UncachedInputTokens` breakdown is available for
debugging but not displayed in the default UI.

### Handler Callback

```go
OnUsage(promptTokens, cachedTokens, uncachedTokens int)
```

Three parameters give the transport full visibility into cache behavior:
- `promptTokens` — total input tokens
- `cachedTokens` — tokens served from cache
- `uncachedTokens` — tokens not cached (for cost estimation)
