// skill_tools.go — load_skill tool implementation.
// Layer: tool execution. Dependencies: standard library.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SkillArgs are the arguments for load_skill.
type SkillArgs struct {
	Name string `json:"name"`
}

// LoadSkillFunc resolves and renders a skill body.
type LoadSkillFunc func(name string) (string, string, error)

// LoadSkillTool loads a rendered skill body into the conversation.
type LoadSkillTool struct {
	load LoadSkillFunc
}

// NewLoadSkillTool creates a LoadSkillTool bound to the given body loader.
func NewLoadSkillTool(load LoadSkillFunc) *LoadSkillTool {
	return &LoadSkillTool{load: load}
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
	return "name → load the skill body into the conversation; project scope → use project/name"
}

// Parameters returns the JSON schema for the tool's parameters.
func (t *LoadSkillTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {
				"type": "string",
				"description": "name = skill id; project scope → use project/name"
			}
		},
		"required": ["name"]
	}`)
}

// Execute resolves the skill name and returns its rendered body.
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
	if t.load == nil {
		return "error: skill loader is not configured"
	}
	resolved, body, err := t.load(name)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return fmt.Sprintf("Skill loaded: %s\n\n%s", strings.TrimPrefix(resolved, "global/"), body)
}

// normalizeSkillName strips the optional .md suffix.
func normalizeSkillName(name string) string {
	return strings.TrimSuffix(name, ".md")
}
