// state.go — Telegram bridge instance state loading and saving.
// Loads `state.json`, validates the selected model against global providers,
// and persists per-instance model changes without touching global config files.
// Layer: transport state. Dependencies: internal/config.
package telegram

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"blazeai/internal/config"
)

const stateFileName = "state.json"

// State holds mutable per-instance Telegram bridge state.
//
// WHAT:  Stores the Telegram instance's selected model.
// WHY:   Telegram model changes must persist locally per instance, not globally.
// PARAMS: SelectedModel — active `provider/model_name` for this instance.
type State struct {
	SelectedModel string `json:"selected_model"`
}

// LoadState loads and validates state.json for one instance.
func LoadState(instance string, cfg *config.Config) (*State, string, error) {
	dir, err := InstanceDir(instance)
	if err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, stateFileName)
	state, err := LoadStateFrom(path, cfg)
	if err != nil {
		return nil, path, err
	}
	return state, path, nil
}

// LoadStateFrom loads and validates state from an explicit path.
func LoadStateFrom(path string, cfg *config.Config) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("telegram state file missing: %s", path)
		}
		return nil, fmt.Errorf("cannot read telegram state file %s: %w", path, err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("cannot parse telegram state file %s: %w", path, err)
	}
	if err := state.Validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid telegram state file %s: %w", path, err)
	}
	return &state, nil
}

// Validate checks the selected model against global provider config.
func (s *State) Validate(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("global config is required")
	}
	modelID := strings.TrimSpace(s.SelectedModel)
	if modelID == "" {
		return fmt.Errorf("selected_model is required")
	}
	providerName, modelName := config.SplitModelID(modelID)
	if providerName == "" || modelName == "" || strings.Contains(modelName, "/") {
		return fmt.Errorf("selected_model must be in provider/model_name format")
	}
	if cfg.ProviderByName(providerName) == nil {
		return fmt.Errorf("selected_model provider not found: %s", providerName)
	}
	return nil

}

// SaveTo writes state.json atomically to an explicit path.
func (s *State) SaveTo(path string, cfg *config.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create telegram state directory %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal telegram state: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("cannot write temp telegram state %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("cannot commit telegram state %s: %w", path, err)
	}
	return nil
}
