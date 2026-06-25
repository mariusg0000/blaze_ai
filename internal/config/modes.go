// modes.go — work modes configuration: load, save, validate, migration.
// Separated from config.json to isolate the frequently-edited modes list from
// the sensitive provider/API key data. Uses atomic temp-file writes for safety.
// Layer: configuration. Dependencies: internal/platform (app home path resolution).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"blazeai/internal/platform"
)

// ErrModesMissing is returned when modes.json does not exist.
var ErrModesMissing = errors.New("modes file missing")

// ModesConfig holds the work modes list and persisted active mode name.
//
// WHAT:  Self-contained modes configuration with its own file, load, and save logic.
// WHY:   Separating modes from config.json isolates frequently-edited data from
//        critical provider/role configuration, reducing corruption risk.
// PARAMS: Modes — work mode definitions; LastMode — persisted active mode name.
type ModesConfig struct {
	Modes    []Mode `json:"modes"`
	LastMode string `json:"last_mode,omitempty"`
}

// modesPath resolves the full path to modes.json under app home.
func modesPath() (string, error) {
	home, err := platform.AppHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "config", "modes.json"), nil
}

// DefaultMode creates a single-element ModesConfig with the given model as default.
//
// WHAT:  Returns a ModesConfig with one "default" mode pointing to modelID.
// WHY:   When no modes exist (first start, migration failure, or corruption),
//        the runtime needs at least one mode to function.
// PARAMS: modelID — provider/model_name for the default mode.
// RETURNS: *ModesConfig — minimal valid modes config.
func DefaultMode(modelID string) *ModesConfig {
	return &ModesConfig{
		Modes: []Mode{
			{Name: "default", Model: modelID},
		},
		LastMode: "default",
	}
}

// LoadModes reads modes.json from app_home/config/.
// If the file does not exist, is empty, or is corrupted, returns a fallback
// ModesConfig with a single default mode using the given defaultModel.
//
// WHAT:  Loads modes configuration with graceful fallback on corruption.
// WHY:   A corrupted modes.json may result from LLM editing errors; the runtime
//        should never crash on invalid modes — it uses a safe default instead.
// PARAMS: defaultModel — provider/model_name for the fallback default mode.
// RETURNS: *ModesConfig — loaded or fallback modes; error if app home resolution fails.
func LoadModes(defaultModel string) (*ModesConfig, error) {
	path, err := modesPath()
	if err != nil {
		return DefaultMode(defaultModel), nil
	}
	return LoadModesFrom(path, defaultModel)
}

// LoadModesFrom reads modes from a specific path. See LoadModes for behavior.
func LoadModesFrom(path string, defaultModel string) (*ModesConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultMode(defaultModel), nil
	}
	var mc ModesConfig
	if err := json.Unmarshal(data, &mc); err != nil {
		return DefaultMode(defaultModel), nil
	}
	if len(mc.Modes) == 0 {
		return DefaultMode(defaultModel), nil
	}
	if err := mc.validateBasic(); err != nil {
		return DefaultMode(defaultModel), nil
	}
	return &mc, nil
}

// Validate checks modes for structural integrity against known providers.
//
// WHAT:  Full validation of mode definitions against provider data.
// PARAMS: providerNames — set of valid provider names from config.
// RETURNS: error if any mode has duplicate names, invalid models, or missing providers.
func (m *ModesConfig) Validate(providerNames map[string]bool) error {
	if err := validateModes(m.Modes, providerNames); err != nil {
		return err
	}
	if m.LastMode != "" {
		if err := validateLastMode(m.LastMode, m.Modes); err != nil {
			return err
		}
	}
	return nil
}

// validateBasic checks modes for correctness without needing provider data.
func (m *ModesConfig) validateBasic() error {
	seen := make(map[string]bool, len(m.Modes))
	for _, mode := range m.Modes {
		if mode.Name == "" {
			return fmt.Errorf("mode name is empty")
		}
		if seen[mode.Name] {
			return fmt.Errorf("%w: %s", ErrDuplicateModeName, mode.Name)
		}
		seen[mode.Name] = true
		if mode.Model == "" {
			return fmt.Errorf("mode %q: model is empty", mode.Name)
		}
		if err := validateModelFormat(mode.Model); err != nil {
			return fmt.Errorf("mode %q: %w: %s", mode.Name, ErrModeModelInvalid, mode.Model)
		}
	}
	return nil
}

// Save writes modes to app_home/config/modes.json with atomic temp-file pattern.
//
// WHAT:  Persists modes atomically: write to .tmp, validate JSON, rename.
// WHY:   An atomic write prevents corruption if the process crashes mid-write.
//        Coupled with the fallback in LoadModes, a corrupted modes.json is never fatal.
// RETURNS: error if marshaling, writing, or renaming fails.
func (m *ModesConfig) Save() error {
	path, err := modesPath()
	if err != nil {
		return err
	}
	return m.SaveTo(path)
}

// SaveTo writes modes to a specific path with atomic temp-file pattern.
func (m *ModesConfig) SaveTo(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create modes directory %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal modes: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("cannot write temp modes file: %w", err)
	}
	verifyData, err := os.ReadFile(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("cannot verify temp modes file: %w", err)
	}
	var verify ModesConfig
	if err := json.Unmarshal(verifyData, &verify); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("temp modes file invalid JSON: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("cannot commit modes file: %w", err)
	}
	return nil
}

// MigrateFromConfig reads legacy modes from config.json, saves them to modes.json,
// strips modes and last_mode from config.json, and saves the cleaned config.
//
// WHAT:  One-time migration from config.json modes field to modes.json.
// WHY:   Modes were previously stored in config.json; this extracts them safely.
// HOW:   Reads the raw config JSON, extracts modes/last_mode if present, writes
//        modes.json, strips them from config.json.
// RETURNS: error if config read, modes save, or config strip fails.
func MigrateFromConfig() error {
	cfgPath, err := configPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("cannot parse config: %w", err)
	}
	// Check if modes field exists.
	modesJSON, hasModes := raw["modes"]
	if !hasModes {
		return nil // Nothing to migrate.
	}
	// Build the modes config from raw fields.
	var modes []Mode
	if err := json.Unmarshal(modesJSON, &modes); err != nil {
		return fmt.Errorf("cannot parse legacy modes: %w", err)
	}
	if len(modes) == 0 {
		return nil
	}
	lastMode := ""
	if rawLM, ok := raw["last_mode"]; ok {
		json.Unmarshal(rawLM, &lastMode)
	}
	mc := &ModesConfig{
		Modes:    modes,
		LastMode: lastMode,
	}
	if err := mc.Save(); err != nil {
		return fmt.Errorf("cannot save migrated modes: %w", err)
	}
	delete(raw, "modes")
	delete(raw, "last_mode")
	cleaned, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal cleaned config: %w", err)
	}
	if err := os.WriteFile(cfgPath, cleaned, 0600); err != nil {
		return fmt.Errorf("cannot write cleaned config: %w", err)
	}
	return nil
}
