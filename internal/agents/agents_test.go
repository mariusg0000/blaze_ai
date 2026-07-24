// agents_test.go — tests for Markdown agent discovery and validation.
// Covers syntax, strict metadata checks, duplicate names, allowlist validation,
// type-specific rules, and executor reference resolution.
// Layer: agent definitions. Dependencies: internal/tools.
package agents

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestLoadValidExecutorDefinition(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "review.md", "---\nname: review\ndescription: Review files and summarize findings\ntype: executor\nmodel: openai/gpt-4.1\ntools:\n  - read_file\n---\nReview the input.\n")
	got, err := Load(dir, testRegistry())
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "review" || got[0].Description == "" || got[0].Type != TypeExecutor || got[0].Model != "openai/gpt-4.1" {
		t.Fatalf("unexpected definition: %+v", got)
	}
	if got[0].Instructions != "Review the input." {
		t.Fatalf("instructions = %q", got[0].Instructions)
	}
}

func TestLoadValidInteractiveDefinition(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "review.md", "---\nname: review\ndescription: Review files\ntype: executor\ntools:\n  - read_file\n---\n")
	writeAgent(t, dir, "planner.md", "---\nname: planner\ndescription: Plan and delegate tasks\ntype: interactive\nmodel: openai/gpt-4.1\ntools:\n  - read_file\n  - shell\nagents:\n  - review\ndirective: Be concise and plan only.\n---\nPlan the work.\n")
	got, err := Load(dir, testRegistry())
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(got))
	}
	planner := got[0]
	if planner.Name != "planner" || planner.Type != TypeInteractive || planner.Model != "openai/gpt-4.1" {
		t.Fatalf("unexpected planner definition: %+v", planner)
	}
	if planner.Directive != "Be concise and plan only." {
		t.Fatalf("directive = %q", planner.Directive)
	}
	if len(planner.ExecutorNames) != 1 || planner.ExecutorNames[0] != "review" {
		t.Fatalf("executor names = %v", planner.ExecutorNames)
	}
	if planner.Instructions != "Plan the work." {
		t.Fatalf("instructions = %q", planner.Instructions)
	}
}

func TestLoadValidExecutorNoModel(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "worker.md", "---\name: worker\ndescription: Execute tasks\ntype: executor\ntools:\n  - shell\n---\nDo work.\n")
	// Note: The front matter above has a typo (backslash instead of dash).
	// Fix it:
	writeAgent(t, dir, "worker.md", "---\nname: worker\ndescription: Execute tasks\ntype: executor\ntools:\n  - shell\n---\nDo work.\n")
	got, err := Load(dir, testRegistry())
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(got) != 1 || got[0].Type != TypeExecutor || got[0].Model != "" {
		t.Fatalf("expected executor with empty model, got %+v", got)
	}
}

func TestLoadRejectsInvalidDefinitions(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"missing-name", "---\ndescription: Missing name\ntype: executor\ntools:\n  - read_file\n---\n", "name is required"},
		{"missing-type", "---\nname: x\ndescription: Missing type\ntools:\n  - read_file\n---\n", "type is required"},
		{"bad-type", "---\nname: x\ndescription: Invalid type\ntype: invalid\ntools:\n  - read_file\n---\n", "unsupported type"},
		{"bad-model", "---\nname: x\ndescription: Invalid model\ntype: executor\nmodel: broken\ntools:\n  - read_file\n---\n", "invalid model"},
		{"unknown-tool", "---\nname: x\ndescription: Unknown tool\ntype: executor\ntools:\n  - missing\n---\n", "unknown tool"},
		{"missing-frontmatter", "# x\n", "opening front matter"},
		{"legacy-kind", "---\nname: x\ndescription: Legacy kind\nkind: one-shot\ntools:\n  - read_file\n---\n", "unknown metadata key"},
		{"interactive-missing-model", "---\nname: x\ndescription: Interactive without model\ntype: interactive\ntools:\n  - read_file\n---\n", "interactive agent requires a model"},
		{"executor-with-directive", "---\nname: x\ndescription: Executor with directive\ntype: executor\ndirective: do this\ntools:\n  - read_file\n---\n", "executor agent must not have a directive"},
		{"executor-with-agents", "---\nname: x\ndescription: Executor with agents\ntype: executor\nagents:\n  - other\ntools:\n  - read_file\n---\n", "executor agent must not list agents"},
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
	body := "---\nname: same\ndescription: Duplicate\ntype: executor\ntools:\n  - read_file\n---\n"
	writeAgent(t, dir, "a.md", body)
	writeAgent(t, dir, "b.md", body)
	_, err := Load(dir, testRegistry())
	if err == nil || !strings.Contains(err.Error(), "duplicate agent name") {
		t.Fatalf("Load() error = %v, want duplicate error", err)
	}
}

func TestLoadRejectsDuplicateTools(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "dup.md", "---\nname: dup\ndescription: Duplicate tools\ntype: executor\ntools:\n  - read_file\n  - read_file\n---\n")
	_, err := Load(dir, testRegistry())
	if err == nil || !strings.Contains(err.Error(), "duplicate tool") {
		t.Fatalf("expected duplicate tool error, got %v", err)
	}
}

func TestExecutorRunAgentForbidden(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "nested.md", "---\nname: nested\ndescription: Recursive test\ntype: executor\ntools:\n  - run_agent\n---\n")
	if _, err := Load(dir, testRegistry()); err == nil || !strings.Contains(err.Error(), "executor agents cannot use run_agent") {
		t.Fatalf("expected executor run_agent rejection, got %v", err)
	}
}

func TestInteractiveRunAgentPermitted(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "interactive.md", "---\nname: interactive\ndescription: Interactive with run_agent\ntype: interactive\nmodel: openai/gpt-4.1\ntools:\n  - read_file\n  - run_agent\n---\n")
	if _, err := Load(dir, testRegistry()); err != nil {
		t.Fatalf("interactive run_agent should be permitted: %v", err)
	}
}

func TestLoadRejectsUnknownExecutorReference(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "planner.md", "---\nname: planner\ndescription: References unknown executor\ntype: interactive\nmodel: openai/gpt-4.1\ntools:\n  - read_file\nagents:\n  - ghost\n---\n")
	_, err := Load(dir, testRegistry())
	if err == nil || !strings.Contains(err.Error(), "unknown executor reference") {
		t.Fatalf("expected unknown executor reference error, got %v", err)
	}
}

func TestLoadRejectsDuplicateExecutorReferences(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "exec.md", "---\nname: exec\ndescription: Executor\ntype: executor\ntools:\n  - read_file\n---\n")
	writeAgent(t, dir, "planner.md", "---\nname: planner\ndescription: Duplicate exec ref\ntype: interactive\nmodel: openai/gpt-4.1\ntools:\n  - read_file\nagents:\n  - exec\n  - exec\n---\n")
	_, err := Load(dir, testRegistry())
	if err == nil || !strings.Contains(err.Error(), "duplicate executor reference") {
		t.Fatalf("expected duplicate executor reference error, got %v", err)
	}
}

func TestLoadRejectsNonExecutorReference(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "other.md", "---\nname: other\ndescription: Interactive referencing interactive\ntype: interactive\nmodel: openai/gpt-4.1\ntools:\n  - read_file\n---\n")
	writeAgent(t, dir, "planner.md", "---\nname: planner\ndescription: References interactive as executor\ntype: interactive\nmodel: openai/gpt-4.1\ntools:\n  - read_file\nagents:\n  - other\n---\n")
	_, err := Load(dir, testRegistry())
	if err == nil || !strings.Contains(err.Error(), "is not an executor") {
		t.Fatalf("expected non-executor reference error, got %v", err)
	}
}

func TestLoadTimeoutParsed(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "timed.md", "---\nname: timed\ndescription: Agent with timeout\ntype: executor\ntimeout: 15m\ntools:\n  - read_file\n---\nDo work.\n")
	got, err := Load(dir, testRegistry())
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(got) != 1 || got[0].Timeout != 15*time.Minute {
		t.Fatalf("expected timeout 15m, got %+v", got[0].Timeout)
	}
}

func TestLoadTimeoutDefaultZero(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "notime.md", "---\nname: notime\ndescription: Agent without timeout\ntype: executor\ntools:\n  - read_file\n---\nDo work.\n")
	got, err := Load(dir, testRegistry())
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(got) != 1 || got[0].Timeout != 0 {
		t.Fatalf("expected timeout 0 (default), got %+v", got[0].Timeout)
	}
}

func TestLoadRejectsInvalidTimeout(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "bad.md", "---\nname: bad\ndescription: Bad timeout\ntype: executor\ntimeout: notaduration\ntools:\n  - read_file\n---\n")
	_, err := Load(dir, testRegistry())
	if err == nil || !strings.Contains(err.Error(), "invalid timeout") {
		t.Fatalf("expected invalid timeout error, got %v", err)
	}
}

func TestLoadRejectsLegacyKind(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "old.md", "---\nname: old\ndescription: Uses legacy kind\nkind: one-shot\ntools:\n  - read_file\n---\n")
	_, err := Load(dir, testRegistry())
	if err == nil || !strings.Contains(err.Error(), "unknown metadata key") {
		t.Fatalf("expected legacy kind rejection, got %v", err)
	}
}

func TestLoadInteractiveWithDirectivesAndBody(t *testing.T) {
	dir := t.TempDir()
	body := "---\nname: lead\ndescription: Lead agent\ntype: interactive\nmodel: openai/gpt-4.1\ntimeout: 10m\ntools:\n  - read_file\n  - shell\nagents:\n  - exec1\ndirective: Be thorough.\n---\nLead the work.\n"
	writeAgent(t, dir, "lead.md", body)
	writeAgent(t, dir, "exec1.md", "---\nname: exec1\ndescription: Executor 1\ntype: executor\ntools:\n  - read_file\n---\n")
	got, err := Load(dir, testRegistry())
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(got))
	}
	// Files are sorted alphabetically: exec1.md comes before lead.md.
	lead := got[1]
	if lead.Name != "lead" || lead.Type != TypeInteractive || lead.Directive != "Be thorough." {
		t.Fatalf("unexpected lead definition: %+v", lead)
	}
	if lead.Timeout != 10*time.Minute {
		t.Fatalf("expected timeout 10m, got %v", lead.Timeout)
	}
	if len(lead.ExecutorNames) != 1 || lead.ExecutorNames[0] != "exec1" {
		t.Fatalf("unexpected executor names: %v", lead.ExecutorNames)
	}
	if lead.Instructions != "Lead the work." {
		t.Fatalf("instructions = %q", lead.Instructions)
	}
}

func TestLoadRejectsEmptyToolList(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "notools.md", "---\nname: notools\ndescription: No tools\ntype: executor\n---\n")
	_, err := Load(dir, testRegistry())
	if err == nil || !strings.Contains(err.Error(), "tools allowlist is required") {
		t.Fatalf("expected empty tools error, got %v", err)
	}
}
