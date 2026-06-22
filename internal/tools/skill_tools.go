// skill_tools.go — load_skill and unload_skill tool implementations.
// These tools modify the in-memory active skills list. They do not touch disk.
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
//
// WHAT:  Parsed arguments for skill management tools.
// PARAMS: Name — the skill name to load or unload.
type SkillArgs struct {
	Name string `json:"name"`
}

// LoadSkillTool adds a skill to the active skills list.
//
// WHAT:  Implements the load_skill tool — activates a skill by name.
// WHY:   The LLM calls this to make a skill's [DETAILS] available in subsequent prompts.
// PARAMS: active — the session's active skills list.
type LoadSkillTool struct {
	active *skills.ActiveList
}

// NewLoadSkillTool creates a LoadSkillTool bound to the given active list.
//
// PARAMS: active — the active skills list to modify.
// RETURNS: *LoadSkillTool — ready to execute.
func NewLoadSkillTool(active *skills.ActiveList) *LoadSkillTool {
	return &LoadSkillTool{active: active}
}

// Name returns the tool's unique identifier.
func (t *LoadSkillTool) Name() string {
	return "load_skill"
}

// FormatArgs extracts the skill name for display.
func (t *LoadSkillTool) FormatArgs(args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[SkillArgs](args)
	if err != nil || parsed.Name == "" {
		return ""
	}
	return normalizeSkillName(parsed.Name)
}

// Description returns the human-readable description for the LLM.
func (t *LoadSkillTool) Description() string {
	return "Load a skill by name to activate it. The skill's full details will be available in the next prompt."
}

// Parameters returns the JSON schema for the tool's parameters.
func (t *LoadSkillTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {
				"type": "string",
				"description": "The skill name to load."
			}
		},
		"required": ["name"]
	}`)
}

// Execute adds the skill name to the active list.
//
// PARAMS: ctx — turn cancellation context; args — raw JSON with the skill name.
// RETURNS: string — confirmation message.
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
	t.active.Load(name)
	return fmt.Sprintf("skill loaded: %s", name)
}

// UnloadSkillTool removes a skill from the active skills list.
//
// WHAT:  Implements the unload_skill tool — deactivates a skill by name.
// WHY:   The LLM calls this to remove a skill's [DETAILS] from subsequent prompts.
// PARAMS: active — the session's active skills list.
type UnloadSkillTool struct {
	active *skills.ActiveList
}

// NewUnloadSkillTool creates an UnloadSkillTool bound to the given active list.
//
// PARAMS: active — the active skills list to modify.
// RETURNS: *UnloadSkillTool — ready to execute.
func NewUnloadSkillTool(active *skills.ActiveList) *UnloadSkillTool {
	return &UnloadSkillTool{active: active}
}

// Name returns the tool's unique identifier.
func (t *UnloadSkillTool) Name() string {
	return "unload_skill"
}

// FormatArgs extracts the skill name for display.
func (t *UnloadSkillTool) FormatArgs(args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[SkillArgs](args)
	if err != nil || parsed.Name == "" {
		return ""
	}
	return normalizeSkillName(parsed.Name)
}

// Description returns the human-readable description for the LLM.
func (t *UnloadSkillTool) Description() string {
	return "Unload a skill by name to deactivate it. The skill's details will no longer be in the prompt."
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

// Execute removes the skill name from the active list.
//
// PARAMS: ctx — turn cancellation context; args — raw JSON with the skill name.
// RETURNS: string — confirmation message.
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
	t.active.Unload(name)
	return fmt.Sprintf("skill unloaded: %s", name)
}

// normalizeSkillName converts the user-facing skill filename to the internal skill key.
//
// WHAT:  Strips the optional .md suffix from a skill name.
// WHY:   Available skills are displayed as filenames like memory.md, while discovery keys use
//
//	the basename without extension, like memory.
//
// PARAMS: name — the skill name from tool input.
// RETURNS: string — normalized internal skill name.
func normalizeSkillName(name string) string {
	return strings.TrimSuffix(name, ".md")
}
