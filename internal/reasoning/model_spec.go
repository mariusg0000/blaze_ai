// model_spec.go — canonical model identifier with optional reasoning level suffix.
// Provides ModelSpec parsing for the `model_id[|reasoning_level]` syntax used
// throughout config, modes, roles, agents, provider construction, and runtime.
// Layer: pure domain logic. Dependencies: none (internal package only).
package reasoning

import (
	"fmt"
	"strings"
)

// ModelSpec holds a parsed model identifier with an optional reasoning level suffix.
//
// WHAT:  Separates the base model identifier from its reasoning level suffix.
// WHY:   The suffix syntax `model_id|level` is the single source of truth for
//
//	reasoning configuration; every model identifier is parsed through ModelSpec
//	before validation, provider construction, or API normalization.
//
// PARAMS: ModelID — the bare provider/model_name without suffix;
//
//	ReasoningLevel — the standard reasoning level, or "" if no suffix was present.
type ModelSpec struct {
	ModelID        string
	ReasoningLevel string
}

// HasReasoning reports whether the ModelSpec includes a non-empty reasoning level.
func (ms ModelSpec) HasReasoning() bool {
	return ms.ReasoningLevel != ""
}

// ParseModelSpec splits a model identifier on the LAST '|' separator.
//
// WHAT:  Extracts the base model ID and optional reasoning level from `model_id[|level]`.
// WHY:   Every config, mode, role, agent, and runtime model string is parsed through this
//
//	function so that the suffix is the sole source of reasoning configuration.
//
// HOW:   Searches for the last '|' in the string. If found, the part after the last '|'
//
//	is validated as a standard reasoning level. If no '|' is found, the level is empty.
//	The model ID part is never validated here (just passed through).
//
// PARAMS: fullID — the full model string, optionally with `|level` suffix.
// RETURNS: ModelSpec — parsed components; error if the suffix is present but invalid,
//
//	or if the model ID part would be empty after stripping the suffix.
//
// EXAMPLES:
//
//	ParseModelSpec("openrouter/openai/o3|max") → {ModelID: "openrouter/openai/o3", ReasoningLevel: "max"}
//	ParseModelSpec("openrouter/o3") → {ModelID: "openrouter/o3", ReasoningLevel: ""}
//	ParseModelSpec("o3|invalid") → error
//	ParseModelSpec("|max") → error (empty model ID)
func ParseModelSpec(fullID string) (ModelSpec, error) {
	if fullID == "" {
		return ModelSpec{}, fmt.Errorf("model identifier is empty")
	}
	lastPipe := strings.LastIndex(fullID, "|")
	if lastPipe < 0 {
		return ModelSpec{ModelID: fullID}, nil
	}
	modelID := fullID[:lastPipe]
	level := fullID[lastPipe+1:]
	if modelID == "" {
		return ModelSpec{}, fmt.Errorf("model identifier is empty before | separator")
	}
	if level == "" {
		return ModelSpec{}, fmt.Errorf("reasoning level suffix is empty after | separator")
	}
	if !IsValidLevel(level) {
		return ModelSpec{}, fmt.Errorf("invalid reasoning level %q: must be one of: %s",
			level, strings.Join(ValidLevels, ", "))
	}
	return ModelSpec{ModelID: modelID, ReasoningLevel: level}, nil
}
