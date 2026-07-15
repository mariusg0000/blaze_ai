// openai_chat.go — OpenAI Chat Completions reasoning descriptor.
// Maps standard reasoning levels to the `reasoning_effort` flat field
// used by the /chat/completions endpoint.
// Layer: pure domain logic. Dependencies: reasoning/levels.go only.
package reasoning

import "strings"

// openaiChatSupportedModels lists OpenAI model name prefixes that accept
// reasoning_effort in the Chat Completions API.
//
// WHAT:  Known reasoning-capable model families for the Chat Completions path.
// WHY:   The normalizer must reject non-reasoning models explicitly (no fallback).
// HOW:   Prefix-based matching covers all known variants without listing every slug.
var openaiChatSupportedModels = []string{
	"o1",
	"o3",
	"o4",
	"gpt-5",
	"codex",
}

// openaiChatSupportedLevels defines the reasoning levels accepted by the
// Chat Completions API.
//
// WHAT:  The Chat Completions API accepts none through max.
// WHY:   Each standard level maps directly to its OpenAI wire value.
var openaiChatSupportedLevels = []string{
	LevelNone, LevelMin, LevelLow, LevelMed, LevelHigh, LevelXHigh, LevelMax,
}

func init() {
	Register("openai_chat", &Descriptor{
		SupportedLevels: openaiChatSupportedLevels,
		DefaultLevel:    LevelMed,
		Transform:       transformOpenAIChat,
		IsModelSupported: func(modelID string) bool {
			return isOpenAIChatModel(modelID)
		},
	})
}

// isOpenAIChatModel checks whether a bare model name is a known OpenAI
// reasoning model for the Chat Completions API.
//
// WHAT:  Prefix-based model capability check.
// WHY:   Avoids maintaining an exhaustive list of every model slug.
// PARAMS: modelID — bare model name (e.g., "o3", "o3-mini", "gpt-5-chat").
// RETURNS: true if the model matches any known reasoning prefix.
func isOpenAIChatModel(modelID string) bool {
	lower := strings.ToLower(modelID)
	for _, prefix := range openaiChatSupportedModels {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
		// Check for provider/model format (e.g., "openai/o3") after any slash.
		if idx := strings.Index(lower, "/"); idx >= 0 {
			if strings.HasPrefix(lower[idx+1:], prefix) {
				return true
			}
		}
	}
	return false
}

// openaiWireMapping translates standard reasoning levels to OpenAI wire values.
//
// WHAT:  The OpenAI API accepts specific wire strings that differ from the standard names.
// WHY:   Standard "min" must become wire "minimal"; standard "med" must become "medium".
//
//	All other standard levels match their wire values exactly.
var openaiWireMapping = map[string]string{
	LevelNone:  "none",
	LevelMin:   "minimal",
	LevelLow:   "low",
	LevelMed:   "medium",
	LevelHigh:  "high",
	LevelXHigh: "xhigh",
	LevelMax:   "max",
}

// openaiWireLevel translates a standard reasoning level to its OpenAI wire value.
//
// WHAT:  Looks up the wire value for a standard level; passes through if not mapped.
// WHY:   Both Chat Completions and Responses descriptors share the same wire format.
// HOW:   The level is already validated upstream, so a missing key indicates a bug.
// PARAMS: level — a validated standard reasoning level.
// RETURNS: string — the OpenAI wire value.
func openaiWireLevel(level string) string {
	if wire, ok := openaiWireMapping[level]; ok {
		return wire
	}
	return level
}

// transformOpenAIChat converts a standard level to the Chat Completions
// request fragment: {"reasoning_effort": "<wire_level>"}.
//
// WHAT:  Produces the flat reasoning_effort field for /chat/completions.
// WHY:   Chat Completions uses a top-level string field, not nested.
//
//	HOW: Maps standard level to OpenAI wire value via openaiWireLevel.
//
// PARAMS: level — a validated standard reasoning level.
// RETURNS: map[string]any with a single "reasoning_effort" key.
func transformOpenAIChat(level string) (map[string]any, error) {
	return map[string]any{"reasoning_effort": openaiWireLevel(level)}, nil
}
