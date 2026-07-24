// bootstrap.go — provisioning of a user-editable default interactive agent definition.
// Creates a starter default.md on fresh app homes with no interactive definitions
// and no existing agents.json state, without overwriting any existing file.
// Layer: agent definitions. Dependencies: internal/config (model validation).
package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"blazeai/internal/config"
)

const defaultBody = "You are the primary interactive BlazeAI agent. Use your available tools responsibly.\n"

// EnsureDefaultInteractive creates a starter interactive agent definition when none exist.
//
// WHAT:  Provisions a user-editable default.md on fresh app homes.
// HOW:   Checks target existence, validates the model, builds sorted tool/executor
//
//	lists with control tools filtered, generates valid front matter accepted by Load,
//	and writes the file with mode 0600. Never overwrites an existing file.
//
// PARAMS: agentsDir — the agents directory (e.g. ~/.blazeai/agents);
//
//	model — validated provider/model_name identifier;
//	toolNames — unsorted tool names to include (agent_done and run_agent are filtered out);
//	executorNames — unsorted executor names to include when non-empty.
//
// RETURNS: bool — true if the file was created; error if validation or write fails.
func EnsureDefaultInteractive(agentsDir, model string, toolNames, executorNames []string) (bool, error) {
	target := filepath.Join(agentsDir, "default.md")

	// Check whether default.md already exists. Existing definitions are never overwritten.
	_, err := os.Stat(target)
	if err == nil {
		return false, nil
	}
	if !os.IsNotExist(err) {
		return false, err
	}

	// Validate model format before creating any file.
	if err := config.ValidateModelFormat(model); err != nil {
		return false, err
	}

	// Build sorted tool list, filtering control-only tools.
	sortedTools := sortedFilteredTools(toolNames)

	// Build sorted executor list.
	sortedExecutors := make([]string, len(executorNames))
	copy(sortedExecutors, executorNames)
	sort.Strings(sortedExecutors)

	// Generate front matter accepted by agents.Load.
	var buf strings.Builder
	buf.WriteString("---\n")
	buf.WriteString("name: default\n")
	buf.WriteString("description: General-purpose interactive agent\n")
	buf.WriteString("type: interactive\n")
	buf.WriteString("model: " + model + "\n")
	buf.WriteString("tools:\n")
	for _, name := range sortedTools {
		buf.WriteString("  - " + name + "\n")
	}
	if len(sortedExecutors) > 0 {
		buf.WriteString("agents:\n")
		for _, name := range sortedExecutors {
			buf.WriteString("  - " + name + "\n")
		}
	}
	buf.WriteString("---\n")
	buf.WriteString(defaultBody)

	// Create the agents directory if needed, then write the definition.
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return false, fmt.Errorf("cannot create agents directory %s: %w", agentsDir, err)
	}
	if err := os.WriteFile(target, []byte(buf.String()), 0600); err != nil {
		return false, fmt.Errorf("cannot write default agent definition: %w", err)
	}
	return true, nil
}

// sortedFilteredTools returns a sorted copy of toolNames with agent_done and run_agent removed.
// WHAT: Produces a deterministic tool allowlist for the generated default definition.
// HOW: Copies, filters, sorts. Does not mutate the caller's slice.
func sortedFilteredTools(toolNames []string) []string {
	filtered := make([]string, 0, len(toolNames))
	for _, name := range toolNames {
		if name == "agent_done" || name == "run_agent" {
			continue
		}
		filtered = append(filtered, name)
	}
	sort.Strings(filtered)
	return filtered
}
