// skill_tools_test.go — tests for loading rendered skill bodies.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestLoadSkillExecute(t *testing.T) {
	var got string
	tool := NewLoadSkillTool(func(name string) (string, string, error) {
		got = name
		return "project/demo", "body", nil
	})
	result := tool.Execute(context.Background(), json.RawMessage(`{"name":"demo.md"}`))
	if got != "demo" || result != "Skill loaded: project/demo\n\nbody" {
		t.Fatalf("result=%q name=%q", result, got)
	}
}

func TestLoadSkillErrors(t *testing.T) {
	tool := NewLoadSkillTool(func(string) (string, string, error) { return "", "", errors.New("missing") })
	for _, args := range []string{`{invalid}`, `{"name":""}`} {
		if result := tool.Execute(context.Background(), json.RawMessage(args)); len(result) < 7 || result[:7] != "error: " {
			t.Errorf("Execute(%s) = %q", args, result)
		}
	}
	if result := NewLoadSkillTool(nil).Execute(context.Background(), json.RawMessage(`{"name":"x"}`)); result != "error: skill loader is not configured" {
		t.Errorf("nil loader result = %q", result)
	}
}

func TestLoadSkillMetadata(t *testing.T) {
	tool := NewLoadSkillTool(func(string) (string, string, error) { return "x", "y", nil })
	if tool.Name() != "load_skill" || tool.Description() == "" || !json.Valid(tool.Parameters()) {
		t.Fatal("invalid load_skill metadata")
	}
}

func TestLoadSkillExecuteDisplayScopes(t *testing.T) {
	for _, tc := range []struct {
		resolved string
		want     string
	}{
		{"builtin/skill-manager", "skill-manager"},
		{"global/custom", "custom"},
		{"project/custom", "project/custom"},
	} {
		tool := NewLoadSkillTool(func(string) (string, string, error) { return tc.resolved, "body", nil })
		got := tool.Execute(context.Background(), json.RawMessage(`{"name":"custom"}`))
		want := "Skill loaded: " + tc.want + "\n\nbody"
		if got != want {
			t.Errorf("resolved %q result = %q, want %q", tc.resolved, got, want)
		}
	}
}
