// migration_test.go — tests for legacy agent and mode migration.
// Covers kind→type replacement, mode-to-interactive conversion, safety checks,
// and no-overwrite guarantees.
// Layer: agent definitions. Dependencies: internal/config.
package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blazeai/internal/config"
)

func TestMigrateLegacyDefinitions(t *testing.T) {
	dir := t.TempDir()
	// File with legacy kind: one-shot.
	legacy := "---\nname: worker\ndescription: Legacy worker\nkind: one-shot\nmodel: openai/gpt-4.1\ntools:\n  - read_file\n---\nDo work.\n"
	writeAgent(t, dir, "worker.md", legacy)
	// File without any kind line (already new-style or no kind).
	clean := "---\nname: reviewer\ndescription: No kind line\ntype: executor\ntools:\n  - read_file\n---\nReview.\n"
	writeAgent(t, dir, "reviewer.md", clean)

	if err := MigrateLegacyDefinitions(dir); err != nil {
		t.Fatalf("MigrateLegacyDefinitions() error: %v", err)
	}

	// Verify worker.md was migrated.
	data, err := os.ReadFile(filepath.Join(dir, "worker.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Contains(content, "kind: one-shot") {
		t.Fatalf("worker.md still contains kind: one-shot")
	}
	if !strings.Contains(content, "type: executor") {
		t.Fatalf("worker.md does not contain type: executor")
	}
	if !strings.Contains(content, "name: worker") {
		t.Fatalf("worker.md lost its name")
	}

	// Verify reviewer.md was not modified.
	reviewData, err := os.ReadFile(filepath.Join(dir, "reviewer.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(reviewData) != clean {
		t.Fatalf("reviewer.md was modified unexpectedly")
	}
}

func TestMigrateLegacyDefinitionsRejectsUnknownKind(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "bad.md", "---\nname: bad\ndescription: Bad kind\nkind: unknown-value\ntools:\n  - read_file\n---\n")
	err := MigrateLegacyDefinitions(dir)
	if err == nil || !strings.Contains(err.Error(), "unsupported legacy kind") {
		t.Fatalf("expected unsupported legacy kind error, got %v", err)
	}
}

func TestMigrateLegacyModesCreatesInteractiveDefinitions(t *testing.T) {
	dir := t.TempDir()
	legacy := &config.ModesConfig{
		Modes: []config.Mode{
			{
				Name:        "planning",
				Model:       "openai/gpt-4.1",
				Directive:   "Analyze and\nplan only.",
				DeniedTools: []string{"shell"},
				Agents:      []string{"worker"},
			},
			{
				Name:  "default",
				Model: "openai/gpt-4o",
			},
		},
		LastMode: "planning",
	}
	baseTools := []string{"read_file", "shell", "agent_done", "run_agent"}

	if err := MigrateLegacyModes(dir, legacy, baseTools); err != nil {
		t.Fatalf("MigrateLegacyModes() error: %v", err)
	}

	// Verify planning.md.
	data, err := os.ReadFile(filepath.Join(dir, "planning.md"))
	if err != nil {
		t.Fatalf("planning.md not created: %v", err)
	}
	content := string(data)

	// Check type.
	if !strings.Contains(content, "type: interactive") {
		t.Fatalf("planning.md missing type: interactive")
	}
	// Check name and description.
	if !strings.Contains(content, "name: planning") {
		t.Fatalf("planning.md missing name")
	}
	if !strings.Contains(content, "description: Migrated from mode planning") {
		t.Fatalf("planning.md wrong description: %s", content)
	}
	// Check model.
	if !strings.Contains(content, "model: openai/gpt-4.1") {
		t.Fatalf("planning.md missing model")
	}
	// Check tools: read_file only (shell is denied, agent_done and run_agent excluded).
	if strings.Contains(content, "shell") {
		t.Fatalf("planning.md should not contain denied tool shell")
	}
	if strings.Contains(content, "agent_done") {
		t.Fatalf("planning.md should not contain agent_done")
	}
	if strings.Contains(content, "run_agent") {
		t.Fatalf("planning.md should not contain run_agent")
	}
	if !strings.Contains(content, "  - read_file") {
		t.Fatalf("planning.md should contain read_file tool")
	}
	// Check agents.
	if !strings.Contains(content, "  - worker") {
		t.Fatalf("planning.md missing agents entry worker")
	}
	// Check directive with newline converted to space.
	if strings.Contains(content, "Analyze and\nplan") {
		t.Fatalf("planning.md directive should have newlines converted to spaces")
	}
	if !strings.Contains(content, "directive: Analyze and plan only.") {
		t.Fatalf("planning.md directive wrong: %s", content)
	}
	// Check empty body (only after closing ---).
	parts := strings.SplitN(content, "---\n", 3)
	if len(parts) == 3 && strings.TrimSpace(parts[2]) != "" {
		t.Fatalf("planning.md should have empty body, got: %q", parts[2])
	}

	// Verify default.md.
	defaultData, err := os.ReadFile(filepath.Join(dir, "default.md"))
	if err != nil {
		t.Fatalf("default.md not created: %v", err)
	}
	defaultContent := string(defaultData)
	if !strings.Contains(defaultContent, "type: interactive") {
		t.Fatalf("default.md missing type: interactive")
	}
	if !strings.Contains(defaultContent, "model: openai/gpt-4o") {
		t.Fatalf("default.md missing model")
	}
	if !strings.Contains(defaultContent, "description: Migrated from mode default") {
		t.Fatalf("default.md wrong description")
	}
}

func TestMigrateLegacyModesDoesNotOverwriteExistingFiles(t *testing.T) {
	dir := t.TempDir()
	// Pre-existing file for the "planning" mode.
	existing := "---\nname: planning\ndescription: Existing custom planner\ntype: interactive\nmodel: openai/gpt-4o\ntools:\n  - read_file\n---\nCustom body.\n"
	writeAgent(t, dir, "planning.md", existing)

	legacy := &config.ModesConfig{
		Modes: []config.Mode{
			{Name: "planning", Model: "openai/gpt-4.1"},
		},
		LastMode: "planning",
	}
	baseTools := []string{"read_file"}

	if err := MigrateLegacyModes(dir, legacy, baseTools); err != nil {
		t.Fatalf("MigrateLegacyModes() error: %v", err)
	}

	// Verify the file was NOT overwritten.
	data, err := os.ReadFile(filepath.Join(dir, "planning.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != existing {
		t.Fatalf("existing file was overwritten")
	}
}

func TestMigrateLegacyModesRejectsUnsafeNames(t *testing.T) {
	tests := []struct {
		name string
		mode config.Mode
	}{
		{"dot", config.Mode{Name: ".", Model: "openai/gpt-4o"}},
		{"dotdot", config.Mode{Name: "..", Model: "openai/gpt-4o"}},
		{"slash", config.Mode{Name: "bad/name", Model: "openai/gpt-4o"}},
		{"backslash", config.Mode{Name: "bad\\name", Model: "openai/gpt-4o"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			legacy := &config.ModesConfig{Modes: []config.Mode{tt.mode}}
			err := MigrateLegacyModes(dir, legacy, []string{"read_file"})
			if err == nil || !strings.Contains(err.Error(), "unsafe mode name") {
				t.Fatalf("expected unsafe mode name error, got %v", err)
			}
		})
	}
}

func TestMigrateLegacyModesNilLegacy(t *testing.T) {
	dir := t.TempDir()
	if err := MigrateLegacyModes(dir, nil, []string{"read_file"}); err != nil {
		t.Fatalf("nil legacy should be no-op, got error: %v", err)
	}
}
