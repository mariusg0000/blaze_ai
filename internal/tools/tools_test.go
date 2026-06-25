// tools_test.go — tests for the tool registry and OpenAI format conversion.
package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"blazeai/internal/platform"
)

func schemaIncludesRequiredPurpose(t *testing.T, raw json.RawMessage) {
	t.Helper()
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("cannot parse schema: %v", err)
	}
	if _, ok := schema.Properties["purpose"]; !ok {
		t.Fatal("schema missing purpose property")
	}
	for _, name := range schema.Required {
		if name == "purpose" {
			return
		}
	}
	t.Fatal("schema missing required purpose field")
}

// dummyTool is a minimal Tool implementation for registry tests.
type dummyTool struct {
	name string
	desc string
}

func (d *dummyTool) Name() string                                             { return d.name }
func (d *dummyTool) Description() string                                      { return d.desc }
func (d *dummyTool) Parameters() json.RawMessage                              { return json.RawMessage(`{"type":"object"}`) }
func (d *dummyTool) Execute(ctx context.Context, args json.RawMessage) string { return "ok" }
func (d *dummyTool) FormatArgs(args json.RawMessage) string                   { return "" }

// TestRegistryRegisterAndGet verifies basic register and lookup.
func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	d := &dummyTool{name: "test", desc: "a test tool"}
	r.Register(d)

	got := r.Get("test")
	if got == nil {
		t.Fatal("Get() returned nil for registered tool")
	}
	if got.Name() != "test" {
		t.Errorf("Get().Name() = %q, want 'test'", got.Name())
	}
}

// TestRegistryGetMissing verifies nil for unregistered tool.
func TestRegistryGetMissing(t *testing.T) {
	r := NewRegistry()
	if r.Get("nonexistent") != nil {
		t.Error("Get() returned non-nil for unregistered tool")
	}
}

// TestRegistryAll verifies all registered tools are returned.
func TestRegistryAll(t *testing.T) {
	r := NewRegistry()
	r.Register(&dummyTool{name: "a", desc: "a"})
	r.Register(&dummyTool{name: "b", desc: "b"})
	all := r.All()
	if len(all) != 2 {
		t.Errorf("All() = %d tools, want 2", len(all))
	}
}

// TestToOpenAI verifies conversion to OpenAI tool format.
func TestToOpenAI(t *testing.T) {
	d := &dummyTool{name: "shell", desc: "run a command"}
	oai := ToOpenAI(d)
	if oai.Type != "function" {
		t.Errorf("Type = %q, want 'function'", oai.Type)
	}
	if oai.Function.Name != "shell" {
		t.Errorf("Function.Name = %q, want 'shell'", oai.Function.Name)
	}
	if oai.Function.Description != "run a command" {
		t.Errorf("Function.Description = %q, want 'run a command'", oai.Function.Description)
	}
}

// TestAllToOpenAI verifies registry conversion to OpenAI tools array.
func TestAllToOpenAI(t *testing.T) {
	r := NewRegistry()
	r.Register(&dummyTool{name: "a", desc: "a"})
	r.Register(&dummyTool{name: "b", desc: "b"})
	oai := AllToOpenAI(r)
	if len(oai) != 2 {
		t.Fatalf("AllToOpenAI() = %d tools, want 2", len(oai))
	}
	for _, tool := range oai {
		if tool.Type != "function" {
			t.Errorf("Type = %q, want 'function'", tool.Type)
		}
	}
}

// TestParseToolCallArgs verifies typed argument parsing from raw JSON.
func TestParseToolCallArgs(t *testing.T) {
	args := json.RawMessage(`{"command":"ls -la","timeout":30}`)
	parsed, err := ParseToolCallArgs[ShellArgs](args)
	if err != nil {
		t.Fatalf("ParseToolCallArgs() error: %v", err)
	}
	if parsed.Command != "ls -la" {
		t.Errorf("Command = %q, want 'ls -la'", parsed.Command)
	}
	if parsed.Timeout == nil || *parsed.Timeout != 30 {
		t.Errorf("Timeout = %v, want 30", parsed.Timeout)
	}
}

// TestParseToolCallArgsInvalid verifies error on malformed JSON.
func TestParseToolCallArgsInvalid(t *testing.T) {
	args := json.RawMessage(`{invalid}`)
	_, err := ParseToolCallArgs[ShellArgs](args)
	if err == nil {
		t.Fatal("ParseToolCallArgs() expected error for invalid JSON, got nil")
	}
}

// TestRegistryFormatArgsFallback verifies fallback to raw JSON when FormatArgs returns empty.
func TestRegistryFormatArgsFallback(t *testing.T) {
	r := NewRegistry()
	r.Register(&dummyTool{name: "test", desc: "a test tool"})
	result := r.FormatArgs("test", json.RawMessage(`{"key":"value"}`))
	if result != `{"key":"value"}` {
		t.Errorf("FormatArgs() = %q, want raw JSON fallback", result)
	}
}

// TestRegistryFormatArgsMissingTool verifies fallback for unknown tool.
func TestRegistryFormatArgsMissingTool(t *testing.T) {
	r := NewRegistry()
	result := r.FormatArgs("unknown", json.RawMessage(`{"x":1}`))
	if result != `{"x":1}` {
		t.Errorf("FormatArgs() = %q, want raw JSON fallback for missing tool", result)
	}
}

// TestShellFormatArgs verifies shell tool prefers purpose for display.
func TestShellFormatArgs(t *testing.T) {
	s := NewShellTool(platform.Linux)
	result := s.FormatArgs(json.RawMessage(`{"purpose":"Inspect package.json scripts","command":"cat '/path/file'"}`))
	if result != "Inspect package.json scripts" {
		t.Errorf("FormatArgs() = %q, want %q", result, "Inspect package.json scripts")
	}
}

// TestShellFormatArgsFallback verifies shell falls back to command when purpose is missing.
func TestShellFormatArgsFallback(t *testing.T) {
	s := NewShellTool(platform.Linux)
	result := s.FormatArgs(json.RawMessage(`{"command":"cat '/path/file'"}`))
	if result != "cat '/path/file'" {
		t.Errorf("FormatArgs() = %q, want %q", result, "cat '/path/file'")
	}
}

// TestShellFormatArgsFallbackTruncated verifies shell fallback is truncated at 80 chars.
func TestShellFormatArgsFallbackTruncated(t *testing.T) {
	s := NewShellTool(platform.Linux)
	longCmd := "echo "
	for i := 0; i < 90; i++ {
		longCmd += "x"
	}
	result := s.FormatArgs(json.RawMessage(`{"command":"` + longCmd + `"}`))
	if len(result) > 80 {
		t.Errorf("FormatArgs() len = %d, want ≤ 80", len(result))
	}
	if !strings.HasSuffix(result, "...") {
		t.Errorf("FormatArgs() = %q, want truncated with '...'", result)
	}
}

// TestLoadSkillFormatArgs verifies load_skill prefers purpose for display.
func TestLoadSkillFormatArgs(t *testing.T) {
	l := NewLoadSkillTool(nil, nil)
	result := l.FormatArgs(json.RawMessage(`{"purpose":"Load memory manager skill for persistence rules","name":"memory-manager.md"}`))
	if result != "Load memory manager skill for persistence rules" {
		t.Errorf("FormatArgs() = %q, want %q", result, "Load memory manager skill for persistence rules")
	}
}

// TestLoadSkillFormatArgsFallback verifies load_skill falls back to normalized skill name.
func TestLoadSkillFormatArgsFallback(t *testing.T) {
	l := NewLoadSkillTool(nil, nil)
	result := l.FormatArgs(json.RawMessage(`{"name":"memory-manager.md"}`))
	if result != "memory-manager" {
		t.Errorf("FormatArgs() = %q, want %q", result, "memory-manager")
	}
}

// TestUnloadSkillFormatArgs verifies unload_skill prefers purpose for display.
func TestUnloadSkillFormatArgs(t *testing.T) {
	u := NewUnloadSkillTool(nil, nil)
	result := u.FormatArgs(json.RawMessage(`{"purpose":"Unload memory manager skill after finishing persistence update","name":"memory-manager"}`))
	if result != "Unload memory manager skill after finishing persistence update" {
		t.Errorf("FormatArgs() = %q, want %q", result, "Unload memory manager skill after finishing persistence update")
	}
}

// TestUnloadSkillFormatArgsFallback verifies unload_skill falls back to skill name.
func TestUnloadSkillFormatArgsFallback(t *testing.T) {
	u := NewUnloadSkillTool(nil, nil)
	result := u.FormatArgs(json.RawMessage(`{"name":"memory-manager"}`))
	if result != "memory-manager" {
		t.Errorf("FormatArgs() = %q, want %q", result, "memory-manager")
	}
}

// TestReplaceBlockFormatArgs verifies replace_block prefers purpose for display.
func TestReplaceBlockFormatArgs(t *testing.T) {
	r := NewReplaceBlockTool()
	result := r.FormatArgs(json.RawMessage(`{"purpose":"Update console renderer in internal/console/console.go","file_path":"/path/to/file.go","old_block":"old","new_block":"new"}`))
	if result != "Update console renderer in internal/console/console.go" {
		t.Errorf("FormatArgs() = %q, want %q", result, "Update console renderer in internal/console/console.go")
	}
}

// TestReplaceBlockFormatArgsFallback verifies replace_block falls back to file path.
func TestReplaceBlockFormatArgsFallback(t *testing.T) {
	r := NewReplaceBlockTool()
	result := r.FormatArgs(json.RawMessage(`{"file_path":"/path/to/file.go","old_block":"old","new_block":"new"}`))
	if result != "/path/to/file.go" {
		t.Errorf("FormatArgs() = %q, want %q", result, "/path/to/file.go")
	}
}
