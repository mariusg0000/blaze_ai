// openai_chat_test.go — tests for OpenAI Chat Completions reasoning transform.
package reasoning

import "testing"

// TestOpenAIChatTransformAllLevels verifies the transform produces correct wire values.
func TestOpenAIChatTransformAllLevels(t *testing.T) {
	for _, level := range openaiChatSupportedLevels {
		fragment, err := transformOpenAIChat(level)
		if err != nil {
			t.Fatalf("transformOpenAIChat(%q) error: %v", level, err)
		}
		got, ok := fragment["reasoning_effort"]
		if !ok {
			t.Fatalf("transformOpenAIChat(%q) missing reasoning_effort", level)
		}
		want := openaiWireLevel(level)
		if level == LevelMax {
			// max is clamped to xhigh for the Chat Completions wire format.
			want = openaiWireLevel(LevelXHigh)
		}
		if got != want {
			t.Errorf("transformOpenAIChat(%q) reasoning_effort = %v, want %q (wire)", level, got, want)
		}
	}
}

// TestOpenAIChatModelVariations verifies various OpenAI model name patterns.
func TestOpenAIChatModelVariations(t *testing.T) {
	cases := []struct {
		model    string
		expected bool
	}{
		{"o3", true},
		{"o3-mini", true},
		{"O3", true},
		{"O3-MINI", true},
		{"o1", true},
		{"o1-preview", true},
		{"o1-mini", true},
		{"o4-mini", true},
		{"gpt-5", true},
		{"gpt-5-chat", true},
		{"gpt-5.6-2025-07-16", true},
		{"codex-mini", true},
		{"gpt-4o", false},
		{"gpt-4", false},
		{"claude-3-opus", false},
		{"gemini-pro", false},
	}
	for _, tc := range cases {
		got := isOpenAIChatModel(tc.model)
		if got != tc.expected {
			t.Errorf("isOpenAIChatModel(%q) = %v, want %v", tc.model, got, tc.expected)
		}
	}
}

// TestOpenAIChatDescriptorRegistered verifies the descriptor was registered at init.
func TestOpenAIChatDescriptorRegistered(t *testing.T) {
	d := lookup("openai_chat")
	if d == nil {
		t.Fatal("openai_chat descriptor not registered")
	}
	if d.DefaultLevel != "med" {
		t.Errorf("DefaultLevel = %q, want med", d.DefaultLevel)
	}
	if len(d.SupportedLevels) != 7 {
		t.Errorf("SupportedLevels = %d entries, want 7 (includes max)", len(d.SupportedLevels))
	}
}

// TestOpenAIChatTransformMaxClamped verifies max is clamped to xhigh wire value.
func TestOpenAIChatTransformMaxClamped(t *testing.T) {
	fragment, err := transformOpenAIChat(LevelMax)
	if err != nil {
		t.Fatalf("transformOpenAIChat(max) error: %v", err)
	}
	got := fragment["reasoning_effort"]
	want := openaiWireLevel(LevelXHigh)
	if got != want {
		t.Errorf("transformOpenAIChat(max) reasoning_effort = %v, want %q", got, want)
	}
}

// TestOpenAIChatModelProviderFormat verifies provider/model format is recognized.
func TestOpenAIChatModelProviderFormat(t *testing.T) {
	cases := []struct {
		model    string
		expected bool
	}{
		{"openai/o3", true},
		{"openai/o3-mini", true},
		{"openai/gpt-5", true},
		{"anthropic/claude-3", false},
	}
	for _, tc := range cases {
		got := isOpenAIChatModel(tc.model)
		if got != tc.expected {
			t.Errorf("isOpenAIChatModel(%q) = %v, want %v", tc.model, got, tc.expected)
		}
	}
}

// TestOpenAIChatTransformMinAndMed verifies the critical min→minimal and med→medium wire mapping.
func TestOpenAIChatTransformMinAndMed(t *testing.T) {
	fragMin, err := transformOpenAIChat(LevelMin)
	if err != nil {
		t.Fatalf("transformOpenAIChat(min) error: %v", err)
	}
	if fragMin["reasoning_effort"] != "minimal" {
		t.Errorf("min wire value = %v, want minimal", fragMin["reasoning_effort"])
	}
	fragMed, err := transformOpenAIChat(LevelMed)
	if err != nil {
		t.Fatalf("transformOpenAIChat(med) error: %v", err)
	}
	if fragMed["reasoning_effort"] != "medium" {
		t.Errorf("med wire value = %v, want medium", fragMed["reasoning_effort"])
	}
}
