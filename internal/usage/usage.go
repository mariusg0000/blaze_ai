// usage.go — provider-independent LLM token usage extraction and normalization.
// Converts raw provider JSON usage objects into one standard usage record.
// Layer: usage normalization. Dependencies: standard JSON decoding only.
package usage

import "encoding/json"

// Usage is the normalized token usage from one LLM response.
type Usage struct {
	PromptTokens        int
	CompletionTokens    int
	TotalTokens         int
	CachedTokens        int
	UncachedInputTokens int
	CacheWriteTokens    int
	ReasoningTokens     int
	CacheStatus         string
}

// rawUsage describes common OpenAI-compatible usage variants.
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

type tokenDetails struct {
	CachedTokens     int `json:"cached_tokens"`
	CacheWriteTokens int `json:"cache_write_tokens"`
}

type reasoningDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

// Extract reads one raw provider event and returns usage when the event contains it.
//
// WHAT: Finds usage under either `usage` or Responses `response.usage`.
// HOW:  Decodes provider JSON once, then normalizes supported field aliases.
func Extract(raw []byte) (*Usage, bool) {
	var event struct {
		Usage    *rawUsage `json:"usage"`
		Response *struct {
			Usage *rawUsage `json:"usage"`
		} `json:"response"`
	}
	if json.Unmarshal(raw, &event) != nil {
		return nil, false
	}
	candidate := event.Usage
	if candidate == nil && event.Response != nil {
		candidate = event.Response.Usage
	}
	if candidate == nil {
		return nil, false
	}

	input := candidate.PromptTokens
	if input == 0 {
		input = candidate.InputTokens
	}
	output := candidate.CompletionTokens
	if output == 0 {
		output = candidate.OutputTokens
	}
	cached, writes, hasCache := 0, 0, false
	for _, details := range []*tokenDetails{candidate.InputTokensDetails, candidate.PromptTokensDetails} {
		if details != nil {
			cached = details.CachedTokens
			writes = details.CacheWriteTokens
			hasCache = true
			break
		}
	}
	if !hasCache && (candidate.PromptCacheHitTokens > 0 || candidate.PromptCacheMissTokens > 0) {
		cached = candidate.PromptCacheHitTokens
		hasCache = true
	}
	uncached := candidate.PromptCacheMissTokens
	if uncached == 0 && hasCache {
		uncached = input - cached
	}
	status := "unknown"
	if hasCache {
		status = "miss"
		if cached > 0 {
			status = "hit"
		}
	}
	reasoning := 0
	if candidate.CompletionTokensDetails != nil {
		reasoning = candidate.CompletionTokensDetails.ReasoningTokens
	}
	if candidate.OutputTokensDetails != nil && candidate.OutputTokensDetails.ReasoningTokens > reasoning {
		reasoning = candidate.OutputTokensDetails.ReasoningTokens
	}
	return &Usage{
		PromptTokens: input, CompletionTokens: output, TotalTokens: candidate.TotalTokens,
		CachedTokens: cached, UncachedInputTokens: uncached, CacheWriteTokens: writes,
		ReasoningTokens: reasoning, CacheStatus: status,
	}, true
}
