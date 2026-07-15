// openai_responses_test.go — tests for OpenAI Responses/Codex reasoning transform.
package reasoning

import "testing"

// TestOpenAIResponsesTransformAllLevels verifies the transform produces correct wire values.
func TestOpenAIResponsesTransformAllLevels(t *testing.T) {
	for _, level := range openaiResponsesSupportedLevels {
		fragment, err := transformOpenAIResponses(level)
		if err != nil {
			t.Fatalf("transformOpenAIResponses(%q) error: %v", level, err)
		}
		reasoning, ok := fragment["reasoning"]
		if !ok {
			t.Fatalf("transformOpenAIResponses(%q) missing reasoning key", level)
		}
		reasoningMap, ok := reasoning.(map[string]any)
		if !ok {
			t.Fatalf("transformOpenAIResponses(%q) reasoning is not map[string]any", level)
		}
		effort, ok := reasoningMap["effort"]
		if !ok {
			t.Fatalf("transformOpenAIResponses(%q) missing effort key", level)
		}
		// max is clamped to xhigh, then both are mapped to wire values
		wantLevel := level
		if level == LevelMax {
			wantLevel = LevelXHigh
		}
		want := openaiWireLevel(wantLevel)
		if effort != want {
			t.Errorf("transformOpenAIResponses(%q) effort = %v, want %q (wire)", level, effort, want)
		}
	}
}

// TestOpenAIResponsesMaxClamped verifies max is clamped and mapped to xhigh wire value.
func TestOpenAIResponsesMaxClamped(t *testing.T) {
	fragment, err := transformOpenAIResponses(LevelMax)
	if err != nil {
		t.Fatalf("transformOpenAIResponses(max) error: %v", err)
	}
	reasoning := fragment["reasoning"].(map[string]any)
	want := openaiWireLevel(LevelXHigh)
	if reasoning["effort"] != want {
		t.Errorf("max wire value = %v, want %q", reasoning["effort"], want)
	}
}

// TestOpenAIResponsesNonMaxUnmapped verifies non-max levels are mapped to correct wire values.
func TestOpenAIResponsesNonMaxUnmapped(t *testing.T) {
	nonMaxLevels := []string{LevelNone, LevelMin, LevelLow, LevelMed, LevelHigh, LevelXHigh}
	for _, level := range nonMaxLevels {
		fragment, err := transformOpenAIResponses(level)
		if err != nil {
			t.Fatalf("transformOpenAIResponses(%q) error: %v", level, err)
		}
		reasoning := fragment["reasoning"].(map[string]any)
		want := openaiWireLevel(level)
		if reasoning["effort"] != want {
			t.Errorf("transformOpenAIResponses(%q) effort = %v, want %q (wire)", level, reasoning["effort"], want)
		}
	}
}

// TestOpenAIResponsesTransformMinAndMed verifies the critical min→minimal and med→medium wire mapping.
func TestOpenAIResponsesTransformMinAndMed(t *testing.T) {
	fragMin, err := transformOpenAIResponses(LevelMin)
	if err != nil {
		t.Fatalf("transformOpenAIResponses(min) error: %v", err)
	}
	minEffort := fragMin["reasoning"].(map[string]any)["effort"]
	if minEffort != "minimal" {
		t.Errorf("min wire value = %v, want minimal", minEffort)
	}
	fragMed, err := transformOpenAIResponses(LevelMed)
	if err != nil {
		t.Fatalf("transformOpenAIResponses(med) error: %v", err)
	}
	medEffort := fragMed["reasoning"].(map[string]any)["effort"]
	if medEffort != "medium" {
		t.Errorf("med wire value = %v, want medium", medEffort)
	}
}

// TestOpenAIResponsesModelVariations verifies various model name patterns.
func TestOpenAIResponsesModelVariations(t *testing.T) {
	cases := []struct {
		model    string
		expected bool
	}{
		{"o3", true},
		{"o3-mini", true},
		{"o3-pro", true},
		{"o1", true},
		{"o4-mini", true},
		{"gpt-5", true},
		{"gpt-5-chat", true},
		{"gpt-5.6-2025-07-16", true},
		{"codex-mini-latest", true},
		{"gpt-4o", false},
		{"dall-e-3", false},
		{"whisper-1", false},
		// Provider/model format.
		{"openai/o3", true},
		{"openai/gpt-5", true},
		{"anthropic/claude-3", false},
	}
	for _, tc := range cases {
		got := isOpenAIResponsesModel(tc.model)
		if got != tc.expected {
			t.Errorf("isOpenAIResponsesModel(%q) = %v, want %v", tc.model, got, tc.expected)
		}
	}
}

// TestOpenAIResponsesDescriptorRegistered verifies the descriptor was registered at init.
func TestOpenAIResponsesDescriptorRegistered(t *testing.T) {
	d := lookup("openai_responses")
	if d == nil {
		t.Fatal("openai_responses descriptor not registered")
	}
	if d.DefaultLevel != "med" {
		t.Errorf("DefaultLevel = %q, want med", d.DefaultLevel)
	}
	if len(d.SupportedLevels) != 7 {
		t.Errorf("SupportedLevels = %d entries, want 7 (includes max)", len(d.SupportedLevels))
	}
}
