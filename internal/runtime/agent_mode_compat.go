// agent_mode_compat.go — compatibility conversion from work modes to agents.
// Represents existing modes as in-memory interactive definitions during migration.
// Layer: runtime compatibility. Dependencies: internal/agents and internal/config.
package runtime

import (
	"fmt"

	"blazeai/internal/agents"
	"blazeai/internal/config"
	"blazeai/internal/tools"
)

// addModeAgentDefinitions converts every persisted mode into an explicit interactive definition.
// WHAT: Keeps quick/default/planning visible and executable during migration.
// HOW: Copies each mode model and gives it the complete legacy registry allowlist.
func addModeAgentDefinitions(definitions []agents.Definition, modes *config.ModesConfig, registry *tools.Registry) ([]agents.Definition, error) {
	seen := make(map[string]bool, len(definitions))
	for _, definition := range definitions {
		seen[definition.Name] = true
	}
	names := make([]string, 0, len(registry.All()))
	for _, tool := range registry.All() {
		names = append(names, tool.Name())
	}
	for _, mode := range modes.Modes {
		if seen[mode.Name] {
			return nil, fmt.Errorf("agent name %q conflicts with an existing work mode", mode.Name)
		}
		definitions = append(definitions, agents.Definition{
			Name:         mode.Name,
			Description:  "Compatibility interactive work mode.",
			Kind:         agents.KindInteractive,
			Model:        mode.Model,
			ToolNames:    append([]string(nil), names...),
			Instructions: mode.Directive,
		})
		seen[mode.Name] = true
	}
	return definitions, nil
}
