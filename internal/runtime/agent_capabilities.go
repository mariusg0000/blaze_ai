// agent_capabilities.go — interactive-agent tool and executor allowlist.
// Applies the current interactive agent's ToolNames and ExecutorNames to the runtime.
// Layer: runtime configuration. Dependencies: internal/agents, internal/tools.
package runtime

import (
	"fmt"

	"blazeai/internal/agents"
)

// refreshAgentCapabilities applies the current interactive agent's tool and executor
// allowlists to the runtime tool registry and prompt-visible agent definitions.
//
// WHAT: Filters BaseTools to only ToolNames allowed by the current interactive agent,
// determines which executor definitions are callable, and adds run_agent automatically
// when at least one executor is allowed.
// HOW: Builds a name set from CurrentAgent.ToolNames, filters BaseTools, then
// intersects CurrentAgent.ExecutorNames with loaded TypeExecutor definitions to
// populate Builder.Agents and conditionally register run_agent.
func (a *Agent) refreshAgentCapabilities() error {
	if a.BaseTools == nil {
		return fmt.Errorf("base tool registry is not configured")
	}
	if a.CurrentAgent == nil {
		return fmt.Errorf("current agent is not configured")
	}

	// Build the set of allowed executor names for this interactive agent.
	allowedExecutors := make(map[string]bool, len(a.CurrentAgent.ExecutorNames))
	for _, name := range a.CurrentAgent.ExecutorNames {
		allowedExecutors[name] = true
	}

	// Resolve executor definitions that the current agent is allowed to invoke.
	a.Builder.Agents = make([]agents.Definition, 0, len(a.CurrentAgent.ExecutorNames))
	for _, definition := range a.Definitions {
		if definition.Type == agents.TypeExecutor && allowedExecutors[definition.Name] {
			a.Builder.Agents = append(a.Builder.Agents, definition)
		}
	}

	// Filter base tools by the interactive agent's allowed tool names.
	// If at least one executor is allowed, automatically include run_agent.
	names := make([]string, 0, len(a.CurrentAgent.ToolNames)+1)
	seen := make(map[string]bool, len(a.CurrentAgent.ToolNames)+1)
	for _, name := range a.CurrentAgent.ToolNames {
		if a.BaseTools.Get(name) == nil {
			return fmt.Errorf("agent %q allows unknown tool %q", a.CurrentAgent.Name, name)
		}
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	if len(a.Builder.Agents) > 0 && !seen["run_agent"] && a.BaseTools.Get("run_agent") != nil {
		names = append(names, "run_agent")
	}
	filtered, err := a.BaseTools.Filter(names)
	if err != nil {
		return fmt.Errorf("cannot apply agent %q tool policy: %w", a.CurrentAgent.Name, err)
	}
	a.Tools = filtered

	return nil
}

// definitionsNeedRunAgent reports whether a valid child dispatch target exists.
// WHAT: Returns true if any loaded definition is a TypeExecutor that could be dispatched.
// HOW: Iterates all definitions checking for TypeExecutor.
func definitionsNeedRunAgent(definitions []agents.Definition) bool {
	for _, definition := range definitions {
		if definition.Type == agents.TypeExecutor {
			return true
		}
	}
	return false
}

// interactiveAllowsExecutor reports whether the current interactive agent permits
// invoking the named executor definition.
//
// WHAT: Gates run_agent task dispatch against the active agent's ExecutorNames.
// HOW: Returns true only when CurrentAgent is non-nil and its ExecutorNames contains
// the exact name.
func (a *Agent) interactiveAllowsExecutor(name string) bool {
	if a.CurrentAgent == nil {
		return false
	}
	for _, allowed := range a.CurrentAgent.ExecutorNames {
		if allowed == name {
			return true
		}
	}
	return false
}
