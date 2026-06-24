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
			{Name: "planning", Model: "openrouter/b", Directive: "read-only"},
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

// TestModesReload verifies hot-reload from disk.
func TestModesReload(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mc := DefaultMode("openrouter/a")
	if err := mc.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Load fresh and verify.
	loaded, err := LoadModes("openrouter/a")
	if err != nil {
		t.Fatalf("LoadModes() failed: %v", err)
	}
	if len(loaded.Modes) != 1 {
		t.Fatalf("Modes = %d, want 1", len(loaded.Modes))
	}

	// Write a new modes file with an extra mode.
	mc.Modes = append(mc.Modes, Mode{Name: "planning", Model: "openrouter/b", Directive: "plan"})
	if err := mc.Save(); err != nil {
		t.Fatalf("Save() with extra mode failed: %v", err)
	}

	// Reload and verify hot-reload picks up the new mode.
	if err := loaded.Reload(); err != nil {
		t.Fatalf("Reload() failed: %v", err)
	}
	if len(loaded.Modes) != 2 {
		t.Fatalf("after reload: Modes = %d, want 2", len(loaded.Modes))
	}
	if loaded.Modes[1].Name != "planning" {
		t.Errorf("after reload: Modes[1].Name = %q, want 'planning'", loaded.Modes[1].Name)
	}
}

// TestModesReloadInvalid verifies that corrupt modes on disk are rejected and in-memory unchanged.
func TestModesReloadInvalid(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mc := DefaultMode("openrouter/a")
	if err := mc.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}
	loaded, err := LoadModes("openrouter/a")
	if err != nil {
		t.Fatalf("LoadModes() failed: %v", err)
	}

	// Write duplicate mode names (invalid).
	bad := &ModesConfig{
		Modes: []Mode{
			{Name: "default", Model: "openrouter/a"},
			{Name: "default", Model: "openrouter/b"},
		},
	}
	if err := bad.Save(); err != nil {
		t.Fatalf("Save() with invalid modes failed: %v", err)
	}

	// Reload should fail.
	if err := loaded.Reload(); err == nil {
		t.Fatal("Reload() expected error for invalid modes, got nil")
	}
	// In-memory should remain unchanged.
	if len(loaded.Modes) != 1 {
		t.Errorf("after failed reload: Modes = %d, want 1 (unchanged)", len(loaded.Modes))
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
