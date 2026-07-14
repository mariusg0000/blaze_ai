// agents_test.go — tests for Markdown agent discovery and validation.
// Covers syntax, strict metadata checks, duplicate names, and allowlist validation.
// Layer: agent definitions. Dependencies: internal/tools.
package agents

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blazeai/internal/tools"
)

type testTool struct{ name string }

func (t testTool) Name() string                                    { return t.name }
func (t testTool) Description() string                             { return t.name }
func (t testTool) Parameters() json.RawMessage                     { return json.RawMessage(`{"type":"object"}`) }
func (t testTool) Execute(context.Context, json.RawMessage) string { return "ok" }
func (t testTool) FormatArgs(json.RawMessage) string               { return "" }

func testRegistry() *tools.Registry {
	registry := tools.NewRegistry()
	registry.Register(testTool{name: "read_file"})
	registry.Register(testTool{name: "shell"})
	registry.Register(testTool{name: "run_agent"})
	return registry
}

func writeAgent(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadValidDefinitions(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "review.md", "---\nname: review\ndescription: Review files and summarize findings\nkind: one-shot\nmodel: openai/gpt-4.1\ntools:\n  - read_file\n---\nReview the input.\n")
	got, err := Load(dir, testRegistry())
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "review" || got[0].Description == "" || got[0].Kind != KindOneShot || got[0].Model != "openai/gpt-4.1" {
		t.Fatalf("unexpected definition: %+v", got)
	}
	if got[0].Instructions != "Review the input." {
		t.Fatalf("instructions = %q", got[0].Instructions)
	}
}

func TestLoadRejectsInvalidDefinitions(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"missing-name", "---\ndescription: Missing name\nkind: interactive\ntools:\n  - read_file\n---\n", "name is required"},
		{"bad-kind", "---\nname: x\ndescription: Invalid kind\nkind: invalid\ntools:\n  - read_file\n---\n", "unknown kind"},
		{"bad-model", "---\nname: x\ndescription: Invalid model\nkind: interactive\nmodel: broken\ntools:\n  - read_file\n---\n", "invalid model"},
		{"unknown-tool", "---\nname: x\ndescription: Unknown tool\nkind: interactive\ntools:\n  - missing\n---\n", "unknown tool"},
		{"missing-frontmatter", "# x\n", "opening front matter"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeAgent(t, dir, "agent.md", tt.body)
			_, err := Load(dir, testRegistry())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestLoadRejectsDuplicateNames(t *testing.T) {
	dir := t.TempDir()
	body := "---\nname: same\ndescription: Duplicate\nkind: interactive\ntools:\n  - read_file\n---\n"
	writeAgent(t, dir, "a.md", body)
	writeAgent(t, dir, "b.md", body)
	_, err := Load(dir, testRegistry())
	if err == nil || !strings.Contains(err.Error(), "duplicate agent name") {
		t.Fatalf("Load() error = %v, want duplicate error", err)
	}
}

func TestOneShotRules(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "done.md", "---\nname: done\ndescription: Completion test\nkind: one-shot\ntools:\n  - agent_done\n---\n")
	if _, err := Load(dir, testRegistry()); err != nil {
		t.Fatalf("agent_done should be permitted for one-shot: %v", err)
	}

	dir = t.TempDir()
	writeAgent(t, dir, "nested.md", "---\nname: nested\ndescription: Recursive test\nkind: one-shot\ntools:\n  - run_agent\n---\n")
	if _, err := Load(dir, testRegistry()); err == nil || !strings.Contains(err.Error(), "cannot use run_agent") {
		t.Fatalf("expected one-shot run_agent rejection, got %v", err)
	}
}
