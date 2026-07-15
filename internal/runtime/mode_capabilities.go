// mode_capabilities.go — main runtime mode tool and sub-agent policy.
// Applies config/modes.json denied tools and allowed one-shot agent names.
// Layer: runtime configuration. Dependencies: internal/agents, internal/tools.
package runtime

import (
	"fmt"

	"blazeai/internal/agents"
)

// refreshModeCapabilities applies the current mode to direct tools and prompt-visible agents.
// WHAT: Enforces the main runtime policy from config/modes.json.
// HOW: Removes denied direct tools and exposes only explicitly allowed sub-agents.
func (a *Agent) refreshModeCapabilities() error {
	if a.BaseTools == nil {
		return fmt.Errorf("base tool registry is not configured")
	}
	if a.CurrentMode == nil {
		return fmt.Errorf("current mode is not configured")
	}
	denied := make(map[string]bool, len(a.CurrentMode.DeniedTools))
	for _, name := range a.CurrentMode.DeniedTools {
		if a.BaseTools.Get(name) == nil {
			return fmt.Errorf("mode %q denies unknown tool %q", a.CurrentMode.Name, name)
		}
		denied[name] = true
	}
	names := make([]string, 0, len(a.BaseTools.All()))
	for _, tool := range a.BaseTools.All() {
		if !denied[tool.Name()] {
			names = append(names, tool.Name())
		}
	}
	filtered, err := a.BaseTools.Filter(names)
	if err != nil {
		return fmt.Errorf("cannot apply mode %q tool policy: %w", a.CurrentMode.Name, err)
	}
	a.Tools = filtered

	allowedAgents := make(map[string]bool, len(a.CurrentMode.Agents))
	for _, name := range a.CurrentMode.Agents {
		definition, ok := a.oneShotDefinition(name)
		if !ok {
			return fmt.Errorf("mode %q allows unknown one-shot agent %q", a.CurrentMode.Name, name)
		}
		allowedAgents[definition.Name] = true
	}
	a.Builder.Agents = make([]agents.Definition, 0, len(a.CurrentMode.Agents))
	for _, definition := range a.Definitions {
		if allowedAgents[definition.Name] {
			a.Builder.Agents = append(a.Builder.Agents, definition)
		}
	}
	return nil
}

// definitionsNeedRunAgent reports whether a valid child dispatch target exists.
func definitionsNeedRunAgent(definitions []agents.Definition) bool {
	for _, definition := range definitions {
		if definition.Kind == agents.KindOneShot {
			return true
		}
	}
	return false
}

// modeAllowsAgent reports whether the current mode explicitly permits one child agent.
func (a *Agent) modeAllowsAgent(name string) bool {
	if a.CurrentMode == nil {
		return false
	}
	for _, allowed := range a.CurrentMode.Agents {
		if allowed == name {
			return true
		}
	}
	return false
}
