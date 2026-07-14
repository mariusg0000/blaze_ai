// agent_capabilities.go — active interactive-agent capability selection.
// Applies explicit Markdown allowlists while preserving legacy modes without definitions.
// Layer: runtime configuration. Dependencies: internal/agents and internal/tools.
package runtime

import (
	"fmt"

	"blazeai/internal/agents"
)

// refreshInteractiveTools applies the definition matching the current mode, if any.
// WHAT: Rebuilds the active registry on mode changes.
// HOW: Filters from BaseTools and removes orchestration from legacy modes.
func (a *Agent) refreshInteractiveTools() error {
	if a.BaseTools == nil {
		return fmt.Errorf("base tool registry is not configured")
	}
	if a.CurrentMode != nil {
		for _, definition := range a.Definitions {
			if definition.Kind != agents.KindInteractive || definition.Name != a.CurrentMode.Name {
				continue
			}
			filtered, err := a.BaseTools.Filter(definition.ToolNames)
			if err != nil {
				return fmt.Errorf("cannot build tools for agent %q: %w", definition.Name, err)
			}
			a.Tools = filtered
			return nil
		}
	}
	a.Tools = a.BaseTools.Clone()
	a.Tools.Remove("run_agent")
	return nil
}
