// agents.go — Markdown agent definition discovery, parsing, and validation.
// Defines the agent metadata model and strict loader for app-home agents.
// Layer: agent definitions. Dependencies: internal/config and internal/tools.
package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"blazeai/internal/config"
	"blazeai/internal/tools"
)

// Type identifies the execution role of a Markdown agent definition.
type Type string

const (
	// TypeInteractive identifies a main-session agent selectable by the user.
	TypeInteractive Type = "interactive"
	// TypeExecutor identifies an ephemeral delegated child agent.
	TypeExecutor Type = "executor"
)

// Definition is the validated, executable description of one Markdown agent.
type Definition struct {
	Name          string
	Description   string
	Type          Type
	Model         string
	ToolNames     []string
	Timeout       time.Duration
	Instructions  string
	Path          string
	Directive     string
	ExecutorNames []string
}

// Load discovers and validates only Markdown files directly inside dir.
// WHAT: Loads all user-defined agents and rejects invalid definitions.
// HOW: Sorts direct .md entries, parses front matter, then validates names,
//
//	models, tools, and cross-definition executor references.
func Load(dir string, registry *tools.Registry) ([]Definition, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Definition{}, nil
		}
		return nil, fmt.Errorf("cannot read agents directory %s: %w", dir, err)
	}
	paths := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(paths)
	result := make([]Definition, 0, len(paths))
	seen := make(map[string]string)
	for _, path := range paths {
		definition, err := ParseFile(path)
		if err != nil {
			return nil, err
		}
		if previous, exists := seen[definition.Name]; exists {
			return nil, fmt.Errorf("duplicate agent name %q in %s and %s", definition.Name, previous, path)
		}
		seen[definition.Name] = path
		if err := validateDefinition(definition, registry); err != nil {
			return nil, fmt.Errorf("agent %q (%s): %w", definition.Name, path, err)
		}
		result = append(result, definition)
	}
	// Validate cross-definition executor references for interactive definitions.
	if err := validateExecutorReferences(result); err != nil {
		return nil, err
	}
	return result, nil
}

// ParseFile parses one agent Markdown file using the documented front-matter format.
// WHAT: Converts one Markdown file into an agent Definition.
// HOW: Requires a YAML-like block delimited by the first and closing --- lines; body is preserved.
func ParseFile(path string) (Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Definition{}, fmt.Errorf("cannot read agent definition %s: %w", path, err)
	}
	definition, err := parse(string(data))
	if err != nil {
		return Definition{}, fmt.Errorf("cannot parse agent definition %s: %w", path, err)
	}
	definition.Path = path
	return definition, nil
}

// parse parses front matter and retains the Markdown body as instructions.
// WHAT: Converts front-matter text and Markdown body into a Definition.
// HOW: Iterates key: value lines and YAML-style list items under tools/agents context.
func parse(content string) (Definition, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return Definition{}, fmt.Errorf("missing opening front matter delimiter")
	}
	end := strings.Index(content[4:], "\n---\n")
	if end < 0 {
		return Definition{}, fmt.Errorf("missing closing front matter delimiter")
	}
	end += 4
	meta, body := content[4:end], content[end+5:]
	var definition Definition
	var currentList string
	seen := make(map[string]bool)
	for lineNumber, raw := range strings.Split(meta, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "-") {
			if currentList != "tools" && currentList != "agents" {
				return Definition{}, fmt.Errorf("line %d: list item outside tools or agents", lineNumber+1)
			}
			name := strings.TrimSpace(strings.TrimPrefix(line, "-"))
			if name == "" {
				if currentList == "tools" {
					return Definition{}, fmt.Errorf("line %d: empty tool name", lineNumber+1)
				}
				return Definition{}, fmt.Errorf("line %d: empty agent name", lineNumber+1)
			}
			if currentList == "tools" {
				definition.ToolNames = append(definition.ToolNames, name)
			} else {
				definition.ExecutorNames = append(definition.ExecutorNames, name)
			}
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return Definition{}, fmt.Errorf("line %d: expected key: value", lineNumber+1)
		}
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if seen[key] {
			return Definition{}, fmt.Errorf("duplicate metadata key %q", key)
		}
		seen[key] = true
		currentList = ""
		switch key {
		case "name":
			definition.Name = value
		case "description":
			definition.Description = value
		case "type":
			definition.Type = Type(value)
		case "kind":
			return Definition{}, fmt.Errorf("unknown metadata key %q; use type instead of kind", key)
		case "model":
			definition.Model = value
		case "timeout":
			d, err := time.ParseDuration(value)
			if err != nil {
				return Definition{}, fmt.Errorf("line %d: invalid timeout %q: %w", lineNumber+1, value, err)
			}
			definition.Timeout = d
		case "directive":
			definition.Directive = value
		case "tools":
			currentList = "tools"
			if value != "" {
				for _, item := range strings.Split(value, ",") {
					item = strings.TrimSpace(item)
					if item == "" {
						return Definition{}, fmt.Errorf("empty tool name")
					}
					definition.ToolNames = append(definition.ToolNames, item)
				}
			}
		case "agents":
			currentList = "agents"
			if value != "" {
				for _, item := range strings.Split(value, ",") {
					item = strings.TrimSpace(item)
					if item == "" {
						return Definition{}, fmt.Errorf("empty agent name")
					}
					definition.ExecutorNames = append(definition.ExecutorNames, item)
				}
			}
		default:
			return Definition{}, fmt.Errorf("unknown metadata key %q", key)
		}
	}
	definition.Instructions = strings.TrimSpace(body)
	return definition, nil
}

// validateDefinition applies structural and registry-dependent validation.
// WHAT: Checks name, description, type, model, tools, timeout, and tool registry.
// HOW: Enforces type-specific rules for interactive and executor definitions.
func validateDefinition(definition Definition, registry *tools.Registry) error {
	if strings.TrimSpace(definition.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(definition.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if definition.Type == "" {
		return fmt.Errorf("type is required")
	}
	if definition.Type != TypeInteractive && definition.Type != TypeExecutor {
		return fmt.Errorf("unsupported type %q; must be %q or %q", definition.Type, TypeInteractive, TypeExecutor)
	}
	if definition.Model != "" {
		if err := config.ValidateModelFormat(definition.Model); err != nil {
			return fmt.Errorf("invalid model %q: %w", definition.Model, err)
		}
	}
	if definition.Timeout != 0 && definition.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	// Type-specific model requirement.
	if definition.Type == TypeInteractive && definition.Model == "" {
		return fmt.Errorf("interactive agent requires a model")
	}
	// Executor definitions may not have directive or agents.
	if definition.Type == TypeExecutor {
		if definition.Directive != "" {
			return fmt.Errorf("executor agent must not have a directive")
		}
		if len(definition.ExecutorNames) > 0 {
			return fmt.Errorf("executor agent must not list agents")
		}
	}
	if len(definition.ToolNames) == 0 {
		return fmt.Errorf("tools allowlist is required and cannot be empty")
	}
	seen := make(map[string]bool, len(definition.ToolNames))
	for _, name := range definition.ToolNames {
		if seen[name] {
			return fmt.Errorf("duplicate tool %q", name)
		}
		seen[name] = true
		if name == "agent_done" {
			continue
		}
		if name == "run_agent" {
			if definition.Type == TypeExecutor {
				return fmt.Errorf("executor agents cannot use run_agent")
			}
			continue
		}
		if registry == nil || registry.Get(name) == nil {
			return fmt.Errorf("unknown tool %q", name)
		}
	}
	return nil
}

// validateExecutorReferences checks that every executor name referenced by an
// interactive definition resolves to a loaded TypeExecutor definition.
// WHAT: Ensures no dangling executor references across all loaded definitions.
// HOW: Builds a name→type index and checks each ExecutorNames entry.
func validateExecutorReferences(defs []Definition) error {
	typeIndex := make(map[string]Type, len(defs))
	for _, d := range defs {
		typeIndex[d.Name] = d.Type
	}
	for _, d := range defs {
		if d.Type != TypeInteractive {
			continue
		}
		seen := make(map[string]bool, len(d.ExecutorNames))
		for _, ref := range d.ExecutorNames {
			if seen[ref] {
				return fmt.Errorf("agent %q: duplicate executor reference %q", d.Name, ref)
			}
			seen[ref] = true
			t, ok := typeIndex[ref]
			if !ok {
				return fmt.Errorf("agent %q: unknown executor reference %q", d.Name, ref)
			}
			if t != TypeExecutor {
				return fmt.Errorf("agent %q: executor reference %q is not an executor", d.Name, ref)
			}
		}
	}
	return nil
}
