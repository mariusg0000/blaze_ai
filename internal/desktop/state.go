// state.go — desktop companion state loading and saving.
// Loads app_home/desktop/state.json, validates the selected model against the
// global provider config, and persists desktop-local model changes.
// Layer: transport state. Dependencies: internal/config.
package desktop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"blazeai/internal/config"
)

const stateFileName = "state.json"

// State holds mutable local desktop transport state.
//
// WHAT:  Stores the selected model for the desktop transport.
// WHY:   Desktop model changes should stay local to this transport instance.
// PARAMS: SelectedModel — active provider/model_name for the desktop app.
type State struct {
	SelectedModel string `json:"selected_model"`
}

// LoadState loads and validates the singleton desktop state.
//
// WHAT:  Reads app_home/desktop/state.json.
// WHY:   Desktop startup must fail fast on missing or invalid local state.
// PARAMS: cfg — loaded global runtime config used to validate providers.
// RETURNS: *State — validated state; string — state path; error on failure.
func LoadState(cfg *config.Config) (*State, string, error) {
	dir, err := InstanceDir()
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

// LoadStateFrom loads and validates desktop state from an explicit file path.
//
// WHAT:  Reads one desktop state file from disk.
// WHY:   Tests and startup share the same validation logic.
// PARAMS: path — absolute state file path; cfg — loaded global runtime config.
// RETURNS: *State — validated state; error on read, parse, or validation failure.
func LoadStateFrom(path string, cfg *config.Config) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("desktop state file missing: %s", path)
		}
		return nil, fmt.Errorf("cannot read desktop state file %s: %w", path, err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("cannot parse desktop state file %s: %w", path, err)
	}
	if err := state.Validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid desktop state file %s: %w", path, err)
	}
	return &state, nil
}

// Validate checks the selected model against global providers.
//
// WHAT:  Validates the desktop-local selected model.
// WHY:   The transport must stop instead of silently falling back to another model.
// PARAMS: cfg — loaded global runtime config.
// RETURNS: error if the selected model is missing or invalid.
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
//
// WHAT:  Persists desktop-local selected model changes.
// WHY:   UI model switches must survive restarts without touching global config.
// PARAMS: path — absolute state file path; cfg — loaded global runtime config.
// RETURNS: error if validation or persistence fails.
func (s *State) SaveTo(path string, cfg *config.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create desktop state directory %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal desktop state: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("cannot write temp desktop state %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("cannot commit desktop state %s: %w", path, err)
	}
	return nil
}
