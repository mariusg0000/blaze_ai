// skill_tools_test.go — tests for load_skill and unload_skill tools.
package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"blazeai/internal/skills"
)

// TestLoadSkillExecute verifies that a skill is added to the active list.
func TestLoadSkillExecute(t *testing.T) {
	active := skills.NewActiveList()
	tool := NewLoadSkillTool(active)
	args := json.RawMessage(`{"name":"memory-manager"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "skill loaded: memory-manager") {
		t.Errorf("Execute() = %q, want 'skill loaded: memory-manager'", result)
	}
	if !active.Has("memory-manager") {
		t.Error("active list does not contain 'memory-manager' after load")
	}
}

// TestLoadSkillExecuteWithMarkdownSuffix verifies .md names are normalized.
func TestLoadSkillExecuteWithMarkdownSuffix(t *testing.T) {
	active := skills.NewActiveList()
	tool := NewLoadSkillTool(active)
	args := json.RawMessage(`{"name":"memory-manager.md"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "skill loaded: memory-manager") {
		t.Errorf("Execute() = %q, want 'skill loaded: memory-manager'", result)
	}
	if !active.Has("memory-manager") {
		t.Error("active list does not contain normalized 'memory-manager' after load")
	}
	if active.Has("memory-manager.md") {
		t.Error("active list should not contain raw 'memory-manager.md' after load")
	}
}

// TestLoadSkillExecuteEmptyName verifies error on empty name.
func TestLoadSkillExecuteEmptyName(t *testing.T) {
	active := skills.NewActiveList()
	tool := NewLoadSkillTool(active)
	args := json.RawMessage(`{"name":""}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "error") {
		t.Errorf("Execute() = %q, want error message", result)
	}
}

// TestLoadSkillExecuteInvalidArgs verifies error on invalid JSON.
func TestLoadSkillExecuteInvalidArgs(t *testing.T) {
	active := skills.NewActiveList()
	tool := NewLoadSkillTool(active)
	args := json.RawMessage(`{invalid}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "error") {
		t.Errorf("Execute() = %q, want error message", result)
	}
}

// TestLoadSkillName verifies the tool name.
func TestLoadSkillName(t *testing.T) {
	tool := NewLoadSkillTool(skills.NewActiveList())
	if tool.Name() != "load_skill" {
		t.Errorf("Name() = %q, want 'load_skill'", tool.Name())
	}
}

// TestLoadSkillParameters verifies parameters is valid JSON.
func TestLoadSkillParameters(t *testing.T) {
	tool := NewLoadSkillTool(skills.NewActiveList())
	params := tool.Parameters()
	if !json.Valid(params) {
		t.Error("Parameters() is not valid JSON")
	}
	schemaIncludesRequiredPurpose(t, params)
}

// TestUnloadSkillExecute verifies that a skill is removed from the active list.
func TestUnloadSkillExecute(t *testing.T) {
	active := skills.NewActiveList()
	active.Load("memory-manager")
	tool := NewUnloadSkillTool(active)
	args := json.RawMessage(`{"name":"memory-manager"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "skill unloaded: memory-manager") {
		t.Errorf("Execute() = %q, want 'skill unloaded: memory-manager'", result)
	}
	if active.Has("memory-manager") {
		t.Error("active list still contains 'memory-manager' after unload")
	}
}

// TestUnloadSkillExecuteWithMarkdownSuffix verifies .md names are normalized on unload.
func TestUnloadSkillExecuteWithMarkdownSuffix(t *testing.T) {
	active := skills.NewActiveList()
	active.Load("memory-manager")
	tool := NewUnloadSkillTool(active)
	args := json.RawMessage(`{"name":"memory-manager.md"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "skill unloaded: memory-manager") {
		t.Errorf("Execute() = %q, want 'skill unloaded: memory-manager'", result)
	}
	if active.Has("memory-manager") {
		t.Error("active list still contains normalized 'memory-manager' after unload")
	}
}

// TestUnloadSkillExecuteNotPresent verifies unloading a non-active skill still succeeds.
func TestUnloadSkillExecuteNotPresent(t *testing.T) {
	active := skills.NewActiveList()
	tool := NewUnloadSkillTool(active)
	args := json.RawMessage(`{"name":"ghost"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "skill unloaded: ghost") {
		t.Errorf("Execute() = %q, want confirmation message", result)
	}
}

// TestUnloadSkillExecuteEmptyName verifies error on empty name.
func TestUnloadSkillExecuteEmptyName(t *testing.T) {
	active := skills.NewActiveList()
	tool := NewUnloadSkillTool(active)
	args := json.RawMessage(`{"name":""}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "error") {
		t.Errorf("Execute() = %q, want error message", result)
	}
}

// TestUnloadSkillName verifies the tool name.
func TestUnloadSkillName(t *testing.T) {
	tool := NewUnloadSkillTool(skills.NewActiveList())
	if tool.Name() != "unload_skill" {
		t.Errorf("Name() = %q, want 'unload_skill'", tool.Name())
	}
}

// TestUnloadSkillParameters verifies parameters is valid JSON.
func TestUnloadSkillParameters(t *testing.T) {
	tool := NewUnloadSkillTool(skills.NewActiveList())
	params := tool.Parameters()
	if !json.Valid(params) {
		t.Error("Parameters() is not valid JSON")
	}
	schemaIncludesRequiredPurpose(t, params)
}

// TestUnloadSkillDescription verifies unload description stays minimal.
func TestUnloadSkillDescription(t *testing.T) {
	tool := NewUnloadSkillTool(skills.NewActiveList())
	desc := tool.Description()
	if desc != "Unload a skill by name." {
		t.Fatalf("Description() = %q, want minimal unload description", desc)
	}
}
