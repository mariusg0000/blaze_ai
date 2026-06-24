// memorybank_tools_test.go — tests for load_memory_bank and unload_memory_bank tools.
package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"blazeai/internal/memorybanks"
)

// TestLoadMemoryBankExecute verifies that a memory-bank is added to the active list.
func TestLoadMemoryBankExecute(t *testing.T) {
	active := memorybanks.NewActiveList()
	tool := NewLoadMemoryBankTool(active)
	args := json.RawMessage(`{"name":"my-network"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "memory-bank loaded: my-network") {
		t.Errorf("Execute() = %q, want 'memory-bank loaded: my-network'", result)
	}
	if !active.Has("my-network") {
		t.Error("active list does not contain 'my-network' after load")
	}
}

// TestLoadMemoryBankExecuteWithMarkdownSuffix verifies .md names are normalized.
func TestLoadMemoryBankExecuteWithMarkdownSuffix(t *testing.T) {
	active := memorybanks.NewActiveList()
	tool := NewLoadMemoryBankTool(active)
	args := json.RawMessage(`{"name":"my-network.md"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "memory-bank loaded: my-network") {
		t.Errorf("Execute() = %q, want 'memory-bank loaded: my-network'", result)
	}
	if !active.Has("my-network") {
		t.Error("active list does not contain normalized 'my-network' after load")
	}
	if active.Has("my-network.md") {
		t.Error("active list should not contain raw 'my-network.md' after load")
	}
}

// TestUnloadMemoryBankExecute verifies that a memory-bank is removed from the active list.
func TestUnloadMemoryBankExecute(t *testing.T) {
	active := memorybanks.NewActiveList()
	active.Load("my-network")
	tool := NewUnloadMemoryBankTool(active)
	args := json.RawMessage(`{"name":"my-network"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "memory-bank unloaded: my-network") {
		t.Errorf("Execute() = %q, want 'memory-bank unloaded: my-network'", result)
	}
	if active.Has("my-network") {
		t.Error("active list still contains 'my-network' after unload")
	}
}

// TestMemoryBankToolName verifies the tool names.
func TestMemoryBankToolName(t *testing.T) {
	if NewLoadMemoryBankTool(memorybanks.NewActiveList()).Name() != "load_memory_bank" {
		t.Fatal("load_memory_bank name mismatch")
	}
	if NewUnloadMemoryBankTool(memorybanks.NewActiveList()).Name() != "unload_memory_bank" {
		t.Fatal("unload_memory_bank name mismatch")
	}
}
