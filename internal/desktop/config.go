// config.go — desktop companion config loading and validation.
// Loads app_home/desktop/config.json, validates required fields strictly, and
// stops startup on any missing or invalid value.
// Layer: transport configuration. Dependencies: internal/platform.
package desktop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"blazeai/internal/platform"
)

const configFileName = "config.json"

// Config holds the static configuration for the singleton desktop transport.
//
// WHAT:  Desktop workdir binding for the local companion app.
// WHY:   The desktop transport must run against one explicit project folder.
// PARAMS: WorkDir — absolute project work directory used by the runtime.
type Config struct {
	WorkDir string `json:"workdir"`
}

// InstanceDir resolves the singleton desktop instance folder under app home.
//
// WHAT:  Returns app_home/desktop.
// WHY:   The desktop companion is a singleton transport with no named instances.
// RETURNS: string — absolute desktop instance path; error if app home fails.
func InstanceDir() (string, error) {
	home, err := platform.AppHome()
	if err != nil {
		return "", fmt.Errorf("cannot resolve app home: %w", err)
	}
	return filepath.Join(home, "desktop"), nil
}

// LoadConfig loads and validates the singleton desktop config.
//
// WHAT:  Reads the fixed desktop config file from app_home/desktop/config.json.
// WHY:   Desktop startup must fail fast on missing or invalid local config.
// RETURNS: *Config — validated desktop config; string — config path; error on failure.
func LoadConfig() (*Config, string, error) {
	dir, err := InstanceDir()
	if err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, configFileName)
	cfg, err := LoadConfigFrom(path)
	if err != nil {
		return nil, path, err
	}
	return cfg, path, nil
}

// LoadConfigFrom loads and validates a desktop config from an explicit path.
//
// WHAT:  Reads one desktop config file from disk.
// WHY:   Tests and startup share the same validation path.
// PARAMS: path — absolute config file path.
// RETURNS: *Config — validated config; error on read, parse, or validation failure.
func LoadConfigFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("desktop config missing: %s", path)
		}
		return nil, fmt.Errorf("cannot read desktop config %s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("cannot parse desktop config %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid desktop config %s: %w", path, err)
	}
	return &cfg, nil
}

// Validate checks desktop config fields strictly.
//
// WHAT:  Validates the singleton desktop config.
// WHY:   The desktop companion must stop instead of guessing a workdir.
// RETURNS: error if any required field is missing or invalid.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.WorkDir) == "" {
		return fmt.Errorf("workdir is required")
	}
	if !filepath.IsAbs(c.WorkDir) {
		return fmt.Errorf("workdir must be an absolute path: %s", c.WorkDir)
	}
	info, err := os.Stat(c.WorkDir)
	if err != nil {
		return fmt.Errorf("workdir does not exist: %s", c.WorkDir)
	}
	if !info.IsDir() {
		return fmt.Errorf("workdir is not a directory: %s", c.WorkDir)
	}
	return nil
}
