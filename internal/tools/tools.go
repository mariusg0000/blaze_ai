// tools.go — native tool interface, registry, and OpenAI tool-calling format definitions.
// Defines the Tool interface, Registry, and JSON structures for OpenAI-compatible tool calls.
// Layer: tool execution. Dependencies: internal/skills, internal/platform.
package tools

import (
	"encoding/json"
	"fmt"
)

// DefaultTimeout is the default tool execution timeout in seconds per spec.
const DefaultTimeout = 60

// Tool defines the contract for a native tool executable by the runtime.
//
// WHAT:  Interface for all native tools (shell, load_skill, unload_skill, replace_block).
// WHY:   The runtime executes tools through this uniform interface regardless of implementation.
type Tool interface {
	// Name returns the tool's unique identifier.
	Name() string
	// Description returns the human-readable description for the LLM.
	Description() string
	// Parameters returns the JSON schema for the tool's parameters.
	Parameters() json.RawMessage
	// Execute runs the tool with the given JSON arguments and returns a result string.
	Execute(args json.RawMessage) string
}

// Registry holds all native tools keyed by name.
//
// WHAT:  Maps tool names to tool implementations.
// WHY:   The runtime looks up tools by name when processing LLM tool calls.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry returns an empty Registry.
//
// WHAT:  Creates a new tool registry.
// RETURNS: *Registry — empty registry ready for Register calls.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry. Panics on duplicate name (hardcoded tools, not dynamic).
//
// WHAT:  Adds a tool to the registry by its Name().
// WHY:   All four native tools are registered at startup.
// PARAMS: tool — the tool to register.
func (r *Registry) Register(tool Tool) {
	name := tool.Name()
	if _, exists := r.tools[name]; exists {
		panic(fmt.Sprintf("duplicate tool registration: %s", name))
	}
	r.tools[name] = tool
}

// Get returns a tool by name, or nil if not found.
//
// WHAT:  Looks up a tool by name.
// PARAMS: name — the tool identifier.
// RETURNS: Tool — the registered tool or nil.
func (r *Registry) Get(name string) Tool {
	return r.tools[name]
}

// All returns all registered tools.
//
// WHAT:  Returns all tools in the registry.
// RETURNS: []Tool — all registered tools.
func (r *Registry) All() []Tool {
	result := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

// OpenAITool is the OpenAI-compatible tool definition sent in chat completion requests.
//
// WHAT:  Represents one tool in the OpenAI tools array.
// PARAMS: Type — always "function"; Function — the function definition.
type OpenAITool struct {
	Type     string       `json:"type"`
	Function FunctionDef  `json:"function"`
}

// FunctionDef defines the function interface for the LLM.
//
// WHAT:  The function metadata: name, description, and parameter schema.
// PARAMS: Name — tool identifier; Description — what the tool does; Parameters — JSON schema.
type FunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToOpenAI converts a Tool to the OpenAI tool definition format.
//
// WHAT:  Transforms a Tool interface into the OpenAI API tool definition.
// PARAMS: tool — the tool to convert.
// RETURNS: OpenAITool — the API-compatible tool definition.
func ToOpenAI(tool Tool) OpenAITool {
	return OpenAITool{
		Type: "function",
		Function: FunctionDef{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  tool.Parameters(),
		},
	}
}

// AllToOpenAI converts all tools in a registry to OpenAI tool definitions.
//
// WHAT:  Converts the entire registry to the OpenAI tools array format.
// PARAMS: r — the registry to convert.
// RETURNS: []OpenAITool — all tools in API format.
func AllToOpenAI(r *Registry) []OpenAITool {
	tools := r.All()
	result := make([]OpenAITool, len(tools))
	for i, t := range tools {
		result[i] = ToOpenAI(t)
	}
	return result
}

// ToolCall represents a tool call from the LLM response.
//
// WHAT:  Holds the parsed tool call from the assistant response.
// PARAMS: ID — call identifier from the API; Name — tool name; Arguments — raw JSON arguments.
type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

// ParseToolCallArgs extracts typed arguments from raw JSON.
//
// WHAT:  Unmarshals tool call arguments into a typed struct.
// WHY:   Each tool needs its specific argument structure from the raw JSON.
// PARAMS: args — raw JSON arguments from the LLM.
// RETURNS: T — the typed arguments; error if JSON parsing fails.
func ParseToolCallArgs[T any](args json.RawMessage) (T, error) {
	var result T
	if err := json.Unmarshal(args, &result); err != nil {
		return result, fmt.Errorf("cannot parse tool arguments: %w", err)
	}
	return result, nil
}
