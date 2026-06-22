// tools_test.go — tests for the tool registry and OpenAI format conversion.
package tools

import (
	"encoding/json"
	"testing"
)

// dummyTool is a minimal Tool implementation for registry tests.
type dummyTool struct {
	name string
	desc string
}

func (d *dummyTool) Name() string                 { return d.name }
func (d *dummyTool) Description() string           { return d.desc }
func (d *dummyTool) Parameters() json.RawMessage   { return json.RawMessage(`{"type":"object"}`) }
func (d *dummyTool) Execute(args json.RawMessage) string { return "ok" }

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
