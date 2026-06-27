// skill_tools_test.go — tests for load_skill and unload_skill tools.
package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"blazeai/internal/platform"
	"blazeai/internal/skills"
)

// TestLoadSkillExecute verifies that a skill is added to the active list.
func TestLoadSkillExecute(t *testing.T) {
	active := skills.NewActiveList()
	tool := NewLoadSkillTool(active, nil)
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
	tool := NewLoadSkillTool(active, nil)
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
	tool := NewLoadSkillTool(active, nil)
	args := json.RawMessage(`{"name":""}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "error") {
		t.Errorf("Execute() = %q, want error message", result)
	}
}

// TestLoadSkillExecuteInvalidArgs verifies error on invalid JSON.
func TestLoadSkillExecuteInvalidArgs(t *testing.T) {
	active := skills.NewActiveList()
	tool := NewLoadSkillTool(active, nil)
	args := json.RawMessage(`{invalid}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "error") {
		t.Errorf("Execute() = %q, want error message", result)
	}
}

// TestLoadSkillName verifies the tool name.
func TestLoadSkillName(t *testing.T) {
	tool := NewLoadSkillTool(skills.NewActiveList(), nil)
	if tool.Name() != "load_skill" {
		t.Errorf("Name() = %q, want 'load_skill'", tool.Name())
	}
}

// TestLoadSkillParameters verifies parameters is valid JSON.
func TestLoadSkillParameters(t *testing.T) {
	tool := NewLoadSkillTool(skills.NewActiveList(), nil)
	params := tool.Parameters()
	if !json.Valid(params) {
		t.Error("Parameters() is not valid JSON")
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(params, &schema); err != nil {
		t.Fatalf("cannot parse schema: %v", err)
	}
	if _, ok := schema.Properties["purpose"]; ok {
		t.Fatal("load_skill schema should not include purpose")
	}
	if len(schema.Required) != 1 || schema.Required[0] != "name" {
		t.Fatalf("load_skill required = %v, want [name]", schema.Required)
	}
}

// TestUnloadSkillExecute verifies that a skill is removed from the active list.
func TestUnloadSkillExecute(t *testing.T) {
	active := skills.NewActiveList()
	active.Load("memory-manager")
	tool := NewUnloadSkillTool(active, nil)
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
	tool := NewUnloadSkillTool(active, nil)
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
	tool := NewUnloadSkillTool(active, nil)
	args := json.RawMessage(`{"name":"ghost"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "skill unloaded: ghost") {
		t.Errorf("Execute() = %q, want confirmation message", result)
	}
}

// TestUnloadSkillExecuteEmptyName verifies error on empty name.
func TestUnloadSkillExecuteEmptyName(t *testing.T) {
	active := skills.NewActiveList()
	tool := NewUnloadSkillTool(active, nil)
	args := json.RawMessage(`{"name":""}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "error") {
		t.Errorf("Execute() = %q, want error message", result)
	}
}

// TestUnloadSkillName verifies the tool name.
func TestUnloadSkillName(t *testing.T) {
	tool := NewUnloadSkillTool(skills.NewActiveList(), nil)
	if tool.Name() != "unload_skill" {
		t.Errorf("Name() = %q, want 'unload_skill'", tool.Name())
	}
}

// TestUnloadSkillParameters verifies parameters is valid JSON.
func TestUnloadSkillParameters(t *testing.T) {
	tool := NewUnloadSkillTool(skills.NewActiveList(), nil)
	params := tool.Parameters()
	if !json.Valid(params) {
		t.Error("Parameters() is not valid JSON")
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(params, &schema); err != nil {
		t.Fatalf("cannot parse schema: %v", err)
	}
	if _, ok := schema.Properties["purpose"]; ok {
		t.Fatal("unload_skill schema should not include purpose")
	}
	if len(schema.Required) != 1 || schema.Required[0] != "name" {
		t.Fatalf("unload_skill required = %v, want [name]", schema.Required)
	}
}

// TestUnloadSkillDescription verifies unload description stays compact.
func TestUnloadSkillDescription(t *testing.T) {
	tool := NewUnloadSkillTool(skills.NewActiveList(), nil)
	desc := tool.Description()
	if desc != "name → unload skill from active session" {
		t.Fatalf("Description() = %q, want compact unload description", desc)
	}
}

// TestRunSkillExecute verifies that run_skill executes a runnable shell skill.
func TestRunSkillExecute(t *testing.T) {
	skillDir := filepath.Join(t.TempDir(), "echo")
	tool := NewRunSkillTool(platform.Linux, func(name string) (string, *skills.Skill, error) {
		return "global/echo", &skills.Skill{
			Name:     name,
			Syntax:   "<text>",
			CodeLang: "shell",
			Code:     `printf '%s' "$BLAZE_SKILL_ARGS"`,
			Dir:      skillDir,
		}, nil
	}, func() string { return t.TempDir() })
	args := json.RawMessage(`{"name":"echo","arguments":"hello world"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "exit_code: 0") {
		t.Fatalf("Execute() = %q, want successful exit code", result)
	}
	if !strings.Contains(result, "stdout:\nhello world") {
		t.Fatalf("Execute() = %q, want stdout with raw arguments", result)
	}
}

// TestRunSkillExecuteRejectsUnsupportedLanguage verifies v1 only accepts shell code.
func TestRunSkillExecuteRejectsUnsupportedLanguage(t *testing.T) {
	tool := NewRunSkillTool(platform.Linux, func(name string) (string, *skills.Skill, error) {
		return "global/echo", &skills.Skill{Name: name, Syntax: "<text>", CodeLang: "python", Code: "print(1)"}, nil
	}, nil)
	args := json.RawMessage(`{"name":"echo","arguments":"hello"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "unsupported [CODE] language") {
		t.Fatalf("Execute() = %q, want unsupported language error", result)
	}
}

// TestRunSkillExecuteRejectsMalformedCode verifies parser failures are surfaced clearly.
func TestRunSkillExecuteRejectsMalformedCode(t *testing.T) {
	tool := NewRunSkillTool(platform.Linux, func(name string) (string, *skills.Skill, error) {
		return "global/echo", &skills.Skill{Name: name, Syntax: "<text>", CodeError: "[CODE] must start with a fenced code block"}, nil
	}, nil)
	args := json.RawMessage(`{"name":"echo","arguments":"hello"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "invalid [CODE]") {
		t.Fatalf("Execute() = %q, want invalid code error", result)
	}
}
