// descriptor.go — reasoning descriptor registry with strict validation.
// Provides Normalize, Supported, Default, and IsReasoningCapable functions
// that map standard reasoning levels to provider-specific request fragments.
// Layer: pure domain logic. Dependencies: none (internal package only).
package reasoning

import (
	"fmt"
	"strings"
)

// Descriptor defines reasoning support for one provider/API-path combination.
//
// WHAT:  Encapsulates the supported levels, default, model capability, and transform
//
//	for a specific provider and API path (e.g., OpenAI Chat Completions vs Responses).
//
// WHY:   Each API path has its own request-body shape and supported level set;
//
//	a descriptor per path keeps normalization clean and extensible.
//
// PARAMS: SupportedLevels — allowed standard levels; DefaultLevel — fallback level;
//
//	Transform — converts a standard level to a JSON-serializable request fragment;
//	IsModelSupported — reports whether a bare model name supports reasoning.
type Descriptor struct {
	SupportedLevels  []string
	DefaultLevel     string
	Transform        func(level string) (map[string]any, error)
	IsModelSupported func(modelID string) bool
}

// registry holds all registered descriptors keyed by "provider/apiPath".
//
// WHAT:  Global map populated by init() functions in provider-specific files.
// WHY:   The caller (provider code) selects the descriptor by key; the normalizer
//
//	is itself API-path agnostic.
var registry = make(map[string]*Descriptor)

// Register adds a descriptor under the given key.
//
// WHAT:  Registers a provider descriptor at package init time.
// WHY:   Descriptors are declared in separate files and self-register.
// PARAMS: key — unique identifier like "openai_chat" or "openai_responses";
//
//	d — the descriptor to register.
func Register(key string, d *Descriptor) {
	registry[key] = d
}

// lookup finds a descriptor by key, returning nil if not found.
func lookup(key string) *Descriptor {
	return registry[key]
}

// Normalize validates a reasoning level and returns the JSON request-body fragment.
//
// WHAT:  The primary entry point for provider request construction.
// WHY:   Centralizes level validation, model capability check, and transform in
//
//	one strict function; returns an error for every failure mode.
//
// HOW:   Looks up the descriptor, checks model support, validates the level,
//
//	and calls the descriptor's Transform function.
//
// PARAMS: provider — descriptor key (e.g., "openai_chat", "openai_responses");
//
//	modelID — bare model name (e.g., "o3", "gpt-5-chat"); level — requested standard level.
//
// RETURNS: map[string]any — JSON request fragment to merge into the request body;
//
//	error if provider is unknown, model is not reasoning-capable, or level is invalid.
func Normalize(provider, modelID, level string) (map[string]any, error) {
	d := lookup(provider)
	if d == nil {
		return nil, fmt.Errorf("reasoning: unknown provider descriptor: %s", provider)
	}
	if !d.IsModelSupported(modelID) {
		return nil, fmt.Errorf("reasoning: model %s does not support reasoning via %s", modelID, provider)
	}
	if !IsValidLevel(level) {
		return nil, fmt.Errorf("reasoning: invalid level %q; valid levels: %s", level, strings.Join(ValidLevels, ", "))
	}
	if !levelSupported(d.SupportedLevels, level) {
		return nil, fmt.Errorf("reasoning: level %q is not supported by %s for model %s; supported: %s",
			level, provider, modelID, strings.Join(d.SupportedLevels, ", "))
	}
	return d.Transform(level)
}

// Supported returns the allowed reasoning levels for a model and provider.
//
// WHAT:  Exposes the supported levels for UI display and level cycling.
// WHY:   The status bar and future /reasoning command need to know what's allowed.
// HOW:   Looks up the descriptor; returns nil if provider is unknown or model is
//
//	not reasoning-capable.
//
// PARAMS: provider — descriptor key; modelID — bare model name.
// RETURNS: []string — allowed levels, or nil if not applicable.
func Supported(provider, modelID string) []string {
	d := lookup(provider)
	if d == nil {
		return nil
	}
	if !d.IsModelSupported(modelID) {
		return nil
	}
	out := make([]string, len(d.SupportedLevels))
	copy(out, d.SupportedLevels)
	return out
}

// Default returns the default reasoning level for a model and provider.
//
// WHAT:  Provides the fallback level when no level is explicitly configured.
// WHY:   New sessions and model switches need a sensible default.
// HOW:   Returns the descriptor's DefaultLevel; empty string if provider or model
//
//	is unknown.
//
// PARAMS: provider — descriptor key; modelID — bare model name.
// RETURNS: string — the default level, or "" if not applicable.
func Default(provider, modelID string) string {
	d := lookup(provider)
	if d == nil {
		return ""
	}
	if !d.IsModelSupported(modelID) {
		return ""
	}
	return d.DefaultLevel
}

// IsReasoningCapable reports whether the model supports reasoning through the
// given provider descriptor.
//
// WHAT:  Simple capability check without level validation.
// WHY:   The runtime and status bar need to know whether to show reasoning controls.
// PARAMS: provider — descriptor key; modelID — bare model name.
// RETURNS: true if the provider is known and the model is supported.
func IsReasoningCapable(provider, modelID string) bool {
	d := lookup(provider)
	if d == nil {
		return false
	}
	return d.IsModelSupported(modelID)
}

// levelSupported checks if a level is in the allowed list.
func levelSupported(allowed []string, level string) bool {
	for _, l := range allowed {
		if l == level {
			return true
		}
	}
	return false
}

// splitModelID separates a full provider/model_name identifier into its parts.
//
// WHAT:  Extracts the provider prefix and bare model name from a full model ID.
// WHY:   The runtime uses full model IDs (e.g., "openrouter/openai/o3") but
//
//	descriptor model checks operate on bare model names.
//
// HOW:   Splits on the first "/" only; a model without "/" returns ("", modelID).
//
// PARAMS: modelID — full or bare model identifier.
// RETURNS: provider name and bare model name.
func splitModelID(modelID string) (string, string) {
	idx := strings.Index(modelID, "/")
	if idx < 0 {
		return "", modelID
	}
	return modelID[:idx], modelID[idx+1:]
}

// ValidateLevel checks whether level is one of the standard reasoning levels.
//
// WHAT:  Strict validation of the abstract reasoning level string.
// WHY:   SetActiveReasoningLevel must reject unknown strings without fallback.
// PARAMS: level — candidate level string.
// RETURNS: nil if valid; error listing valid levels otherwise.
func ValidateLevel(level string) error {
	if !IsValidLevel(level) {
		return fmt.Errorf("invalid reasoning level %q: must be one of: %s", level, strings.Join(ValidLevels, ", "))
	}
	return nil
}

// IsReasoningCapableForModel reports whether a full model ID supports reasoning.
//
// WHAT:  Model-aware capability check using the OpenAI Chat Completions descriptor
//
//	as the default wire-shape path.
//
// WHY:   The runtime and console need capability checks against full model IDs
//
//	(e.g., "openrouter/openai/o3") without knowing the internal descriptor key.
//
// PARAMS: modelID — full provider/model_name identifier.
// RETURNS: true if the model matches a known reasoning prefix.
func IsReasoningCapableForModel(modelID string) bool {
	_, bareModel := splitModelID(modelID)
	return IsReasoningCapable("openai_chat", bareModel)
}

// DefaultForModel returns the default reasoning level for a full model ID.
//
// WHAT:  Model-aware default lookup using the OpenAI Chat Completions descriptor.
// WHY:   The runtime needs the default level when no level is explicitly configured.
// HOW:   Extracts the bare model name and delegates to the descriptor.
//
// PARAMS: modelID — full provider/model_name identifier.
// RETURNS: the default level string, or "" if the model is not reasoning-capable.
func DefaultForModel(modelID string) string {
	_, bareModel := splitModelID(modelID)
	return Default("openai_chat", bareModel)
}

// SupportedForModel returns the supported reasoning levels for a full model ID.
//
// WHAT:  Model-aware level list using the OpenAI Chat Completions descriptor.
// WHY:   UI and cycling need the allowed levels for the active model.
// HOW:   Extracts the bare model name and delegates to the descriptor.
//
// PARAMS: modelID — full provider/model_name identifier.
// RETURNS: slice of allowed levels, or nil if not reasoning-capable.
func SupportedForModel(modelID string) []string {
	_, bareModel := splitModelID(modelID)
	return Supported("openai_chat", bareModel)
}
