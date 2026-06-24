// config.go — configuration types, loading, validation, and persistence.
// Defines the Config struct tree matching config.json, loads from app_home/config/,
// validates provider/model/role integrity, and provides default values for first-run setup.
// Layer: configuration. Dependencies: internal/platform (app home path resolution).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"blazeai/internal/platform"
)

// ErrConfigMissing is returned when config.json does not exist on disk.
var ErrConfigMissing = errors.New("config file missing")

// ErrDefaultRoleUnassigned is returned when the default model role is empty.
var ErrDefaultRoleUnassigned = errors.New("default model role is not assigned")

// ErrInvalidModelFormat is returned when a model identifier is not in provider/model_name form.
var ErrInvalidModelFormat = errors.New("model identifier must be in provider/model_name format")

// ErrProviderNotFound is returned when a model references a provider not in the providers list.
var ErrProviderNotFound = errors.New("model references a provider that does not exist")

// ErrDuplicateProvider is returned when two providers share the same name.
var ErrDuplicateProvider = errors.New("duplicate provider name")

// ErrDuplicateModeName is returned when two modes share the same name.
var ErrDuplicateModeName = errors.New("duplicate mode name")

// ErrModeModelInvalid is returned when a mode's model identifier is malformed or references a missing provider.
var ErrModeModelInvalid = errors.New("mode model invalid")

// ErrLastModeNotFound is returned when last_mode references a mode that does not exist.
var ErrLastModeNotFound = errors.New("last_mode references a non-existent mode")

// Provider defines a single OpenAI-compatible endpoint.
//
// WHAT:  Represents one API provider with credentials.
// WHY:   The runtime needs endpoint and key to make LLM calls.
// PARAMS: Name — unique provider identifier; Endpoint — base API URL; APIKey — secret key.
type Provider struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	APIKey   string `json:"api_key"`
}

// Roles maps model roles to provider/model_name identifiers.
//
// WHAT:  Assigns models to functional roles in the runtime.
// WHY:   Different tasks (default, vision, summarization) may use different models.
// PARAMS: Default — required, normal interaction; Vision — optional; Summarization — optional.
type Roles struct {
	Default       string `json:"default"`
	Vision        string `json:"vision,omitempty"`
	Summarization string `json:"summarization,omitempty"`
}

// Mode defines one work mode: a name, its assigned model, and an optional directive.
//
// WHAT:  Represents a named work mode with model binding and behavioral directive.
// WHY:   Modes let the user switch between different operational styles (planning, quick, etc.)
// and automatically assign the right model and directive for each context.
// PARAMS: Name — unique mode identifier; Model — provider/model_name for this mode;
// Directive — optional text injected into the last LLM message (volatile, not persisted in session).
type Mode struct {
	Name      string `json:"name"`
	Model     string `json:"model"`
	Directive string `json:"directive,omitempty"`
}

// Compaction holds context compaction thresholds.
//
// WHAT:  Configurable thresholds that control when and how context is compacted.
// WHY:   Long sessions exceed model context windows; these values tune the compaction behavior.
// PARAMS: see field comments for each threshold.
type Compaction struct {
	MaxContextTokens       int     `json:"maxContextTokens"`       // base trigger threshold
	MinContextTokens       int     `json:"minContextTokens"`       // target retained tokens after compaction
	SummaryMaxTokens       int     `json:"summaryMaxTokens"`       // token budget for summarizer
	MaxSummaryFiles        int     `json:"maxSummaryFiles"`        // max summary chunks retained per session
	TokenCoefficient       float64 `json:"tokenCoefficient"`       // char-to-token divisor for local estimator
	MaxBackoffOffsetTokens int     `json:"maxBackoffOffsetTokens"` // max offset above base threshold (hard cap)
}

// StripReasoning controls reasoning part stripping in the LLM payload.
//
// WHAT:  Configures whether and how reasoning parts are removed from payloads.
// WHY:   Some providers error if reasoning parts are missing; stripping reduces token usage.
// PARAMS: Enable — toggle stripping; PreserveLast — number of newest reasoning parts to keep.
type StripReasoning struct {
	Enable       bool `json:"enable"`
	PreserveLast int  `json:"preserveLast"`
}

// HelperSetup holds user preferences for optional host helper utility setup prompts.
// It does NOT store whether a helper is actually installed — that is detected live.
//
// WHAT:  UX preferences for helper installation suggestions.
// WHY:   Avoids annoying the user with repeated install prompts for utilities they declined.
// PARAMS: Dismissed — suppress all optional helper install suggestions; Declined — helpers explicitly declined.
type HelperSetup struct {
	Dismissed bool     `json:"dismissed"`
	Declined  []string `json:"declined,omitempty"`
}

// Config is the root configuration structure loaded from config.json.
//
// WHAT:  The single source of truth for BlazeAI runtime configuration.
// WHY:   All runtime behavior depends on these values; no fallbacks are applied silently.
// PARAMS: Providers — endpoint definitions; FavoriteModels — model list; Roles — role assignments;
//
//	Compaction — thresholds; StripReasoning — payload settings; LastModel — persisted selection;
//	HelperSetup — UX preferences for optional host helper installation prompts.
type Config struct {
	Providers      []Provider     `json:"providers"`
	FavoriteModels []string       `json:"favorite_models"`
	Roles          Roles          `json:"roles"`
	Compaction     Compaction     `json:"compaction"`
	StripReasoning StripReasoning `json:"stripReasoning"`
	LastModel      string         `json:"last_model,omitempty"`
	HelperSetup    HelperSetup    `json:"helperSetup,omitempty"`
}

// DefaultCompaction returns the pre-filled compaction thresholds from spec 05.
//
// WHAT:  Returns compaction defaults for first-run setup.
// WHY:   Spec requires these values to be pre-filled at init.
// RETURNS: Compaction — populated with spec defaults.
func DefaultCompaction() Compaction {
	return Compaction{
		MaxContextTokens:       100000,
		MinContextTokens:       50000,
		SummaryMaxTokens:       2000,
		MaxSummaryFiles:        10,
		TokenCoefficient:       3.5,
		MaxBackoffOffsetTokens: 25000,
	}
}

// DefaultStripReasoning returns the pre-filled strip reasoning settings from spec 05.
//
// WHAT:  Returns strip reasoning defaults for first-run setup.
// WHY:   Spec requires these values to be pre-filled at init.
// RETURNS: StripReasoning — populated with spec defaults.
func DefaultStripReasoning() StripReasoning {
	return StripReasoning{
		Enable:       true,
		PreserveLast: 5,
	}
}

// Default returns a Config with default compaction and strip reasoning values,
// empty providers, models, and roles. Used as a template during first-run setup.
//
// WHAT:  Returns a config skeleton with spec defaults for optional sections.
// WHY:   First-run setup starts from defaults and fills in provider/model/role data.
// RETURNS: Config — defaults populated, providers/models/roles empty.
func Default() *Config {
	return &Config{
		Providers:      []Provider{},
		FavoriteModels: []string{},
		Roles:          Roles{},
		Compaction:     DefaultCompaction(),
		StripReasoning: DefaultStripReasoning(),
		HelperSetup: HelperSetup{
			Dismissed: false,
			Declined:  []string{},
		},
	}
}

// configPath resolves the full path to config.json under app home.
//
// WHAT:  Returns the absolute path to the config file.
// WHY:   Load and Save need a consistent path for config.json.
// RETURNS: string — path to app_home/config/config.json; error if app home cannot be resolved.
func configPath() (string, error) {
	home, err := platform.AppHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "config", "config.json"), nil
}

// NeedsFirstRun returns true if config.json is missing or the default role is unassigned.
//
// WHAT:  Checks whether first-run setup must be triggered.
// WHY:   Spec requires first-run when config is missing or default role is empty.
// RETURNS: bool — true if first-run is needed; error if config exists but cannot be read/parsed.
func NeedsFirstRun() (bool, error) {
	path, err := configPath()
	if err != nil {
		return false, err
	}
	return NeedsFirstRunAt(path)
}

// NeedsFirstRunAt checks a specific path for first-run conditions.
//
// WHAT:  Same as NeedsFirstRun but for an explicit file path.
// WHY:   Enables testing with temp directories.
// PARAMS: path — the config file path to check.
// RETURNS: bool — true if first-run is needed; error if config exists but cannot be read/parsed.
func NeedsFirstRunAt(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("cannot stat config file: %w", err)
	}
	cfg, err := loadRawFrom(path)
	if err != nil {
		return false, err
	}
	return cfg.Roles.Default == "", nil
}

// Load reads and parses config.json from app_home/config/.
//
// WHAT:  Loads the runtime configuration from disk.
// WHY:   Every session start needs the current config to select providers and models.
// HOW:   Reads config.json, parses JSON into Config, validates integrity.
// RETURNS: *Config — parsed and validated config; error if file missing, malformed, or invalid.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	return LoadFrom(path)
}

// LoadFrom reads and parses a config file from an explicit path.
//
// WHAT:  Same as Load but for an explicit file path.
// WHY:   Enables testing with temp directories and custom config locations.
// PARAMS: path — the config file path to load.
// RETURNS: *Config — parsed and validated config; error if file missing, malformed, or invalid.
func LoadFrom(path string) (*Config, error) {
	cfg, err := loadRawFrom(path)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// loadRawFrom reads and parses a config file without running validation.
//
// WHAT:  Loads the config JSON from disk without checking required fields.
// WHY:   NeedsFirstRun must inspect the default role before full validation rejects it.
// PARAMS: path — the config file path to load.
// RETURNS: *Config — parsed but unvalidated config; error if file missing or malformed.
func loadRawFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrConfigMissing, path)
		}
		return nil, fmt.Errorf("cannot read config file: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config file: %w", err)
	}
	return &cfg, nil
}

// Save writes the config to app_home/config/config.json with indentation.
//
// WHAT:  Persists the configuration to disk.
// WHY:   First-run setup and /model command need to persist changes.
// HOW:   Marshals to indented JSON, writes to config.json, creates parent dir if needed.
// RETURNS: error if marshaling or writing fails.
func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	return c.SaveTo(path)
}

// SaveTo writes the config to an explicit path with indentation.
//
// WHAT:  Same as Save but for an explicit file path.
// WHY:   Enables testing with temp directories.
// PARAMS: path — the file path to write to.
// RETURNS: error if marshaling or writing fails.
func (c *Config) SaveTo(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create config directory %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("cannot write config file %s: %w", path, err)
	}
	return nil
}

// Validate checks the config for structural integrity and required fields.
// Returns an error describing the first problem found. No silent fallbacks.
//
// WHAT:  Verifies that all required config fields are present and correctly formatted.
// WHY:   The runtime must stop with a clear error if config is invalid — no fallbacks.
// HOW:   Checks default role, provider uniqueness, model format, and provider references.
// RETURNS: error — nil if valid; specific error if any check fails.
func (c *Config) Validate() error {
	if c.Roles.Default == "" {
		return ErrDefaultRoleUnassigned
	}
	if err := validateModelFormat(c.Roles.Default); err != nil {
		return fmt.Errorf("default role: %w", err)
	}
	if c.Roles.Vision != "" {
		if err := validateModelFormat(c.Roles.Vision); err != nil {
			return fmt.Errorf("vision role: %w", err)
		}
	}
	if c.Roles.Summarization != "" {
		if err := validateModelFormat(c.Roles.Summarization); err != nil {
			return fmt.Errorf("summarization role: %w", err)
		}
	}
	if err := validateProviders(c.Providers); err != nil {
		return err
	}
	providerNames := providerNameSet(c.Providers)
	if err := validateModelProvider(c.Roles.Default, providerNames); err != nil {
		return fmt.Errorf("default role: %w", err)
	}
	if c.Roles.Vision != "" {
		if err := validateModelProvider(c.Roles.Vision, providerNames); err != nil {
			return fmt.Errorf("vision role: %w", err)
		}
	}
	if c.Roles.Summarization != "" {
		if err := validateModelProvider(c.Roles.Summarization, providerNames); err != nil {
			return fmt.Errorf("summarization role: %w", err)
		}
	}
	for _, model := range c.FavoriteModels {
		if err := validateModelFormat(model); err != nil {
			return fmt.Errorf("favorite model %q: %w", model, err)
		}
		if err := validateModelProvider(model, providerNames); err != nil {
			return fmt.Errorf("favorite model %q: %w", model, err)
		}
	}
	return nil
}

// validateModelFormat checks that a model identifier is in provider/model_name form.
//
// WHAT:  Verifies that a model string contains exactly one "/" separator.
// WHY:   The runtime always works with full provider/model_name identifiers.
// PARAMS: model — the model identifier to check.
// RETURNS: error if the format is invalid.
func validateModelFormat(model string) error {
	idx := strings.Index(model, "/")
	if idx <= 0 || idx == len(model)-1 {
		return ErrInvalidModelFormat
	}
	if strings.Contains(model[idx+1:], "/") {
		return ErrInvalidModelFormat
	}
	return nil
}

// validateProviders checks for duplicate provider names and empty fields.
//
// WHAT:  Verifies that all providers have non-empty fields and unique names.
// WHY:   Provider names are used as identifiers; duplicates cause ambiguity.
// PARAMS: providers — the provider list to check.
// RETURNS: error if a provider is malformed or a duplicate name exists.
func validateProviders(providers []Provider) error {
	seen := make(map[string]bool)
	for _, p := range providers {
		if p.Name == "" {
			return fmt.Errorf("provider name is empty")
		}
		if p.Endpoint == "" {
			return fmt.Errorf("provider %q: endpoint is empty", p.Name)
		}
		if p.APIKey == "" {
			return fmt.Errorf("provider %q: api_key is empty", p.Name)
		}
		if seen[p.Name] {
			return fmt.Errorf("%w: %s", ErrDuplicateProvider, p.Name)
		}
		seen[p.Name] = true
	}
	return nil
}

// providerNameSet returns a set of provider names for fast lookup.
//
// WHAT:  Builds a map of provider names for O(1) existence checks.
// WHY:   Model validation needs to verify that referenced providers exist.
// PARAMS: providers — the provider list to index.
// RETURNS: map[string]bool — provider name lookup table.
func providerNameSet(providers []Provider) map[string]bool {
	set := make(map[string]bool, len(providers))
	for _, p := range providers {
		set[p.Name] = true
	}
	return set
}

// validateModelProvider checks that the provider part of a model ID exists.
//
// WHAT:  Verifies that a model's provider prefix matches an existing provider.
// WHY:   A model identifier without a matching provider is a dangling reference.
// PARAMS: model — the model identifier; providers — the provider name lookup table.
// RETURNS: error if the provider does not exist.
func validateModelProvider(model string, providers map[string]bool) error {
	idx := strings.Index(model, "/")
	providerName := model[:idx]
	if !providers[providerName] {
		return fmt.Errorf("%w: %s", ErrProviderNotFound, providerName)
	}
	return nil
}

// ProviderByName returns the provider with the given name, or nil if not found.
//
// WHAT:  Looks up a provider by name from the config.
// WHY:   The runtime needs provider details (endpoint, key) to make API calls.
// PARAMS: name — the provider identifier to find.
// RETURNS: *Provider — matching provider or nil.
func (c *Config) ProviderByName(name string) *Provider {
	for i := range c.Providers {
		if c.Providers[i].Name == name {
			return &c.Providers[i]
		}
	}
	return nil
}

// SplitModelID separates a provider/model_name identifier into its parts.
//
// WHAT:  Splits a model identifier into provider name and model name.
// WHY:   API calls need the bare model name and the provider separately.
// PARAMS: modelID — the full provider/model_name string.
// RETURNS: provider string, model string — the two parts.
func SplitModelID(modelID string) (provider, model string) {
	idx := strings.Index(modelID, "/")
	if idx < 0 {
		return modelID, ""
	}
	return modelID[:idx], modelID[idx+1:]
}

// validateModes checks the modes list for integrity: unique names, valid models.
//
// WHAT:  Verifies all modes have unique names and valid model identifiers.
// WHY:   Duplicate names cause ambiguity; invalid models cause runtime failures.
// PARAMS: modes — the mode list to check; providerNames — existing provider names.
// RETURNS: error if any mode is invalid.
func validateModes(modes []Mode, providerNames map[string]bool) error {
	seen := make(map[string]bool, len(modes))
	for _, m := range modes {
		if m.Name == "" {
			return fmt.Errorf("mode name is empty")
		}
		if seen[m.Name] {
			return fmt.Errorf("%w: %s", ErrDuplicateModeName, m.Name)
		}
		seen[m.Name] = true
		if err := validateModelFormat(m.Model); err != nil {
			return fmt.Errorf("mode %q: %w: %s", m.Name, ErrModeModelInvalid, m.Model)
		}
		if err := validateModelProvider(m.Model, providerNames); err != nil {
			return fmt.Errorf("mode %q: %w: %s", m.Name, ErrModeModelInvalid, m.Model)
		}
	}
	return nil
}

// validateLastMode checks that last_mode references an existing mode name.
//
// WHAT:  Verifies last_mode is a valid reference to a mode.
// WHY:   A dangling last_mode reference is a config corruption that must stop the runtime.
// PARAMS: lastMode — the persisted mode name; modes — the mode list.
// RETURNS: error if the mode name is not found.
func validateLastMode(lastMode string, modes []Mode) error {
	for _, m := range modes {
		if m.Name == lastMode {
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrLastModeNotFound, lastMode)
}
