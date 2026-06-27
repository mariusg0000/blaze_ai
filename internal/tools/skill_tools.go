// skill_tools.go — load_skill, unload_skill, and run_skill tool implementations.
// These tools modify the in-memory active skills list or execute runnable skill code.
// Layer: tool execution. Dependencies: internal/platform, internal/skills.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"blazeai/internal/platform"
	"blazeai/internal/skills"
)

// SkillArgs are the arguments for load_skill and unload_skill.
type SkillArgs struct {
	Name string `json:"name"`
}

// ResolveFunc resolves a skill name to a canonical skill ID.
// Returns an error if the name is not found or ambiguous.
type ResolveFunc func(name string) (string, error)

// ResolveSkillFunc resolves a skill name to its canonical ID and parsed skill.
type ResolveSkillFunc func(name string) (string, *skills.Skill, error)

// RunSkillArgs are the arguments for run_skill.
type RunSkillArgs struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
	Timeout   *int   `json:"timeout,omitempty"`
}

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
	return "name → load skill into active session; project scope → use project/name"
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
	return "name → unload skill from active session"
}

// Parameters returns the JSON schema for the tool's parameters.
func (t *UnloadSkillTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {
				"type": "string",
				"description": "name = skill id to unload"
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

// RunSkillTool executes a runnable skill's [CODE] section with raw arguments.
type RunSkillTool struct {
	os      platform.OS
	resolve ResolveSkillFunc
	workDir func() string
}

// NewRunSkillTool creates a RunSkillTool for the given OS, resolver, and workdir accessor.
func NewRunSkillTool(os platform.OS, resolve ResolveSkillFunc, workDir func() string) *RunSkillTool {
	return &RunSkillTool{os: os, resolve: resolve, workDir: workDir}
}

// Name returns the tool's unique identifier.
func (t *RunSkillTool) Name() string {
	return "run_skill"
}

// FormatArgs returns a compact UI label for runnable skill execution.
func (t *RunSkillTool) FormatArgs(args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[RunSkillArgs](args)
	if err != nil {
		return "Running skill"
	}
	if parsed.Name == "" {
		return "Running skill"
	}
	label := "Running skill: " + normalizeSkillName(parsed.Name)
	if strings.TrimSpace(parsed.Arguments) != "" {
		label += " " + strings.TrimSpace(parsed.Arguments)
	}
	return truncateDisplay(label, 80)
}

// Description returns the human-readable description for the LLM.
func (t *RunSkillTool) Description() string {
	return "name + arguments → execute runnable skill"
}

// Parameters returns the JSON schema for the tool's parameters.
func (t *RunSkillTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {
				"type": "string",
				"description": "name = runnable skill id; project scope → use project/name"
			},
			"arguments": {
				"type": "string",
				"description": "arguments = raw string"
			},
			"timeout": {
				"type": "integer",
				"description": "timeout = seconds; optional = true; default = 60"
			}
		},
		"required": ["name"]
	}`)
}

// Execute resolves the skill and runs its code if it is runnable in v1.
func (t *RunSkillTool) Execute(ctx context.Context, args json.RawMessage) string {
	if ctx != nil && ctx.Err() != nil {
		return "aborted before execution by user"
	}
	parsed, err := ParseToolCallArgs[RunSkillArgs](args)
	if err != nil {
		return fmt.Sprintf("error: invalid arguments: %v", err)
	}
	if parsed.Name == "" {
		return "error: name is required"
	}
	if t.resolve == nil {
		return "error: runnable skill resolver is not configured"
	}

	resolved, skill, err := t.resolve(normalizeSkillName(parsed.Name))
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if strings.TrimSpace(skill.Syntax) == "" {
		return fmt.Sprintf("error: skill is not runnable: missing [SYNTAX]: %s", strings.TrimPrefix(resolved, "global/"))
	}
	if skill.CodeError != "" {
		return fmt.Sprintf("error: invalid [CODE] in skill %s: %s", strings.TrimPrefix(resolved, "global/"), skill.CodeError)
	}
	if strings.TrimSpace(skill.Code) == "" {
		return fmt.Sprintf("error: skill is not runnable: missing [CODE]: %s", strings.TrimPrefix(resolved, "global/"))
	}
	if skill.CodeLang != "shell" {
		return fmt.Sprintf("error: unsupported [CODE] language for skill %s: %s", strings.TrimPrefix(resolved, "global/"), skill.CodeLang)
	}

	timeoutSec := DefaultTimeout
	if parsed.Timeout != nil && *parsed.Timeout > 0 {
		timeoutSec = *parsed.Timeout
	}

	return executeShell(ctx, t.os, skill.Code, t.currentWorkDir(), map[string]string{
		"BLAZE_SKILL_ARGS": parsed.Arguments,
		"BLAZE_SKILL_DIR":  skill.Dir,
		"BLAZE_SKILL_ID":   resolved,
		"BLAZE_SKILL_NAME": strings.TrimPrefix(resolved, "global/"),
	}, timeoutSec)
}

// currentWorkDir returns the agent workdir for runnable skill execution.
func (t *RunSkillTool) currentWorkDir() string {
	if t.workDir == nil {
		return ""
	}
	return t.workDir()
}
