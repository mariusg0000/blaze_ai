// config_test.go — tests for config loading, validation, saving, and first-run detection.
// Uses temp directories for file-based tests to avoid touching the real app home.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// validConfig returns a Config with all required fields populated and valid.
func validConfig() *Config {
	return &Config{
		Providers: []Provider{
			{Name: "openrouter", Endpoint: "https://openrouter.ai/api/v1", APIKey: "sk-test123"},
		},
		FavoriteModels: []string{"openrouter/deepseek-v4-flash"},
		Roles: Roles{
			Default: "openrouter/deepseek-v4-flash",
			Vision:  "openrouter/gpt-4o",
		},
		Compaction:     DefaultCompaction(),
		StripReasoning: DefaultStripReasoning(),
	}
}

// writeConfigToTemp writes a config JSON to a temp file and returns the path.
func writeConfigToTemp(t *testing.T, cfg any) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "config.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("cannot marshal test config: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("cannot create temp config dir: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("cannot write temp config: %v", err)
	}
	return path
}

// TestLoadFromValid verifies that a valid config loads without errors.
func TestLoadFromValid(t *testing.T) {
	cfg := validConfig()
	path := writeConfigToTemp(t, cfg)
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom() unexpected error: %v", err)
	}
	if loaded.Roles.Default != cfg.Roles.Default {
		t.Errorf("LoadFrom() default = %q, want %q", loaded.Roles.Default, cfg.Roles.Default)
	}
	if len(loaded.Providers) != 1 {
		t.Errorf("LoadFrom() providers = %d, want 1", len(loaded.Providers))
	}
}

// TestLoadFromMissing verifies that a missing file returns ErrConfigMissing.
func TestLoadFromMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("LoadFrom() expected error for missing file, got nil")
	}
}

// TestLoadFromMalformed verifies that invalid JSON returns a parse error.
func TestLoadFromMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{invalid json"), 0600); err != nil {
		t.Fatalf("cannot write malformed config: %v", err)
	}
	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("LoadFrom() expected error for malformed JSON, got nil")
	}
}

// TestValidateValid verifies that a complete valid config passes validation.
func TestValidateValid(t *testing.T) {
	cfg := validConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

// TestValidateMissingDefault verifies that an empty default role fails.
func TestValidateMissingDefault(t *testing.T) {
	cfg := validConfig()
	cfg.Roles.Default = ""
	err := cfg.Validate()
	if err != ErrDefaultRoleUnassigned {
		t.Errorf("Validate() err = %v, want ErrDefaultRoleUnassigned", err)
	}
}

// TestValidateInvalidModelFormat verifies that a malformed model ID fails.
func TestValidateInvalidModelFormat(t *testing.T) {
	cfg := validConfig()
	cfg.Roles.Default = "no-slash"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for invalid model format, got nil")
	}
}

// TestValidateInvalidModelTrailingSlash verifies that "provider/" fails.
func TestValidateInvalidModelTrailingSlash(t *testing.T) {
	cfg := validConfig()
	cfg.Roles.Default = "openrouter/"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for trailing slash model, got nil")
	}
}

// TestValidateProviderNotFound verifies that a model referencing a missing provider fails.
func TestValidateProviderNotFound(t *testing.T) {
	cfg := validConfig()
	cfg.Roles.Default = "nonexistent/model-x"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for missing provider, got nil")
	}
}

// TestValidateDuplicateProvider verifies that duplicate provider names fail.
func TestValidateDuplicateProvider(t *testing.T) {
	cfg := validConfig()
	cfg.Providers = append(cfg.Providers, Provider{
		Name: "openrouter", Endpoint: "https://other.com", APIKey: "sk-other",
	})
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for duplicate provider, got nil")
	}
}

// TestValidateEmptyProviderField verifies that an empty provider field fails.
func TestValidateEmptyProviderField(t *testing.T) {
	cfg := validConfig()
	cfg.Providers[0].APIKey = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for empty api_key, got nil")
	}
}

// TestValidateFavoriteModelBadProvider verifies favorite model with missing provider fails.
func TestValidateFavoriteModelBadProvider(t *testing.T) {
	cfg := validConfig()
	cfg.FavoriteModels = append(cfg.FavoriteModels, "ghost/model-y")
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for favorite model with missing provider, got nil")
	}
}

// TestSaveToAndLoadFrom verifies round-trip save and load.
func TestSaveToAndLoadFrom(t *testing.T) {
	cfg := validConfig()
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "config.json")
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo() unexpected error: %v", err)
	}
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom() after SaveTo() unexpected error: %v", err)
	}
	if loaded.Roles.Default != cfg.Roles.Default {
		t.Errorf("round-trip default = %q, want %q", loaded.Roles.Default, cfg.Roles.Default)
	}
	if loaded.Compaction.MaxContextTokens != cfg.Compaction.MaxContextTokens {
		t.Errorf("round-trip maxContextTokens = %d, want %d",
			loaded.Compaction.MaxContextTokens, cfg.Compaction.MaxContextTokens)
	}
}

// TestSaveToCreatesDir verifies that SaveTo creates parent directories.
func TestSaveToCreatesDir(t *testing.T) {
	cfg := validConfig()
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "config.json")
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo() with nested dirs failed: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("SaveTo() did not create file: %v", err)
	}
}

// TestNeedsFirstRunAtMissing verifies that a missing file triggers first-run.
func TestNeedsFirstRunAtMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	needed, err := NeedsFirstRunAt(path)
	if err != nil {
		t.Fatalf("NeedsFirstRunAt() unexpected error: %v", err)
	}
	if !needed {
		t.Error("NeedsFirstRunAt() = false for missing file, want true")
	}
}

// TestNeedsFirstRunAtNoDefault verifies that an empty default role triggers first-run.
func TestNeedsFirstRunAtNoDefault(t *testing.T) {
	cfg := Default()
	cfg.Providers = []Provider{
		{Name: "test", Endpoint: "https://example.com", APIKey: "sk-test"},
	}
	path := writeConfigToTemp(t, cfg)
	needed, err := NeedsFirstRunAt(path)
	if err != nil {
		t.Fatalf("NeedsFirstRunAt() unexpected error: %v", err)
	}
	if !needed {
		t.Error("NeedsFirstRunAt() = false for empty default role, want true")
	}
}

// TestNeedsFirstRunAtConfigured verifies that a valid config with default role does not trigger.
func TestNeedsFirstRunAtConfigured(t *testing.T) {
	cfg := validConfig()
	path := writeConfigToTemp(t, cfg)
	needed, err := NeedsFirstRunAt(path)
	if err != nil {
		t.Fatalf("NeedsFirstRunAt() unexpected error: %v", err)
	}
	if needed {
		t.Error("NeedsFirstRunAt() = true for configured config, want false")
	}
}

// TestDefaultCompaction verifies spec default values.
func TestDefaultCompaction(t *testing.T) {
	c := DefaultCompaction()
	if c.MaxContextTokens != 100000 {
		t.Errorf("MaxContextTokens = %d, want 100000", c.MaxContextTokens)
	}
	if c.MinContextTokens != 50000 {
		t.Errorf("MinContextTokens = %d, want 50000", c.MinContextTokens)
	}
	if c.SummaryMaxTokens != 2000 {
		t.Errorf("SummaryMaxTokens = %d, want 2000", c.SummaryMaxTokens)
	}
	if c.MaxSummaryFiles != 10 {
		t.Errorf("MaxSummaryFiles = %d, want 10", c.MaxSummaryFiles)
	}
	if c.TokenCoefficient != 3.5 {
		t.Errorf("TokenCoefficient = %f, want 3.5", c.TokenCoefficient)
	}
	if c.MaxBackoffOffsetTokens != 25000 {
		t.Errorf("MaxBackoffOffsetTokens = %d, want 25000", c.MaxBackoffOffsetTokens)
	}
}

// TestDefaultStripReasoning verifies spec default values.
func TestDefaultStripReasoning(t *testing.T) {
	sr := DefaultStripReasoning()
	if !sr.Enable {
		t.Error("Enable = false, want true")
	}
	if sr.PreserveLast != 5 {
		t.Errorf("PreserveLast = %d, want 5", sr.PreserveLast)
	}
}

// TestDefault verifies that Default returns a config with populated defaults and empty roles.
func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Roles.Default != "" {
		t.Errorf("Default() Roles.Default = %q, want empty", cfg.Roles.Default)
	}
	if len(cfg.Providers) != 0 {
		t.Errorf("Default() Providers = %d, want 0", len(cfg.Providers))
	}
	if cfg.Compaction.MaxContextTokens != 100000 {
		t.Errorf("Default() Compaction.MaxContextTokens = %d, want 100000", cfg.Compaction.MaxContextTokens)
	}
	if !cfg.StripReasoning.Enable {
		t.Error("Default() StripReasoning.Enable = false, want true")
	}
}

// TestProviderByName verifies lookup by name.
func TestProviderByName(t *testing.T) {
	cfg := validConfig()
	p := cfg.ProviderByName("openrouter")
	if p == nil {
		t.Fatal("ProviderByName() returned nil for existing provider")
	}
	if p.Endpoint != "https://openrouter.ai/api/v1" {
		t.Errorf("ProviderByName() endpoint = %q, want correct URL", p.Endpoint)
	}
}

// TestProviderByNameNotFound verifies nil for a non-existent provider.
func TestProviderByNameNotFound(t *testing.T) {
	cfg := validConfig()
	p := cfg.ProviderByName("ghost")
	if p != nil {
		t.Error("ProviderByName() returned non-nil for missing provider")
	}
}

// TestSplitModelID verifies that model IDs are split correctly.
func TestSplitModelID(t *testing.T) {
	provider, model := SplitModelID("openrouter/deepseek-v4-flash")
	if provider != "openrouter" {
		t.Errorf("provider = %q, want 'openrouter'", provider)
	}
	if model != "deepseek-v4-flash" {
		t.Errorf("model = %q, want 'deepseek-v4-flash'", model)
	}
}

// TestSplitModelIDNoSlash verifies behavior when there is no separator.
func TestSplitModelIDNoSlash(t *testing.T) {
	provider, model := SplitModelID("barename")
	if provider != "barename" {
		t.Errorf("provider = %q, want 'barename'", provider)
	}
	if model != "" {
		t.Errorf("model = %q, want empty", model)
	}
}

// TestDefaultHelperSetup verifies default HelperSetup values.
func TestDefaultHelperSetup(t *testing.T) {
	cfg := Default()
	if cfg.HelperSetup.Dismissed {
		t.Error("Default() HelperSetup.Dismissed = true, want false")
	}
	if cfg.HelperSetup.Declined == nil {
		t.Error("Default() HelperSetup.Declined = nil, want empty slice")
	}
}

// TestLoadFromWithoutHelperSetup verifies backward-compatibility:
// configs without helperSetup field load successfully with zero-value.
func TestLoadFromWithoutHelperSetup(t *testing.T) {
	raw := struct {
		Providers      []Provider     `json:"providers"`
		FavoriteModels []string       `json:"favorite_models"`
		Roles          Roles          `json:"roles"`
		Compaction     Compaction     `json:"compaction"`
		StripReasoning StripReasoning `json:"stripReasoning"`
	}{
		Providers: []Provider{
			{Name: "test", Endpoint: "https://example.com/v1", APIKey: "sk-test"},
		},
		FavoriteModels: []string{"test/model"},
		Roles: Roles{
			Default: "test/model",
		},
		Compaction:     DefaultCompaction(),
		StripReasoning: DefaultStripReasoning(),
	}
	path := writeConfigToTemp(t, raw)
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom() without helperSetup failed: %v", err)
	}
	if loaded.HelperSetup.Dismissed {
		t.Error("HelperSetup.Dismissed = true for old config, want false (zero-value)")
	}
}

// TestSaveLoadHelperSetup verifies round-trip preservation of helper setup preferences.
func TestSaveLoadHelperSetup(t *testing.T) {
	cfg := validConfig()
	cfg.HelperSetup.Dismissed = true
	cfg.HelperSetup.Declined = []string{"rg", "fd"}
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "config.json")
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo() failed: %v", err)
	}
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom() after SaveTo() failed: %v", err)
	}
	if !loaded.HelperSetup.Dismissed {
		t.Error("HelperSetup.Dismissed = false, want true")
	}
	if len(loaded.HelperSetup.Declined) != 2 {
		t.Errorf("HelperSetup.Declined = %v, want [rg fd]", loaded.HelperSetup.Declined)
	}
}

