// memory_tools_test.go — tests for load_memory and unload_memory tools.
package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"blazeai/internal/memories"
)

// TestLoadMemoryExecute verifies that a memory is added to the active list.
func TestLoadMemoryExecute(t *testing.T) {
	active := memories.NewActiveList()
	tool := NewLoadMemoryTool(active)
	args := json.RawMessage(`{"name":"my-network"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "memory loaded: my-network") {
		t.Errorf("Execute() = %q, want 'memory loaded: my-network'", result)
	}
	if !active.Has("my-network") {
		t.Error("active list does not contain 'my-network' after load")
	}
}

// TestLoadMemoryExecuteWithMarkdownSuffix verifies .md names are normalized.
func TestLoadMemoryExecuteWithMarkdownSuffix(t *testing.T) {
	active := memories.NewActiveList()
	tool := NewLoadMemoryTool(active)
	args := json.RawMessage(`{"name":"my-network.md"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "memory loaded: my-network") {
		t.Errorf("Execute() = %q, want 'memory loaded: my-network'", result)
	}
	if !active.Has("my-network") {
		t.Error("active list does not contain normalized 'my-network' after load")
	}
	if active.Has("my-network.md") {
		t.Error("active list should not contain raw 'my-network.md' after load")
	}
}

// TestUnloadMemoryExecute verifies that a memory is removed from the active list.
func TestUnloadMemoryExecute(t *testing.T) {
	active := memories.NewActiveList()
	active.Load("my-network")
	tool := NewUnloadMemoryTool(active)
	args := json.RawMessage(`{"name":"my-network"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "memory unloaded: my-network") {
		t.Errorf("Execute() = %q, want 'memory unloaded: my-network'", result)
	}
	if active.Has("my-network") {
		t.Error("active list still contains 'my-network' after unload")
	}
}

// TestMemoryToolName verifies the tool names.
func TestMemoryToolName(t *testing.T) {
	if NewLoadMemoryTool(memories.NewActiveList()).Name() != "load_memory" {
		t.Fatal("load_memory name mismatch")
	}
	if NewUnloadMemoryTool(memories.NewActiveList()).Name() != "unload_memory" {
		t.Fatal("unload_memory name mismatch")
	}
}
