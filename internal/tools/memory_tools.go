// memory_tools.go — load_memory and unload_memory tool implementations.
// These tools modify the in-memory active memory list. They do not touch disk.
// Layer: tool execution. Dependencies: internal/memories.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"blazeai/internal/memories"
)

// MemoryArgs are the arguments for load_memory and unload_memory.
//
// WHAT:  Parsed arguments for memory management tools.
// PARAMS: Name — the memory name to load or unload.
type MemoryArgs struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose,omitempty"`
}

// LoadMemoryTool adds a memory to the active memory list.
//
// WHAT:  Implements the load_memory tool — activates a memory by name.
// WHY:   The LLM calls this to make a memory's [DETAILS] available in subsequent prompts.
// PARAMS: active — the session's active memory list.
type LoadMemoryTool struct {
	active *memories.ActiveList
}

// NewLoadMemoryTool creates a LoadMemoryTool bound to the given active list.
//
// PARAMS: active — the active memory list to modify.
// RETURNS: *LoadMemoryTool — ready to execute.
func NewLoadMemoryTool(active *memories.ActiveList) *LoadMemoryTool {
	return &LoadMemoryTool{active: active}
}

// Name returns the tool's unique identifier.
func (t *LoadMemoryTool) Name() string {
	return "load_memory"
}

// FormatArgs extracts the memory name for display.
func (t *LoadMemoryTool) FormatArgs(args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[MemoryArgs](args)
	if err != nil {
		return ""
	}
	if strings.TrimSpace(parsed.Purpose) != "" {
		return strings.TrimSpace(parsed.Purpose)
	}
	if parsed.Name == "" {
		return ""
	}
	return truncateDisplay(normalizeMemoryName(parsed.Name), 80)
}

// Description returns the human-readable description for the LLM.
func (t *LoadMemoryTool) Description() string {
	return "Load a memory by name to activate it. The memory's full details will be available in the next prompt."
}

// Parameters returns the JSON schema for the tool's parameters.
func (t *LoadMemoryTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"purpose": {
				"type": "string",
				"description": "A concise 1-2 sentence summary of this memory change. State the intent and the memory being activated."
			},
			"name": {
				"type": "string",
				"description": "The memory name to load."
			}
		},
		"required": ["purpose", "name"]
	}`)
}

// Execute adds the memory name to the active list.
//
// PARAMS: ctx — turn cancellation context; args — raw JSON with the memory name.
// RETURNS: string — confirmation message.
func (t *LoadMemoryTool) Execute(ctx context.Context, args json.RawMessage) string {
	if ctx != nil && ctx.Err() != nil {
		return "aborted before execution by user"
	}
	parsed, err := ParseToolCallArgs[MemoryArgs](args)
	if err != nil {
		return fmt.Sprintf("error: invalid arguments: %v", err)
	}
	if parsed.Name == "" {
		return "error: name is required"
	}
	name := normalizeMemoryName(parsed.Name)
	t.active.Load(name)
	return fmt.Sprintf("memory loaded: %s", name)
}

// UnloadMemoryTool removes a memory from the active memory list.
//
// WHAT:  Implements the unload_memory tool — deactivates a memory by name.
// WHY:   The LLM calls this to remove a memory's [DETAILS] from subsequent prompts.
// PARAMS: active — the session's active memory list.
type UnloadMemoryTool struct {
	active *memories.ActiveList
}

// NewUnloadMemoryTool creates an UnloadMemoryTool bound to the given active list.
//
// PARAMS: active — the active memory list to modify.
// RETURNS: *UnloadMemoryTool — ready to execute.
func NewUnloadMemoryTool(active *memories.ActiveList) *UnloadMemoryTool {
	return &UnloadMemoryTool{active: active}
}

// Name returns the tool's unique identifier.
func (t *UnloadMemoryTool) Name() string {
	return "unload_memory"
}

// FormatArgs extracts the memory name for display.
func (t *UnloadMemoryTool) FormatArgs(args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[MemoryArgs](args)
	if err != nil {
		return ""
	}
	if strings.TrimSpace(parsed.Purpose) != "" {
		return strings.TrimSpace(parsed.Purpose)
	}
	if parsed.Name == "" {
		return ""
	}
	return truncateDisplay(normalizeMemoryName(parsed.Name), 80)
}

// Description returns the human-readable description for the LLM.
func (t *UnloadMemoryTool) Description() string {
	return "Unload a memory by name to deactivate it for subsequent prompts. Use this when the user clearly changes topic or task, or when the loaded memory would interfere with the next turn. Do not unload a memory immediately after one successful action if the conversation is continuing in the same domain."
}

// Parameters returns the JSON schema for the tool's parameters.
func (t *UnloadMemoryTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"purpose": {
				"type": "string",
				"description": "A concise 1-2 sentence summary of this memory change. State the intent and the memory being deactivated, and use unload only for a clear topic or task shift or when the memory would interfere with the next turn."
			},
			"name": {
				"type": "string",
				"description": "The memory name to unload from subsequent prompts. Do not use this for reflex cleanup after a single successful action in the same ongoing domain."
			}
		},
		"required": ["purpose", "name"]
	}`)
}

// Execute removes the memory name from the active list.
//
// PARAMS: ctx — turn cancellation context; args — raw JSON with the memory name.
// RETURNS: string — confirmation message.
func (t *UnloadMemoryTool) Execute(ctx context.Context, args json.RawMessage) string {
	if ctx != nil && ctx.Err() != nil {
		return "aborted before execution by user"
	}
	parsed, err := ParseToolCallArgs[MemoryArgs](args)
	if err != nil {
		return fmt.Sprintf("error: invalid arguments: %v", err)
	}
	if parsed.Name == "" {
		return "error: name is required"
	}
	name := normalizeMemoryName(parsed.Name)
	t.active.Unload(name)
	return fmt.Sprintf("memory unloaded: %s", name)
}

// normalizeMemoryName converts the user-facing memory filename to the internal key.
//
// WHAT:  Strips the optional .md suffix from a memory name.
// WHY:   Available memories are displayed as filenames like my-network.md, while discovery keys use
//
//	the basename without extension, like my-network.
//
// PARAMS: name — the memory name from tool input.
// RETURNS: string — normalized internal memory name.
func normalizeMemoryName(name string) string {
	return strings.TrimSuffix(name, ".md")
}
