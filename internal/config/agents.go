// agents.go — Persistent interactive-agent state: load, save, validate.
// Stores selected model and active agent name for interactive agents.
// Layer: configuration. Dependencies: internal/platform (app home path resolution).
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// InteractiveAgentState holds the persisted model selection for one interactive agent.
type InteractiveAgentState struct {
	Name  string `json:"name"`
	Model string `json:"model"`
}

// AgentsConfig holds the list of interactive agent states and the last active agent name.
type AgentsConfig struct {
	Agents    []InteractiveAgentState `json:"agents"`
	LastAgent string                  `json:"last_agent,omitempty"`
	path      string
}

// agentsPath resolves the full path to agents.json under app home.
func agentsPath(appHome string) string {
	return filepath.Join(appHome, "config", "agents.json")
}

// LoadAgents reads agents.json from <appHome>/config/agents.json.
// WHAT: Loads the interactive agent state from disk.
// HOW: Delegates to LoadAgentsFrom with the default path.
func LoadAgents(appHome string) (*AgentsConfig, error) {
	return LoadAgentsFrom(agentsPath(appHome))
}

// LoadAgentsFrom reads agents.json from a specific path.
// WHAT: Loads interactive agent state without relying on app home.
// HOW: Reads JSON, unmarshals into AgentsConfig, rejects missing/corrupt files.
func LoadAgentsFrom(path string) (*AgentsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("agents config file missing: %s", path)
		}
		return nil, fmt.Errorf("cannot read agents config %s: %w", path, err)
	}
	var ac AgentsConfig
	if err := json.Unmarshal(data, &ac); err != nil {
		return nil, fmt.Errorf("cannot parse agents config %s: %w", path, err)
	}
	ac.path = path
	return &ac, nil
}

// Save writes the agents config to the path resolved from the stored app home.
// WHAT: Persists the interactive agent state to disk.
// HOW: Delegates to SaveTo with the stored path.
func (c *AgentsConfig) Save() error {
	if c.path == "" {
		return fmt.Errorf("agents config has no path; use SaveTo instead")
	}
	return c.SaveTo(c.path)
}

// SaveTo writes the agents config to a specific path with atomic temp-file pattern.
// WHAT: Atomically persists interactive agent state.
// HOW: Write to .tmp, validate JSON by re-reading, rename to final path.
func (c *AgentsConfig) SaveTo(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create agents config directory %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal agents config: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("cannot write temp agents config: %w", err)
	}
	verifyData, err := os.ReadFile(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("cannot verify temp agents config: %w", err)
	}
	var verify AgentsConfig
	if err := json.Unmarshal(verifyData, &verify); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("temp agents config invalid JSON: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("cannot commit agents config: %w", err)
	}
	c.path = path
	return nil
}

// Validate checks the agents config for structural integrity.
// WHAT: Rejects duplicate/empty names, invalid models, and empty LastAgent when states exist.
// HOW: Iterates agents for unique non-empty names, validates non-empty models via
//
//	config.ValidateModelFormat, and checks LastAgent consistency.
func (c *AgentsConfig) Validate(providerNames []string) error {
	seen := make(map[string]bool, len(c.Agents))
	for _, a := range c.Agents {
		if a.Name == "" {
			return fmt.Errorf("agent state name is empty")
		}
		if seen[a.Name] {
			return fmt.Errorf("duplicate agent state name %q", a.Name)
		}
		seen[a.Name] = true
		if a.Model != "" {
			if err := ValidateModelFormat(a.Model); err != nil {
				return fmt.Errorf("agent %q: invalid model %q: %w", a.Name, a.Model, err)
			}
		}
	}
	if len(c.Agents) > 0 && c.LastAgent == "" {
		return fmt.Errorf("last_agent is required when agents are present")
	}
	return nil
}
