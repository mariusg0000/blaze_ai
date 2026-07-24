// agent_capabilities_test.go — tests for interactive-agent tool and executor allowlist.
// Purpose: Verify tool filtering, executor resolution, run_agent gating, and allowlist isolation.
// Layer: runtime capabilities tests. Dependencies: internal/agents, internal/tools.
package runtime

import (
	"context"
	"encoding/json"
	"testing"

	"blazeai/internal/agents"
	"blazeai/internal/prompt"
	"blazeai/internal/tools"
)

func capTestRegistry() *tools.Registry {
	registry := tools.NewRegistry()
	registry.Register(testCapTool{name: "shell"})
	registry.Register(testCapTool{name: "read_file"})
	registry.Register(testCapTool{name: "write_file"})
	registry.Register(testCapTool{name: "run_agent"})
	return registry
}

type testCapTool struct{ name string }

func (t testCapTool) Name() string                                        { return t.name }
func (t testCapTool) Description() string                                 { return t.name }
func (t testCapTool) Parameters() json.RawMessage                         { return json.RawMessage(`{"type":"object"}`) }
func (t testCapTool) Execute(_ context.Context, _ json.RawMessage) string { return "ok" }
func (t testCapTool) FormatArgs(_ json.RawMessage) string                 { return "" }

// TestRefreshAgentCapabilitiesFiltersTools verifies that refreshAgentCapabilities
// filters BaseTools to only the CurrentAgent's ToolNames.
func TestRefreshAgentCapabilitiesFiltersTools(t *testing.T) {
	registry := capTestRegistry()
	agent := &Agent{
		BaseTools: registry.Clone(),
		CurrentAgent: &agents.Definition{
			Name:      "test",
			Type:      agents.TypeInteractive,
			ToolNames: []string{"shell", "read_file"},
		},
		Definitions: nil,
		Builder:     &prompt.Builder{},
	}

	if err := agent.refreshAgentCapabilities(); err != nil {
		t.Fatalf("refreshAgentCapabilities() error: %v", err)
	}

	if agent.Tools.Get("shell") == nil {
		t.Error("shell should be present in filtered tools")
	}
	if agent.Tools.Get("read_file") == nil {
		t.Error("read_file should be present in filtered tools")
	}
	if agent.Tools.Get("write_file") != nil {
		t.Error("write_file should not be present in filtered tools")
	}
	if agent.Tools.Get("run_agent") != nil {
		t.Error("run_agent should not be present when no executors are allowed")
	}
}

// TestRefreshAgentCapabilitiesFiltersExecutors verifies that refreshAgentCapabilities
// resolves only matching TypeExecutor definitions for Builder.Agents.
func TestRefreshAgentCapabilitiesFiltersExecutors(t *testing.T) {
	registry := capTestRegistry()
	defs := []agents.Definition{
		{Name: "executor-a", Type: agents.TypeExecutor, ToolNames: []string{"shell"}},
		{Name: "executor-b", Type: agents.TypeExecutor, ToolNames: []string{"shell"}},
		{Name: "other", Type: agents.TypeInteractive, Model: "test/m", ToolNames: []string{"shell"}},
	}
	agent := &Agent{
		BaseTools:   registry.Clone(),
		Definitions: defs,
		CurrentAgent: &agents.Definition{
			Name:          "test",
			Type:          agents.TypeInteractive,
			ToolNames:     []string{"shell"},
			ExecutorNames: []string{"executor-a"},
		},
		Builder: &prompt.Builder{},
	}

	if err := agent.refreshAgentCapabilities(); err != nil {
		t.Fatalf("refreshAgentCapabilities() error: %v", err)
	}

	if len(agent.Builder.Agents) != 1 {
		t.Fatalf("expected 1 agent in Builder.Agents, got %d", len(agent.Builder.Agents))
	}
	if agent.Builder.Agents[0].Name != "executor-a" {
		t.Errorf("Builder.Agents[0].Name = %q, want 'executor-a'", agent.Builder.Agents[0].Name)
	}
}

// TestRefreshAgentCapabilitiesAddsRunAgentForAllowedExecutor verifies that run_agent
// is automatically included in the filtered tool registry when at least one executor is allowed.
func TestRefreshAgentCapabilitiesAddsRunAgentForAllowedExecutor(t *testing.T) {
	registry := capTestRegistry()
	defs := []agents.Definition{
		{Name: "worker", Type: agents.TypeExecutor, ToolNames: []string{"shell"}},
	}
	agent := &Agent{
		BaseTools:   registry.Clone(),
		Definitions: defs,
		CurrentAgent: &agents.Definition{
			Name:          "test",
			Type:          agents.TypeInteractive,
			ToolNames:     []string{"shell"},
			ExecutorNames: []string{"worker"},
		},
		Builder: &prompt.Builder{},
	}

	if err := agent.refreshAgentCapabilities(); err != nil {
		t.Fatalf("refreshAgentCapabilities() error: %v", err)
	}

	if agent.Tools.Get("run_agent") == nil {
		t.Error("run_agent should be present when an executor is allowed")
	}
	if agent.Tools.Get("shell") == nil {
		t.Error("shell should be present in filtered tools")
	}
}

// TestRefreshAgentCapabilitiesRejectsUnknownTool verifies an error is returned
// when CurrentAgent references a tool name not in the base registry.
func TestRefreshAgentCapabilitiesRejectsUnknownTool(t *testing.T) {
	registry := capTestRegistry()
	agent := &Agent{
		BaseTools:   registry.Clone(),
		Definitions: []agents.Definition{},
		CurrentAgent: &agents.Definition{
			Name:      "test",
			Type:      agents.TypeInteractive,
			ToolNames: []string{"nonexistent_tool"},
		},
		Builder: &prompt.Builder{},
	}

	err := agent.refreshAgentCapabilities()
	if err == nil {
		t.Fatal("refreshAgentCapabilities() expected error for unknown tool, got nil")
	}
}

// TestInteractiveAllowsExecutor verifies the executor allowlist check.
func TestInteractiveAllowsExecutor(t *testing.T) {
	agent := &Agent{
		CurrentAgent: &agents.Definition{
			Name:          "planner",
			Type:          agents.TypeInteractive,
			ExecutorNames: []string{"coder", "reviewer"},
		},
	}

	if !agent.interactiveAllowsExecutor("coder") {
		t.Error("interactiveAllowsExecutor(coder) = false, want true")
	}
	if !agent.interactiveAllowsExecutor("reviewer") {
		t.Error("interactiveAllowsExecutor(reviewer) = false, want true")
	}
	if agent.interactiveAllowsExecutor("unknown") {
		t.Error("interactiveAllowsExecutor(unknown) = true, want false")
	}

	// Nil CurrentAgent always returns false.
	agent.CurrentAgent = nil
	if agent.interactiveAllowsExecutor("coder") {
		t.Error("interactiveAllowsExecutor(coder) with nil agent = true, want false")
	}
}
