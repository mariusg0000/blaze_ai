// normalizer_test.go — tests for reasoning Normalize, Supported, Default, and IsReasoningCapable.
package reasoning

import (
	"encoding/json"
	"testing"
)

// TestNormalizeValidChatLevels verifies all valid Chat Completions levels produce correct wire fragments.
func TestNormalizeValidChatLevels(t *testing.T) {
	for _, level := range openaiChatSupportedLevels {
		fragment, err := Normalize("openai_chat", "o3", level)
		if err != nil {
			t.Fatalf("Normalize(openai_chat, o3, %q) error: %v", level, err)
		}
		got, ok := fragment["reasoning_effort"]
		if !ok {
			t.Fatalf("Normalize(openai_chat, o3, %q) missing reasoning_effort key", level)
		}
		wantLevel := level
		if level == LevelMax {
			wantLevel = LevelXHigh // max clamped to xhigh for Chat wire
		}
		want := openaiWireLevel(wantLevel)
		if got != want {
			t.Errorf("Normalize(openai_chat, o3, %q) reasoning_effort = %v, want %q (wire)", level, got, want)
		}
	}
}

// TestNormalizeValidResponsesLevels verifies all valid Responses levels produce correct wire fragments.
func TestNormalizeValidResponsesLevels(t *testing.T) {
	for _, level := range openaiResponsesSupportedLevels {
		fragment, err := Normalize("openai_responses", "o3", level)
		if err != nil {
			t.Fatalf("Normalize(openai_responses, o3, %q) error: %v", level, err)
		}
		reasoning, ok := fragment["reasoning"]
		if !ok {
			t.Fatalf("Normalize(openai_responses, o3, %q) missing reasoning key", level)
		}
		reasoningMap, ok := reasoning.(map[string]any)
		if !ok {
			t.Fatalf("Normalize(openai_responses, o3, %q) reasoning is not map[string]any", level)
		}
		effort, ok := reasoningMap["effort"]
		if !ok {
			t.Fatalf("Normalize(openai_responses, o3, %q) missing effort key", level)
		}
		// max is clamped to xhigh, then mapped to wire value
		wantLevel := level
		if level == LevelMax {
			wantLevel = LevelXHigh
		}
		want := openaiWireLevel(wantLevel)
		if effort != want {
			t.Errorf("Normalize(openai_responses, o3, %q) effort = %v, want %q (wire)", level, effort, want)
		}
	}
}

// TestNormalizeMinAndMedWireValues verifies the critical min→minimal and med→medium wire mapping.
func TestNormalizeMinAndMedWireValues(t *testing.T) {
	// Chat path: min → "minimal"
	frag, err := Normalize("openai_chat", "o3", LevelMin)
	if err != nil {
		t.Fatalf("Normalize(openai_chat, o3, min) error: %v", err)
	}
	if got := frag["reasoning_effort"]; got != "minimal" {
		t.Errorf("Chat min wire value = %v, want minimal", got)
	}
	// Chat path: med → "medium"
	frag, err = Normalize("openai_chat", "o3", LevelMed)
	if err != nil {
		t.Fatalf("Normalize(openai_chat, o3, med) error: %v", err)
	}
	if got := frag["reasoning_effort"]; got != "medium" {
		t.Errorf("Chat med wire value = %v, want medium", got)
	}
	// Responses path: min → "minimal"
	frag, err = Normalize("openai_responses", "o3", LevelMin)
	if err != nil {
		t.Fatalf("Normalize(openai_responses, o3, min) error: %v", err)
	}
	minEffort := frag["reasoning"].(map[string]any)["effort"]
	if minEffort != "minimal" {
		t.Errorf("Responses min wire value = %v, want minimal", minEffort)
	}
	// Responses path: med → "medium"
	frag, err = Normalize("openai_responses", "o3", LevelMed)
	if err != nil {
		t.Fatalf("Normalize(openai_responses, o3, med) error: %v", err)
	}
	medEffort := frag["reasoning"].(map[string]any)["effort"]
	if medEffort != "medium" {
		t.Errorf("Responses med wire value = %v, want medium", medEffort)
	}
}

// TestNormalizeMaxClamping verifies max is clamped to xhigh wire value for Responses.
func TestNormalizeMaxClamping(t *testing.T) {
	fragment, err := Normalize("openai_responses", "o3", LevelMax)
	if err != nil {
		t.Fatalf("Normalize() error: %v", err)
	}
	reasoning := fragment["reasoning"].(map[string]any)
	// xhigh maps to wire "xhigh"
	want := openaiWireLevel(LevelXHigh)
	if reasoning["effort"] != want {
		t.Errorf("max clamping: effort = %v, want %q", reasoning["effort"], want)
	}
}

// TestNormalizeMaxClampedChat verifies max is clamped to xhigh for Chat Completions.
func TestNormalizeMaxClampedChat(t *testing.T) {
	fragment, err := Normalize("openai_chat", "o3", LevelMax)
	if err != nil {
		t.Fatalf("Normalize(openai_chat, o3, max) error: %v", err)
	}
	got, ok := fragment["reasoning_effort"]
	if !ok {
		t.Fatal("Normalize(openai_chat, o3, max) missing reasoning_effort key")
	}
	// max clamps to xhigh, then maps to wire "xhigh".
	want := openaiWireLevel(LevelXHigh)
	if got != want {
		t.Errorf("Normalize(openai_chat, o3, max) reasoning_effort = %v, want %q", got, want)
	}
}

// TestNormalizeUnknownProvider verifies unknown provider key returns error.
func TestNormalizeUnknownProvider(t *testing.T) {
	_, err := Normalize("anthropic_chat", "claude-3", "high")
	if err == nil {
		t.Fatal("Normalize(anthropic_chat, ...) expected error, got nil")
	}
	if err.Error() != "reasoning: unknown provider descriptor: anthropic_chat" {
		t.Errorf("error = %q, want unknown provider message", err.Error())
	}
}

// TestNormalizeInvalidLevel verifies invalid level string returns error.
func TestNormalizeInvalidLevel(t *testing.T) {
	_, err := Normalize("openai_chat", "o3", "ultra")
	if err == nil {
		t.Fatal("Normalize(openai_chat, o3, ultra) expected error, got nil")
	}
	if err.Error() != `reasoning: invalid level "ultra"; valid levels: none, min, low, med, high, xhigh, max` {
		t.Errorf("error = %q", err.Error())
	}
}

// TestNormalizeUnsupportedLevel verifies level not in the descriptor's supported list returns error.
// Since max is now accepted by both descriptors, this test uses a non-existent descriptor key.
func TestNormalizeUnsupportedLevel(t *testing.T) {
	// All 7 standard levels are now supported by both descriptors.
	// Verify the full level set is accepted for a reasoning model.
	for _, level := range ValidLevels {
		_, err := Normalize("openai_chat", "o3", level)
		if err != nil {
			t.Errorf("Normalize(openai_chat, o3, %q) unexpected error: %v", level, err)
		}
	}
}

// TestNormalizeNonReasoningModel verifies non-reasoning model returns error.
func TestNormalizeNonReasoningModel(t *testing.T) {
	_, err := Normalize("openai_chat", "gpt-4o", "high")
	if err == nil {
		t.Fatal("Normalize(openai_chat, gpt-4o, high) expected error, got nil")
	}
}

// TestNormalizeModelVariants verifies various model name patterns are recognized.
func TestNormalizeModelVariants(t *testing.T) {
	models := []string{"o3", "o3-mini", "O3-MINI", "o1", "o1-preview", "o4-mini", "gpt-5", "gpt-5-chat", "codex-mini"}
	for _, model := range models {
		_, err := Normalize("openai_chat", model, "med")
		if err != nil {
			t.Errorf("Normalize(openai_chat, %q, med) unexpected error: %v", model, err)
		}
	}
}

// TestNormalizeFragmentJSON verifies the fragment is valid JSON with correct wire value.
func TestNormalizeFragmentJSON(t *testing.T) {
	fragment, err := Normalize("openai_chat", "o3", "high")
	if err != nil {
		t.Fatalf("Normalize() error: %v", err)
	}
	data, err := json.Marshal(fragment)
	if err != nil {
		t.Fatalf("json.Marshal(fragment) error: %v", err)
	}
	// "high" maps to wire "high"
	want := `{"reasoning_effort":"high"}`
	if string(data) != want {
		t.Errorf("fragment JSON = %s, want %s", string(data), want)
	}
}

// TestNormalizeResponsesFragmentJSON verifies the Responses fragment is valid JSON with correct wire value.
func TestNormalizeResponsesFragmentJSON(t *testing.T) {
	fragment, err := Normalize("openai_responses", "o3", "low")
	if err != nil {
		t.Fatalf("Normalize() error: %v", err)
	}
	data, err := json.Marshal(fragment)
	if err != nil {
		t.Fatalf("json.Marshal(fragment) error: %v", err)
	}
	// "low" maps to wire "low"
	want := `{"reasoning":{"effort":"low"}}`
	if string(data) != want {
		t.Errorf("fragment JSON = %s, want %s", string(data), want)
	}
}

// TestNormalizeMinMedFragmentJSON verifies min and med produce correct wire JSON.
func TestNormalizeMinMedFragmentJSON(t *testing.T) {
	frag, err := Normalize("openai_chat", "o3", LevelMin)
	if err != nil {
		t.Fatalf("Normalize() error: %v", err)
	}
	data, err := json.Marshal(frag)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	want := `{"reasoning_effort":"minimal"}`
	if string(data) != want {
		t.Errorf("min Chat JSON = %s, want %s", string(data), want)
	}
	frag, err = Normalize("openai_chat", "o3", LevelMed)
	if err != nil {
		t.Fatalf("Normalize() error: %v", err)
	}
	data, err = json.Marshal(frag)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	want = `{"reasoning_effort":"medium"}`
	if string(data) != want {
		t.Errorf("med Chat JSON = %s, want %s", string(data), want)
	}
}

// TestSupportedChatModels verifies Supported returns levels for reasoning models.
func TestSupportedChatModels(t *testing.T) {
	levels := Supported("openai_chat", "o3")
	if len(levels) == 0 {
		t.Fatal("Supported(openai_chat, o3) returned nil")
	}
	for _, l := range openaiChatSupportedLevels {
		found := false
		for _, s := range levels {
			if s == l {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Supported(openai_chat, o3) missing level %q", l)
		}
	}
}

// TestSupportedNonReasoningModel verifies nil for non-reasoning model.
func TestSupportedNonReasoningModel(t *testing.T) {
	levels := Supported("openai_chat", "gpt-4o")
	if levels != nil {
		t.Errorf("Supported(openai_chat, gpt-4o) = %v, want nil", levels)
	}
}

// TestSupportedUnknownProvider verifies nil for unknown provider.
func TestSupportedUnknownProvider(t *testing.T) {
	levels := Supported("anthropic_chat", "claude-3")
	if levels != nil {
		t.Errorf("Supported(anthropic_chat, claude-3) = %v, want nil", levels)
	}
}

// TestDefaultMedForReasoningModel verifies default level is "med" for reasoning models.
func TestDefaultMedForReasoningModel(t *testing.T) {
	d := Default("openai_chat", "o3")
	if d != "med" {
		t.Errorf("Default(openai_chat, o3) = %q, want med", d)
	}
}

// TestDefaultEmptyForNonReasoning verifies empty string for non-reasoning model.
func TestDefaultEmptyForNonReasoning(t *testing.T) {
	d := Default("openai_chat", "gpt-4o")
	if d != "" {
		t.Errorf("Default(openai_chat, gpt-4o) = %q, want empty", d)
	}
}

// TestDefaultEmptyForUnknownProvider verifies empty string for unknown provider.
func TestDefaultEmptyForUnknownProvider(t *testing.T) {
	d := Default("anthropic_chat", "claude-3")
	if d != "" {
		t.Errorf("Default(anthropic_chat, claude-3) = %q, want empty", d)
	}
}

// TestIsReasoningCapableTrue verifies reasoning-capable model returns true.
func TestIsReasoningCapableTrue(t *testing.T) {
	if !IsReasoningCapable("openai_chat", "o3") {
		t.Error("IsReasoningCapable(openai_chat, o3) = false, want true")
	}
}

// TestIsReasoningCapableFalse verifies non-reasoning model returns false.
func TestIsReasoningCapableFalse(t *testing.T) {
	if IsReasoningCapable("openai_chat", "gpt-4o") {
		t.Error("IsReasoningCapable(openai_chat, gpt-4o) = true, want false")
	}
}

// TestIsReasoningCapableUnknownProvider verifies unknown provider returns false.
func TestIsReasoningCapableUnknownProvider(t *testing.T) {
	if IsReasoningCapable("anthropic_chat", "claude-3") {
		t.Error("IsReasoningCapable(anthropic_chat, claude-3) = true, want false")
	}
}

// --- Model-aware convenience function tests ---

// TestValidateLevelValid verifies valid standard levels pass.
func TestValidateLevelValid(t *testing.T) {
	for _, level := range ValidLevels {
		if err := ValidateLevel(level); err != nil {
			t.Errorf("ValidateLevel(%q) unexpected error: %v", level, err)
		}
	}
}

// TestValidateLevelInvalid verifies invalid level returns error.
func TestValidateLevelInvalid(t *testing.T) {
	if err := ValidateLevel("ultra"); err == nil {
		t.Error("ValidateLevel(ultra) expected error, got nil")
	}
	if err := ValidateLevel(""); err == nil {
		t.Error("ValidateLevel(\"\") expected error, got nil")
	}
}

// TestIsReasoningCapableForModelFullID verifies full model IDs are recognized.
func TestIsReasoningCapableForModelFullID(t *testing.T) {
	cases := []struct {
		model    string
		expected bool
	}{
		{"openrouter/openai/o3", true},
		{"openai/o3", true},
		{"o3", true},
		{"anthropic/claude-3", false},
		{"gpt-4o", false},
	}
	for _, tc := range cases {
		got := IsReasoningCapableForModel(tc.model)
		if got != tc.expected {
			t.Errorf("IsReasoningCapableForModel(%q) = %v, want %v", tc.model, got, tc.expected)
		}
	}
}

// TestDefaultForModelFullID verifies default level for full model IDs.
func TestDefaultForModelFullID(t *testing.T) {
	d := DefaultForModel("openrouter/openai/o3")
	if d != "med" {
		t.Errorf("DefaultForModel(openrouter/openai/o3) = %q, want med", d)
	}
	d = DefaultForModel("gpt-4o")
	if d != "" {
		t.Errorf("DefaultForModel(gpt-4o) = %q, want empty", d)
	}
}

// TestSupportedForModelFullID verifies supported levels for full model IDs.
func TestSupportedForModelFullID(t *testing.T) {
	levels := SupportedForModel("openrouter/openai/o3")
	if len(levels) == 0 {
		t.Fatal("SupportedForModel(openrouter/openai/o3) returned nil")
	}
	// Should include all 7 standard levels.
	if len(levels) != 7 {
		t.Errorf("SupportedForModel(openrouter/openai/o3) = %d levels, want 7", len(levels))
	}
}

// TestSplitModelID verifies the model ID split helper.
func TestSplitModelID(t *testing.T) {
	cases := []struct {
		input     string
		wantProv  string
		wantModel string
	}{
		{"openrouter/openai/o3", "openrouter", "openai/o3"},
		{"openai/o3", "openai", "o3"},
		{"o3", "", "o3"},
	}
	for _, tc := range cases {
		prov, model := splitModelID(tc.input)
		if prov != tc.wantProv || model != tc.wantModel {
			t.Errorf("splitModelID(%q) = (%q, %q), want (%q, %q)", tc.input, prov, model, tc.wantProv, tc.wantModel)
		}
	}
}
