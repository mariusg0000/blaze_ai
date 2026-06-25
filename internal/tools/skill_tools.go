// skill_tools.go — load_skill and unload_skill tool implementations.
// These tools modify the in-memory active skills list. They validate existence
// and resolve ambiguous names via an injectable resolver function.
// Layer: tool execution. Dependencies: internal/skills.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"blazeai/internal/skills"
)

// SkillArgs are the arguments for load_skill and unload_skill.
type SkillArgs struct {
	Name string `json:"name"`
}

// ResolveFunc resolves a skill name to a canonical skill ID.
// Returns an error if the name is not found or ambiguous.
type ResolveFunc func(name string) (string, error)

// LoadSkillTool adds a skill to the active skills list.
type LoadSkillTool struct {
	active  *skills.ActiveList
	resolve ResolveFunc
}

// NewLoadSkillTool creates a LoadSkillTool bound to the given active list and resolver.
func NewLoadSkillTool(active *skills.ActiveList, resolve ResolveFunc) *LoadSkillTool {
	return &LoadSkillTool{active: active, resolve: resolve}
}

// Name returns the tool's unique identifier.
func (t *LoadSkillTool) Name() string {
	return "load_skill"
}

// FormatArgs returns a fixed UI label for the skill load action.
func (t *LoadSkillTool) FormatArgs(args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[SkillArgs](args)
	if err != nil {
		return "Loading skill"
	}
	if parsed.Name == "" {
		return "Loading skill"
	}
	return truncateDisplay("Loading skill: "+normalizeSkillName(parsed.Name), 80)
}

// Description returns the human-readable description for the LLM.
func (t *LoadSkillTool) Description() string {
	return "Load a skill into the current session. Use project/ prefix for project-scoped skills."
}

// Parameters returns the JSON schema for the tool's parameters.
func (t *LoadSkillTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {
				"type": "string",
				"description": "The skill name to load. Use project/ prefix for project-scoped skills."
			}
		},
		"required": ["name"]
	}`)
}

// Execute resolves the skill name and adds it to the active list.
func (t *LoadSkillTool) Execute(ctx context.Context, args json.RawMessage) string {
	if ctx != nil && ctx.Err() != nil {
		return "aborted before execution by user"
	}
	parsed, err := ParseToolCallArgs[SkillArgs](args)
	if err != nil {
		return fmt.Sprintf("error: invalid arguments: %v", err)
	}
	if parsed.Name == "" {
		return "error: name is required"
	}

	name := normalizeSkillName(parsed.Name)
	if t.resolve != nil {
		resolved, err := t.resolve(name)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		t.active.Load(resolved)
		return fmt.Sprintf("ok skill loaded: %s", strings.TrimPrefix(resolved, "global/"))
	}
	t.active.Load(name)
	return fmt.Sprintf("ok skill loaded: %s", name)
}

// UnloadSkillTool removes a skill from the active skills list.
type UnloadSkillTool struct {
	active  *skills.ActiveList
	resolve ResolveFunc
}

// NewUnloadSkillTool creates an UnloadSkillTool bound to the given active list and resolver.
func NewUnloadSkillTool(active *skills.ActiveList, resolve ResolveFunc) *UnloadSkillTool {
	return &UnloadSkillTool{active: active, resolve: resolve}
}

// Name returns the tool's unique identifier.
func (t *UnloadSkillTool) Name() string {
	return "unload_skill"
}

// FormatArgs returns a fixed UI label for the skill unload action.
func (t *UnloadSkillTool) FormatArgs(args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[SkillArgs](args)
	if err != nil {
		return "Unloading skill"
	}
	if parsed.Name == "" {
		return "Unloading skill"
	}
	return truncateDisplay("Unloading skill: "+normalizeSkillName(parsed.Name), 80)
}

// Description returns the human-readable description for the LLM.
func (t *UnloadSkillTool) Description() string {
	return "Unload a skill from the current session."
}

// Parameters returns the JSON schema for the tool's parameters.
func (t *UnloadSkillTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {
				"type": "string",
				"description": "The skill name to unload."
			}
		},
		"required": ["name"]
	}`)
}

// Execute resolves the skill name and removes it from the active list.
func (t *UnloadSkillTool) Execute(ctx context.Context, args json.RawMessage) string {
	if ctx != nil && ctx.Err() != nil {
		return "aborted before execution by user"
	}
	parsed, err := ParseToolCallArgs[SkillArgs](args)
	if err != nil {
		return fmt.Sprintf("error: invalid arguments: %v", err)
	}
	if parsed.Name == "" {
		return "error: name is required"
	}

	name := normalizeSkillName(parsed.Name)

	// Try exact lookup first (what's in active list).
	t.active.Unload(name)

	// Also try resolving in case the active list has a canonical ID.
	if t.resolve != nil {
		resolved, err := t.resolve(name)
		if err == nil && resolved != name {
			t.active.Unload(resolved)
		}
	}

	return fmt.Sprintf("ok skill unloaded: %s", name)
}

// normalizeSkillName strips the optional .md suffix.
func normalizeSkillName(name string) string {
	return strings.TrimSuffix(name, ".md")
}
