// tools_test.go — tests for the tool registry and OpenAI format conversion.
package tools

import (
	"encoding/json"
	"testing"

	"blazeai/internal/platform"
)

// dummyTool is a minimal Tool implementation for registry tests.
type dummyTool struct {
	name string
	desc string
}

func (d *dummyTool) Name() string                           { return d.name }
func (d *dummyTool) Description() string                    { return d.desc }
func (d *dummyTool) Parameters() json.RawMessage            { return json.RawMessage(`{"type":"object"}`) }
func (d *dummyTool) Execute(args json.RawMessage) string    { return "ok" }
func (d *dummyTool) FormatArgs(args json.RawMessage) string { return "" }

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

// TestShellFormatArgs verifies shell tool formats command for display.
func TestShellFormatArgs(t *testing.T) {
	s := NewShellTool(platform.Linux)
	result := s.FormatArgs(json.RawMessage(`{"command":"cat '/path/file'"}`))
	if result != "cat '/path/file'" {
		t.Errorf("FormatArgs() = %q, want %q", result, "cat '/path/file'")
	}
}

// TestLoadSkillFormatArgs verifies load_skill formats name for display.
func TestLoadSkillFormatArgs(t *testing.T) {
	l := NewLoadSkillTool(nil)
	result := l.FormatArgs(json.RawMessage(`{"name":"memory.md"}`))
	if result != "memory" {
		t.Errorf("FormatArgs() = %q, want %q", result, "memory")
	}
}

// TestUnloadSkillFormatArgs verifies unload_skill formats name for display.
func TestUnloadSkillFormatArgs(t *testing.T) {
	u := NewUnloadSkillTool(nil)
	result := u.FormatArgs(json.RawMessage(`{"name":"memory"}`))
	if result != "memory" {
		t.Errorf("FormatArgs() = %q, want %q", result, "memory")
	}
}

// TestReplaceBlockFormatArgs verifies replace_block formats file path for display.
func TestReplaceBlockFormatArgs(t *testing.T) {
	r := NewReplaceBlockTool()
	result := r.FormatArgs(json.RawMessage(`{"file_path":"/path/to/file.go","old_block":"old","new_block":"new"}`))
	if result != "/path/to/file.go" {
		t.Errorf("FormatArgs() = %q, want %q", result, "/path/to/file.go")
	}
}
