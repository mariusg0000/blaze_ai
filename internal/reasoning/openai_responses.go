// openai_responses.go — OpenAI Responses/Codex reasoning descriptor.
// Maps standard reasoning levels to the nested `reasoning.effort` field
// used by the /responses endpoint. Clamps `max` to `xhigh` per codex-rs
// behavior where Ultra is capped before sending.
// Layer: pure domain logic. Dependencies: reasoning/levels.go only.
package reasoning

import "strings"

// openaiResponsesSupportedModels lists OpenAI model name prefixes that accept
// reasoning.effort in the Responses API.
//
// WHAT:  Known reasoning-capable model families for the Responses path.
// WHY:   The normalizer must reject non-reasoning models explicitly (no fallback).
// HOW:   Prefix-based matching covers all known variants without listing every slug.
var openaiResponsesSupportedModels = []string{
	"o1",
	"o3",
	"o4",
	"gpt-5",
	"codex",
}

// openaiResponsesSupportedLevels defines the reasoning levels accepted by
// the Responses API.
//
// WHAT:  The Responses API accepts none through max (max is clamped to xhigh
//
//	in the transform).
//
// WHY:   codex-rs research shows Ultra is clamped to the wire value before sending;
//
//	our standard level set includes max for semantic completeness.
var openaiResponsesSupportedLevels = []string{
	LevelNone, LevelMin, LevelLow, LevelMed, LevelHigh, LevelXHigh, LevelMax,
}

func init() {
	Register("openai_responses", &Descriptor{
		SupportedLevels: openaiResponsesSupportedLevels,
		DefaultLevel:    LevelMed,
		Transform:       transformOpenAIResponses,
		IsModelSupported: func(modelID string) bool {
			return isOpenAIResponsesModel(modelID)
		},
	})
}

// isOpenAIResponsesModel checks whether a model name is a known OpenAI
// reasoning model for the Responses API.
//
// WHAT:  Prefix-based model capability check.
// WHY:   Avoids maintaining an exhaustive list of every model slug.
// PARAMS: modelID — bare model name or provider/model name (e.g., "o3",
//
//	"o3-mini", "openai/o3", "gpt-5-chat").
//
// RETURNS: true if the model matches any known reasoning prefix.
func isOpenAIResponsesModel(modelID string) bool {
	lower := strings.ToLower(modelID)
	for _, prefix := range openaiResponsesSupportedModels {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
		// Handle provider/model format (e.g., "openai/o3").
		if idx := strings.Index(lower, "/"); idx >= 0 {
			if strings.HasPrefix(lower[idx+1:], prefix) {
				return true
			}
		}
	}
	return false
}

// transformOpenAIResponses converts a standard level to the Responses API
// request fragment: {"reasoning": {"effort": "<wire_level>"}}.
//
// WHAT:  Produces the nested reasoning.effort field for /responses.
// WHY:   The Responses API requires a nested structure inside a "reasoning" object.
//
//	HOW: Clamps `max` to `xhigh` per codex-rs behavior, then maps to OpenAI wire value.
//
// PARAMS: level — a validated standard reasoning level.
// RETURNS: map[string]any with a nested "reasoning" object containing "effort".
func transformOpenAIResponses(level string) (map[string]any, error) {
	wireLevel := level
	if level == LevelMax {
		// codex-rs clamps Ultra/max to the highest wire value before sending.
		wireLevel = LevelXHigh
	}
	return map[string]any{
		"reasoning": map[string]any{
			"effort": openaiWireLevel(wireLevel),
		},
	}, nil
}
