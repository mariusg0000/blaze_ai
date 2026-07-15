// model_spec_test.go — tests for ModelSpec parsing and validation.
package reasoning

import (
	"strings"
	"testing"
)

// TestParseModelSpecNoSuffix verifies that a model without | returns empty level.
func TestParseModelSpecNoSuffix(t *testing.T) {
	spec, err := ParseModelSpec("openrouter/deepseek-v4-flash")
	if err != nil {
		t.Fatalf("ParseModelSpec() error: %v", err)
	}
	if spec.ModelID != "openrouter/deepseek-v4-flash" {
		t.Errorf("ModelID = %q, want 'openrouter/deepseek-v4-flash'", spec.ModelID)
	}
	if spec.ReasoningLevel != "" {
		t.Errorf("ReasoningLevel = %q, want empty", spec.ReasoningLevel)
	}
	if spec.HasReasoning() {
		t.Error("HasReasoning() = true, want false")
	}
}

// TestParseModelSpecWithSuffix verifies standard suffix extraction.
func TestParseModelSpecWithSuffix(t *testing.T) {
	spec, err := ParseModelSpec("openrouter/o3|high")
	if err != nil {
		t.Fatalf("ParseModelSpec() error: %v", err)
	}
	if spec.ModelID != "openrouter/o3" {
		t.Errorf("ModelID = %q, want 'openrouter/o3'", spec.ModelID)
	}
	if spec.ReasoningLevel != "high" {
		t.Errorf("ReasoningLevel = %q, want 'high'", spec.ReasoningLevel)
	}
	if !spec.HasReasoning() {
		t.Error("HasReasoning() = false, want true")
	}
}

// TestParseModelSpecLastPipe verifies that splitting works on the LAST | (preserving pipes in model name).
func TestParseModelSpecLastPipe(t *testing.T) {
	spec, err := ParseModelSpec("openrouter/openai/o3|max")
	if err != nil {
		t.Fatalf("ParseModelSpec() error: %v", err)
	}
	if spec.ModelID != "openrouter/openai/o3" {
		t.Errorf("ModelID = %q, want 'openrouter/openai/o3'", spec.ModelID)
	}
	if spec.ReasoningLevel != "max" {
		t.Errorf("ReasoningLevel = %q, want 'max'", spec.ReasoningLevel)
	}
}

// TestParseModelSpecAllValidLevels verifies every standard level is accepted.
func TestParseModelSpecAllValidLevels(t *testing.T) {
	for _, level := range ValidLevels {
		fullID := "openrouter/o3|" + level
		spec, err := ParseModelSpec(fullID)
		if err != nil {
			t.Fatalf("ParseModelSpec(%q) error: %v", fullID, err)
		}
		if spec.ReasoningLevel != level {
			t.Errorf("ParseModelSpec(%q) ReasoningLevel = %q, want %q", fullID, spec.ReasoningLevel, level)
		}
		if spec.ModelID != "openrouter/o3" {
			t.Errorf("ParseModelSpec(%q) ModelID = %q, want 'openrouter/o3'", fullID, spec.ModelID)
		}
	}
}

// TestParseModelSpecInvalidLevel verifies that an invalid suffix returns an error.
func TestParseModelSpecInvalidLevel(t *testing.T) {
	_, err := ParseModelSpec("openrouter/o3|ultra")
	if err == nil {
		t.Fatal("ParseModelSpec() expected error for invalid level, got nil")
	}
	if !strings.Contains(err.Error(), "invalid reasoning level") {
		t.Errorf("error = %q, want 'invalid reasoning level'", err.Error())
	}
}

// TestParseModelSpecEmptyModel verifies that an empty string returns an error.
func TestParseModelSpecEmptyModel(t *testing.T) {
	_, err := ParseModelSpec("")
	if err == nil {
		t.Fatal("ParseModelSpec('') expected error, got nil")
	}
}

// TestParseModelSpecEmptyBeforePipe verifies that |level with empty model ID returns error.
func TestParseModelSpecEmptyBeforePipe(t *testing.T) {
	_, err := ParseModelSpec("|high")
	if err == nil {
		t.Fatal("ParseModelSpec('|high') expected error, got nil")
	}
}

// TestParseModelSpecEmptyAfterPipe verifies that model| with empty level returns error.
func TestParseModelSpecEmptyAfterPipe(t *testing.T) {
	_, err := ParseModelSpec("openrouter/o3|")
	if err == nil {
		t.Fatal("ParseModelSpec('openrouter/o3|') expected error, got nil")
	}
}

// TestParseModelSpecBareModel verifies a model with only the model part.
func TestParseModelSpecBareModel(t *testing.T) {
	spec, err := ParseModelSpec("o3")
	if err != nil {
		t.Fatalf("ParseModelSpec('o3') error: %v", err)
	}
	if spec.ModelID != "o3" {
		t.Errorf("ModelID = %q, want 'o3'", spec.ModelID)
	}
	if spec.ReasoningLevel != "" {
		t.Errorf("ReasoningLevel = %q, want empty", spec.ReasoningLevel)
	}
}

// TestParseModelSpecDeeplyNested verifies deeply nested provider/model with suffix.
func TestParseModelSpecDeeplyNested(t *testing.T) {
	spec, err := ParseModelSpec("openrouter/deepseek/deepseek-r1|med")
	if err != nil {
		t.Fatalf("ParseModelSpec() error: %v", err)
	}
	if spec.ModelID != "openrouter/deepseek/deepseek-r1" {
		t.Errorf("ModelID = %q, want 'openrouter/deepseek/deepseek-r1'", spec.ModelID)
	}
	if spec.ReasoningLevel != "med" {
		t.Errorf("ReasoningLevel = %q, want 'med'", spec.ReasoningLevel)
	}
}

// TestHasReasoning verifies HasReasoning behavior.
func TestHasReasoning(t *testing.T) {
	spec := ModelSpec{ModelID: "test/o3", ReasoningLevel: "high"}
	if !spec.HasReasoning() {
		t.Error("HasReasoning() = false, want true for high")
	}
	spec = ModelSpec{ModelID: "test/o3", ReasoningLevel: ""}
	if spec.HasReasoning() {
		t.Error("HasReasoning() = true, want false for empty")
	}
}
