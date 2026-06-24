// memorybank_tools.go — load_memory_bank and unload_memory_bank tool implementations.
// These tools modify the in-memory active memory-bank list. They do not touch disk.
// Layer: tool execution. Dependencies: internal/memorybanks.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"blazeai/internal/memorybanks"
)

// MemoryBankArgs are the arguments for load_memory_bank and unload_memory_bank.
//
// WHAT:  Parsed arguments for memory-bank management tools.
// PARAMS: Name — the memory-bank name to load or unload.
type MemoryBankArgs struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose,omitempty"`
}

// LoadMemoryBankTool adds a memory-bank to the active memory-bank list.
//
// WHAT:  Implements the load_memory_bank tool — activates a memory-bank by name.
// WHY:   The LLM calls this to make a memory-bank's [DETAILS] available in subsequent prompts.
// PARAMS: active — the session's active memory-bank list.
type LoadMemoryBankTool struct {
	active *memorybanks.ActiveList
}

// NewLoadMemoryBankTool creates a LoadMemoryBankTool bound to the given active list.
//
// PARAMS: active — the active memory-bank list to modify.
// RETURNS: *LoadMemoryBankTool — ready to execute.
func NewLoadMemoryBankTool(active *memorybanks.ActiveList) *LoadMemoryBankTool {
	return &LoadMemoryBankTool{active: active}
}

// Name returns the tool's unique identifier.
func (t *LoadMemoryBankTool) Name() string {
	return "load_memory_bank"
}

// FormatArgs extracts the memory-bank name for display.
func (t *LoadMemoryBankTool) FormatArgs(args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[MemoryBankArgs](args)
	if err != nil {
		return ""
	}
	if strings.TrimSpace(parsed.Purpose) != "" {
		return strings.TrimSpace(parsed.Purpose)
	}
	if parsed.Name == "" {
		return ""
	}
	return truncateDisplay(normalizeMemoryBankName(parsed.Name), 80)
}

// Description returns the human-readable description for the LLM.
func (t *LoadMemoryBankTool) Description() string {
	return "Load a memory-bank by name to activate it. The memory-bank's full details will be available in the next prompt."
}

// Parameters returns the JSON schema for the tool's parameters.
func (t *LoadMemoryBankTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"purpose": {
				"type": "string",
				"description": "A concise 1-2 sentence summary of this memory-bank change. State the intent and the memory-bank being activated."
			},
			"name": {
				"type": "string",
				"description": "The memory-bank name to load."
			}
		},
		"required": ["purpose", "name"]
	}`)
}

// Execute adds the memory-bank name to the active list.
//
// PARAMS: ctx — turn cancellation context; args — raw JSON with the memory-bank name.
// RETURNS: string — confirmation message.
func (t *LoadMemoryBankTool) Execute(ctx context.Context, args json.RawMessage) string {
	if ctx != nil && ctx.Err() != nil {
		return "aborted before execution by user"
	}
	parsed, err := ParseToolCallArgs[MemoryBankArgs](args)
	if err != nil {
		return fmt.Sprintf("error: invalid arguments: %v", err)
	}
	if parsed.Name == "" {
		return "error: name is required"
	}
	name := normalizeMemoryBankName(parsed.Name)
	t.active.Load(name)
	return fmt.Sprintf("memory-bank loaded: %s", name)
}

// UnloadMemoryBankTool removes a memory-bank from the active memory-bank list.
//
// WHAT:  Implements the unload_memory_bank tool — deactivates a memory-bank by name.
// WHY:   The LLM calls this to remove a memory-bank's [DETAILS] from subsequent prompts.
// PARAMS: active — the session's active memory-bank list.
type UnloadMemoryBankTool struct {
	active *memorybanks.ActiveList
}

// NewUnloadMemoryBankTool creates an UnloadMemoryBankTool bound to the given active list.
//
// PARAMS: active — the active memory-bank list to modify.
// RETURNS: *UnloadMemoryBankTool — ready to execute.
func NewUnloadMemoryBankTool(active *memorybanks.ActiveList) *UnloadMemoryBankTool {
	return &UnloadMemoryBankTool{active: active}
}

// Name returns the tool's unique identifier.
func (t *UnloadMemoryBankTool) Name() string {
	return "unload_memory_bank"
}

// FormatArgs extracts the memory-bank name for display.
func (t *UnloadMemoryBankTool) FormatArgs(args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[MemoryBankArgs](args)
	if err != nil {
		return ""
	}
	if strings.TrimSpace(parsed.Purpose) != "" {
		return strings.TrimSpace(parsed.Purpose)
	}
	if parsed.Name == "" {
		return ""
	}
	return truncateDisplay(normalizeMemoryBankName(parsed.Name), 80)
}

// Description returns the human-readable description for the LLM.
func (t *UnloadMemoryBankTool) Description() string {
	return "Unload a memory-bank by name to deactivate it for subsequent prompts. Use this when the user clearly changes topic or task, or when the loaded memory-bank would interfere with the next turn. Do not unload a memory-bank immediately after one successful action if the conversation is continuing in the same domain."
}

// Parameters returns the JSON schema for the tool's parameters.
func (t *UnloadMemoryBankTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"purpose": {
				"type": "string",
				"description": "A concise 1-2 sentence summary of this memory-bank change. State the intent and the memory-bank being deactivated, and use unload only for a clear topic or task shift or when the memory-bank would interfere with the next turn."
			},
			"name": {
				"type": "string",
				"description": "The memory-bank name to unload from subsequent prompts. Do not use this for reflex cleanup after a single successful action in the same ongoing domain."
			}
		},
		"required": ["purpose", "name"]
	}`)
}

// Execute removes the memory-bank name from the active list.
//
// PARAMS: ctx — turn cancellation context; args — raw JSON with the memory-bank name.
// RETURNS: string — confirmation message.
func (t *UnloadMemoryBankTool) Execute(ctx context.Context, args json.RawMessage) string {
	if ctx != nil && ctx.Err() != nil {
		return "aborted before execution by user"
	}
	parsed, err := ParseToolCallArgs[MemoryBankArgs](args)
	if err != nil {
		return fmt.Sprintf("error: invalid arguments: %v", err)
	}
	if parsed.Name == "" {
		return "error: name is required"
	}
	name := normalizeMemoryBankName(parsed.Name)
	t.active.Unload(name)
	return fmt.Sprintf("memory-bank unloaded: %s", name)
}

// normalizeMemoryBankName converts the user-facing memory-bank filename to the internal key.
//
// WHAT:  Strips the optional .md suffix from a memory-bank name.
// WHY:   Available memory-banks are displayed as filenames like my-network.md, while discovery keys use
//
//	the basename without extension, like my-network.
//
// PARAMS: name — the memory-bank name from tool input.
// RETURNS: string — normalized internal memory-bank name.
func normalizeMemoryBankName(name string) string {
	return strings.TrimSuffix(name, ".md")
}
