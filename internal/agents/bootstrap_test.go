// bootstrap_test.go — tests for EnsureDefaultInteractive provisioning logic.
// Covers creation, overwrite prevention, model validation, and write error propagation.
// Layer: agent definitions. Dependencies: internal/config (model validation).
package agents

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestEnsureDefaultInteractiveCreatesDefinition(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, "agents")

	// Unsorted tool and executor slices.
	tools := []string{"write_file", "agent_done", "shell", "run_agent", "read_file"}
	executors := []string{"coder", "reviewer"}

	created, err := EnsureDefaultInteractive(agentsDir, "test/test-model", tools, executors)
	if err != nil {
		t.Fatalf("EnsureDefaultInteractive() error: %v", err)
	}
	if !created {
		t.Fatal("expected created = true")
	}

	// Parse the generated definition through the standard parser (ParseFile avoids
	// cross-definition executor reference validation, which is not under test here).
	target := filepath.Join(agentsDir, "default.md")
	def, err := ParseFile(target)
	if err != nil {
		t.Fatalf("ParseFile() error: %v", err)
	}

	// Verify name, type, description, and model.
	if def.Name != "default" {
		t.Errorf("Name = %q, want 'default'", def.Name)
	}
	if def.Type != TypeInteractive {
		t.Errorf("Type = %q, want 'interactive'", def.Type)
	}
	if def.Description != "General-purpose interactive agent" {
		t.Errorf("Description = %q, want 'General-purpose interactive agent'", def.Description)
	}
	if def.Model != "test/test-model" {
		t.Errorf("Model = %q, want 'test/test-model'", def.Model)
	}

	// Verify tools are sorted and control tools are filtered out.
	expectedTools := []string{"read_file", "shell", "write_file"}
	if len(def.ToolNames) != len(expectedTools) {
		t.Fatalf("ToolNames = %v, want %v", def.ToolNames, expectedTools)
	}
	for i, name := range def.ToolNames {
		if name != expectedTools[i] {
			t.Errorf("ToolNames[%d] = %q, want %q", i, name, expectedTools[i])
		}
	}

	// Verify executor names are sorted.
	expectedExecutors := []string{"coder", "reviewer"}
	if len(def.ExecutorNames) != len(expectedExecutors) {
		t.Fatalf("ExecutorNames = %v, want %v", def.ExecutorNames, expectedExecutors)
	}
	sortedExecs := make([]string, len(def.ExecutorNames))
	copy(sortedExecs, def.ExecutorNames)
	sort.Strings(sortedExecs)
	for i, name := range sortedExecs {
		if name != expectedExecutors[i] {
			t.Errorf("ExecutorNames[%d] = %q, want %q", i, name, expectedExecutors[i])
		}
	}

	// Verify directive is omitted.
	if def.Directive != "" {
		t.Errorf("Directive = %q, want empty", def.Directive)
	}

	// Verify instructions (body) are non-empty and expected.
	if def.Instructions == "" {
		t.Error("Instructions is empty")
	}
	if !strings.Contains(def.Instructions, "primary interactive BlazeAI agent") {
		t.Errorf("Instructions = %q, want body mentioning primary interactive agent", def.Instructions)
	}

	// Verify file mode is 0600.
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("os.Stat(%s) error: %v", target, err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("file mode = %o, want 0600", info.Mode().Perm())
	}
}

func TestEnsureDefaultInteractiveDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}

	customContent := "---\nname: custom\n---\nCustom body.\n"
	target := filepath.Join(agentsDir, "default.md")
	if err := os.WriteFile(target, []byte(customContent), 0600); err != nil {
		t.Fatal(err)
	}

	created, err := EnsureDefaultInteractive(agentsDir, "test/test-model", []string{"shell"}, nil)
	if err != nil {
		t.Fatalf("EnsureDefaultInteractive() error: %v", err)
	}
	if created {
		t.Fatal("expected created = false when file exists")
	}

	// Verify byte-for-byte preservation of the original file.
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(data) != customContent {
		t.Errorf("file was overwritten: got %q, want %q", string(data), customContent)
	}
}

func TestEnsureDefaultInteractiveRejectsInvalidModel(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, "agents")

	created, err := EnsureDefaultInteractive(agentsDir, "bad-model-no-slash", []string{"shell"}, nil)
	if err == nil {
		t.Fatal("expected error for invalid model, got nil")
	}
	if created {
		t.Fatal("expected created = false on validation error")
	}

	// Verify no target file was created.
	target := filepath.Join(agentsDir, "default.md")
	if _, statErr := os.Stat(target); statErr == nil {
		t.Fatal("default.md should not exist after validation error")
	}
}

func TestEnsureDefaultInteractivePropagatesUnsafeWriteError(t *testing.T) {
	dir := t.TempDir()
	// Use a regular file as agentsDir to trigger a write error.
	agentsDir := filepath.Join(dir, "blocked")
	if err := os.WriteFile(agentsDir, []byte("not a dir"), 0600); err != nil {
		t.Fatal(err)
	}

	created, err := EnsureDefaultInteractive(agentsDir, "test/test-model", []string{"shell"}, nil)
	if err == nil {
		t.Fatal("expected error when agentsDir is a file, got nil")
	}
	if created {
		t.Fatal("expected created = false on write error")
	}
}
