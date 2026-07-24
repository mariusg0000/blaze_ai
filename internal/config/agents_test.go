// agents_test.go — tests for interactive agent state persistence and validation.
// Covers JSON round-trip, atomic save, duplicate/empty names, invalid models,
// and missing-file errors.
// Layer: configuration. Dependencies: none beyond standard library.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeAgentsConfig(t *testing.T, ac *AgentsConfig) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "agents.json")
	if err := ac.SaveTo(path); err != nil {
		t.Fatalf("SaveTo() failed: %v", err)
	}
	return path
}

func TestAgentsConfigSaveLoadRoundTrip(t *testing.T) {
	ac := &AgentsConfig{
		Agents: []InteractiveAgentState{
			{Name: "planner", Model: "openai/gpt-4.1"},
			{Name: "default", Model: "openai/gpt-4o"},
		},
		LastAgent: "planner",
	}
	path := writeAgentsConfig(t, ac)

	loaded, err := LoadAgentsFrom(path)
	if err != nil {
		t.Fatalf("LoadAgentsFrom() error: %v", err)
	}
	if len(loaded.Agents) != 2 {
		t.Fatalf("Agents = %d, want 2", len(loaded.Agents))
	}
	if loaded.Agents[0].Name != "planner" || loaded.Agents[0].Model != "openai/gpt-4.1" {
		t.Errorf("Agents[0] = %+v", loaded.Agents[0])
	}
	if loaded.Agents[1].Name != "default" || loaded.Agents[1].Model != "openai/gpt-4o" {
		t.Errorf("Agents[1] = %+v", loaded.Agents[1])
	}
	if loaded.LastAgent != "planner" {
		t.Errorf("LastAgent = %q, want 'planner'", loaded.LastAgent)
	}
}

func TestAgentsConfigSaveIsAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "agents.json")
	ac := &AgentsConfig{
		Agents:    []InteractiveAgentState{{Name: "x", Model: "openai/gpt-4o"}},
		LastAgent: "x",
	}
	if err := ac.SaveTo(path); err != nil {
		t.Fatalf("SaveTo() failed: %v", err)
	}
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Errorf("tmp file %s should not exist after commit", tmpPath)
	}
	loaded, err := LoadAgentsFrom(path)
	if err != nil {
		t.Fatalf("LoadAgentsFrom() failed: %v", err)
	}
	if len(loaded.Agents) != 1 {
		t.Errorf("Agents = %d, want 1", len(loaded.Agents))
	}
}

func TestAgentsConfigValidate(t *testing.T) {
	tests := []struct {
		name string
		ac   *AgentsConfig
		want string
	}{
		{
			"duplicate-names",
			&AgentsConfig{
				Agents: []InteractiveAgentState{
					{Name: "a", Model: "openai/gpt-4o"},
					{Name: "a", Model: "openai/gpt-4o"},
				},
				LastAgent: "a",
			},
			"duplicate agent state name",
		},
		{
			"empty-name",
			&AgentsConfig{
				Agents:    []InteractiveAgentState{{Name: "", Model: "openai/gpt-4o"}},
				LastAgent: "",
			},
			"agent state name is empty",
		},
		{
			"empty-last-agent",
			&AgentsConfig{
				Agents:    []InteractiveAgentState{{Name: "a", Model: "openai/gpt-4o"}},
				LastAgent: "",
			},
			"last_agent is required when agents are present",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ac.Validate(nil)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestAgentsConfigRejectsInvalidModel(t *testing.T) {
	ac := &AgentsConfig{
		Agents:    []InteractiveAgentState{{Name: "a", Model: "no-slash"}},
		LastAgent: "a",
	}
	err := ac.Validate(nil)
	if err == nil || !strings.Contains(err.Error(), "invalid model") {
		t.Fatalf("Validate() error = %v, want substring %q", err, "invalid model")
	}
}

func TestLoadAgentsMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "agents.json")
	_, err := LoadAgentsFrom(path)
	if err == nil {
		t.Fatal("LoadAgentsFrom() expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "agents config file missing") {
		t.Fatalf("LoadAgentsFrom() error = %v, want 'agents config file missing'", err)
	}
}

func TestAgentsConfigEmptyAgentsValid(t *testing.T) {
	ac := &AgentsConfig{
		Agents: []InteractiveAgentState{},
	}
	// Empty agents list with empty LastAgent is valid.
	if err := ac.Validate(nil); err != nil {
		t.Fatalf("Validate() unexpected error for empty agents: %v", err)
	}
}

func TestAgentsConfigJSONOmitEmpty(t *testing.T) {
	ac := &AgentsConfig{
		Agents:    []InteractiveAgentState{{Name: "a", Model: "openai/gpt-4o"}},
		LastAgent: "a",
	}
	data, err := json.Marshal(ac)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}
	// LastAgent should not be omitted when non-empty.
	if !strings.Contains(string(data), "last_agent") {
		t.Fatalf("JSON missing last_agent field")
	}

	// Test with empty LastAgent — should be omitted.
	ac2 := &AgentsConfig{
		Agents: []InteractiveAgentState{{Name: "a", Model: "openai/gpt-4o"}},
	}
	data2, err := json.Marshal(ac2)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}
	if strings.Contains(string(data2), "last_agent") {
		t.Fatalf("JSON should omit empty last_agent field")
	}
}
