// modes_test.go — tests for modes loading, saving, validation, reload, and migration.
package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeModes writes a ModesConfig to a temp file and returns the path.
func writeModes(t *testing.T, mc *ModesConfig) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "modes.json")
	if err := mc.SaveTo(path); err != nil {
		t.Fatalf("SaveTo() failed: %v", err)
	}
	return path
}

func validProviders() map[string]bool {
	return map[string]bool{"openrouter": true}
}

// TestDefaultMode verifies the default mode fallback.
func TestDefaultMode(t *testing.T) {
	mc := DefaultMode("openrouter/test-model")
	if len(mc.Modes) != 1 {
		t.Fatalf("Modes = %d, want 1", len(mc.Modes))
	}
	if mc.Modes[0].Name != "default" {
		t.Errorf("Name = %q, want 'default'", mc.Modes[0].Name)
	}
	if mc.Modes[0].Model != "openrouter/test-model" {
		t.Errorf("Model = %q, want 'openrouter/test-model'", mc.Modes[0].Model)
	}
	if mc.LastMode != "default" {
		t.Errorf("LastMode = %q, want 'default'", mc.LastMode)
	}
}

// TestModesConfigSaveLoadRoundTrip verifies modes survive a save/load cycle.
func TestModesConfigSaveLoadRoundTrip(t *testing.T) {
	mc := &ModesConfig{
		Modes: []Mode{
			{Name: "default", Model: "openrouter/a"},
			{Name: "planning", Model: "openrouter/b", Directive: "read-only", DeniedTools: []string{"shell", "write_file"}, Agents: []string{"worker"}},
		},
		LastMode: "planning",
	}
	path := writeModes(t, mc)
	loaded, err := LoadModesFrom(path, "openrouter/fallback")
	if err != nil {
		t.Fatalf("LoadModesFrom() failed: %v", err)
	}
	if len(loaded.Modes) != 2 {
		t.Fatalf("Modes = %d, want 2", len(loaded.Modes))
	}
	if loaded.Modes[1].Name != "planning" {
		t.Errorf("Modes[1].Name = %q, want 'planning'", loaded.Modes[1].Name)
	}
	if loaded.Modes[1].Directive != "read-only" {
		t.Errorf("Modes[1].Directive = %q, want 'read-only'", loaded.Modes[1].Directive)
	}
	if len(loaded.Modes[1].DeniedTools) != 2 || loaded.Modes[1].DeniedTools[0] != "shell" {
		t.Errorf("Modes[1].DeniedTools = %#v", loaded.Modes[1].DeniedTools)
	}
	if len(loaded.Modes[1].Agents) != 1 || loaded.Modes[1].Agents[0] != "worker" {
		t.Errorf("Modes[1].Agents = %#v", loaded.Modes[1].Agents)
	}
	if loaded.LastMode != "planning" {
		t.Errorf("LastMode = %q, want 'planning'", loaded.LastMode)
	}
}

// TestLoadModesFromMissing falls back to default.
func TestLoadModesFromMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "modes.json")
	mc, err := LoadModesFrom(path, "openrouter/default-model")
	if err != nil {
		t.Fatalf("LoadModesFrom() unexpected error: %v", err)
	}
	if len(mc.Modes) != 1 {
		t.Fatalf("Modes = %d, want 1 (fallback)", len(mc.Modes))
	}
	if mc.Modes[0].Name != "default" {
		t.Errorf("Name = %q, want 'default'", mc.Modes[0].Name)
	}
	if mc.Modes[0].Model != "openrouter/default-model" {
		t.Errorf("Model = %q, want fallback model", mc.Modes[0].Model)
	}
}

// TestLoadModesFromCorrupted falls back to default on invalid JSON.
func TestLoadModesFromCorrupted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "modes.json")
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte("{invalid json}"), 0600)
	mc, err := LoadModesFrom(path, "openrouter/fallback")
	if err != nil {
		t.Fatalf("LoadModesFrom() unexpected error: %v", err)
	}
	if len(mc.Modes) != 1 {
		t.Fatalf("Modes = %d, want 1 (fallback)", len(mc.Modes))
	}
	if mc.Modes[0].Name != "default" {
		t.Errorf("Name = %q, want 'default'", mc.Modes[0].Name)
	}
}

// TestLoadModesFromEmpty falls back to default when modes array is empty.
func TestLoadModesFromEmpty(t *testing.T) {
	mc := &ModesConfig{Modes: []Mode{}, LastMode: ""}
	path := writeModes(t, mc)
	loaded, err := LoadModesFrom(path, "openrouter/fallback")
	if err != nil {
		t.Fatalf("LoadModesFrom() failed: %v", err)
	}
	if len(loaded.Modes) != 1 {
		t.Fatalf("Modes = %d, want 1 (fallback for empty)", len(loaded.Modes))
	}
}

// TestValidateDuplicateModeName verifies duplicate mode names fail.
func TestValidateDuplicateModeName(t *testing.T) {
	mc := &ModesConfig{
		Modes: []Mode{
			{Name: "default", Model: "openrouter/a"},
			{Name: "default", Model: "openrouter/b"},
		},
	}
	err := mc.Validate(validProviders())
	if err == nil {
		t.Fatal("Validate() expected error for duplicate mode name, got nil")
	}
}

// TestValidateModeModelBadFormat verifies bad model format fails.
func TestValidateModeModelBadFormat(t *testing.T) {
	mc := &ModesConfig{
		Modes: []Mode{{Name: "test", Model: "no-slash"}},
	}
	err := mc.Validate(validProviders())
	if err == nil {
		t.Fatal("Validate() expected error for bad mode model format, got nil")
	}
}

// TestValidateModeModelUnknownProvider verifies missing provider fails.
func TestValidateModeModelUnknownProvider(t *testing.T) {
	mc := &ModesConfig{
		Modes: []Mode{{Name: "test", Model: "ghost/model-x"}},
	}
	err := mc.Validate(validProviders())
	if err == nil {
		t.Fatal("Validate() expected error for missing provider in mode, got nil")
	}
}

// TestValidateLastModeNotFound verifies dangling last_mode fails.
func TestValidateLastModeNotFound(t *testing.T) {
	mc := &ModesConfig{
		Modes: []Mode{
			{Name: "default", Model: "openrouter/a"},
		},
		LastMode: "nonexistent",
	}
	err := mc.Validate(validProviders())
	if err == nil {
		t.Fatal("Validate() expected error for non-existent last_mode, got nil")
	}
}

// TestModesSaveAtomic verifies the temp-file pattern prevents corruption.
func TestModesSaveAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "modes.json")
	mc := DefaultMode("openrouter/test")
	if err := mc.SaveTo(path); err != nil {
		t.Fatalf("SaveTo() failed: %v", err)
	}
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Errorf("tmp file %s should not exist after commit", tmpPath)
	}
	loaded, err := LoadModesFrom(path, "openrouter/fallback")
	if err != nil {
		t.Fatalf("LoadModesFrom() failed: %v", err)
	}
	if len(loaded.Modes) != 1 {
		t.Errorf("Modes = %d, want 1", len(loaded.Modes))
	}
}

// TestMigrateFromConfigNoModes verifies migration is a no-op when no modes in config.
func TestMigrateFromConfigNoModes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := validConfig()
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}
	if err := MigrateFromConfig(); err != nil {
		t.Fatalf("MigrateFromConfig() failed: %v", err)
	}
	// modes.json should still not exist (no modes to migrate).
	mc, err := LoadModes("openrouter/deepseek-v4-flash")
	if err != nil {
		t.Fatalf("LoadModes() failed: %v", err)
	}
	if len(mc.Modes) != 1 {
		t.Fatalf("Modes = %d, want 1 (fallback)", len(mc.Modes))
	}
}

// TestReasoningLevelsSaveLoadRoundTrip verifies reasoning_levels survive a save/load cycle.
func TestReasoningLevelsSaveLoadRoundTrip(t *testing.T) {
	mc := &ModesConfig{
		Modes:    []Mode{{Name: "default", Model: "openrouter/a"}},
		LastMode: "default",
		ReasoningLevels: map[string]string{
			"openrouter/o3":    "high",
			"openrouter/gpt-5": "med",
		},
	}
	path := writeModes(t, mc)
	loaded, err := LoadModesFrom(path, "openrouter/fallback")
	if err != nil {
		t.Fatalf("LoadModesFrom() failed: %v", err)
	}
	if loaded.ReasoningLevels == nil {
		t.Fatal("ReasoningLevels is nil after load")
	}
	if loaded.ReasoningLevels["openrouter/o3"] != "high" {
		t.Errorf("ReasoningLevels[openrouter/o3] = %q, want high", loaded.ReasoningLevels["openrouter/o3"])
	}
	if loaded.ReasoningLevels["openrouter/gpt-5"] != "med" {
		t.Errorf("ReasoningLevels[openrouter/gpt-5] = %q, want med", loaded.ReasoningLevels["openrouter/gpt-5"])
	}
}

// TestReasoningLevelsMissing returns empty string when key is absent.
func TestReasoningLevelsMissing(t *testing.T) {
	mc := &ModesConfig{
		Modes:    []Mode{{Name: "default", Model: "openrouter/a"}},
		LastMode: "default",
	}
	level := mc.ReasoningLevelFor("openrouter/o3")
	if level != "" {
		t.Errorf("ReasoningLevelFor() = %q, want empty", level)
	}
}

// TestReasoningLevelsNilMap returns empty string on nil map.
func TestReasoningLevelsNilMap(t *testing.T) {
	mc := &ModesConfig{}
	level := mc.ReasoningLevelFor("openrouter/o3")
	if level != "" {
		t.Errorf("ReasoningLevelFor() = %q, want empty", level)
	}
}

// TestSetReasoningLevel updates the map and persists via SaveTo.
func TestSetReasoningLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "modes.json")
	mc := &ModesConfig{
		Modes:    []Mode{{Name: "default", Model: "openrouter/a"}},
		LastMode: "default",
	}
	if err := mc.SaveTo(path); err != nil {
		t.Fatalf("SaveTo() failed: %v", err)
	}
	// Initialize the map and update the reasoning level.
	mc.ReasoningLevels = make(map[string]string)
	mc.ReasoningLevels["openrouter/o3"] = "high"
	if err := mc.SaveTo(path); err != nil {
		t.Fatalf("SaveTo() after update failed: %v", err)
	}
	// Reload from disk.
	loaded, err := LoadModesFrom(path, "openrouter/fallback")
	if err != nil {
		t.Fatalf("LoadModesFrom() failed: %v", err)
	}
	if loaded.ReasoningLevels["openrouter/o3"] != "high" {
		t.Errorf("persisted level = %q, want high", loaded.ReasoningLevels["openrouter/o3"])
	}
}

// TestReasoningLevelsNotInJSON verifies modes without reasoning_levels load cleanly.
func TestReasoningLevelsNotInJSON(t *testing.T) {
	mc := &ModesConfig{
		Modes:    []Mode{{Name: "default", Model: "openrouter/a"}},
		LastMode: "default",
	}
	path := writeModes(t, mc)
	loaded, err := LoadModesFrom(path, "openrouter/fallback")
	if err != nil {
		t.Fatalf("LoadModesFrom() failed: %v", err)
	}
	if loaded.ReasoningLevels != nil {
		t.Errorf("ReasoningLevels = %v, want nil for absent key", loaded.ReasoningLevels)
	}
}

// TestReasoningLevelsPerModelIsolation verifies models are isolated.
func TestReasoningLevelsPerModelIsolation(t *testing.T) {
	mc := &ModesConfig{
		Modes:    []Mode{{Name: "default", Model: "openrouter/a"}},
		LastMode: "default",
		ReasoningLevels: map[string]string{
			"openrouter/o3":    "high",
			"openrouter/gpt-5": "low",
		},
	}
	if mc.ReasoningLevelFor("openrouter/o3") != "high" {
		t.Errorf("o3 level wrong")
	}
	if mc.ReasoningLevelFor("openrouter/gpt-5") != "low" {
		t.Errorf("gpt-5 level wrong")
	}
	if mc.ReasoningLevelFor("openrouter/other") != "" {
		t.Errorf("unknown model should return empty")
	}
}
