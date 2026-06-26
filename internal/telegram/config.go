// config.go — Telegram bridge instance config loading and validation.
// Loads `bridge.json` from `app_home/telegram/<instance>/`, validates required fields,
// and stops startup on any missing or invalid value.
// Layer: transport configuration. Dependencies: internal/platform.
package telegram

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"blazeai/internal/platform"
)

const bridgeFileName = "bridge.json"

// BridgeConfig holds the static configuration for one Telegram bot instance.
//
// WHAT:  Telegram bot identity, allowed chat, and working directory.
// WHY:   The bridge runs one bot process bound to one trusted chat and one work folder.
// PARAMS: BotToken — Telegram bot token; AllowedChatID — only accepted Telegram chat;
// WorkDir — project working directory for this instance.
type BridgeConfig struct {
	BotToken      string `json:"bot_token"`
	AllowedChatID int64  `json:"allowed_chat_id"`
	WorkDir       string `json:"workdir"`
}

// InstanceDir resolves the Telegram instance folder under app home.
//
// WHAT:  Returns `app_home/telegram/<instance>` for a validated instance name.
// WHY:   Startup and state persistence must use a deterministic per-instance location.
// PARAMS: instance — Telegram instance folder name.
// RETURNS: string — absolute instance folder path; error if the name is invalid or app home fails.
func InstanceDir(instance string) (string, error) {
	name := strings.TrimSpace(instance)
	if name == "" {
		return "", fmt.Errorf("telegram instance name is required")
	}
	if name == "." || name == ".." {
		return "", fmt.Errorf("telegram instance name is invalid: %s", instance)
	}
	if strings.ContainsRune(name, os.PathSeparator) || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return "", fmt.Errorf("telegram instance name must not contain path separators: %s", instance)
	}
	home, err := platform.AppHome()
	if err != nil {
		return "", fmt.Errorf("cannot resolve app home: %w", err)
	}
	return filepath.Join(home, "telegram", name), nil
}

// LoadBridgeConfig loads and validates bridge.json for one instance.
func LoadBridgeConfig(instance string) (*BridgeConfig, string, error) {
	dir, err := InstanceDir(instance)
	if err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, bridgeFileName)
	cfg, err := LoadBridgeConfigFrom(path)
	if err != nil {
		return nil, path, err
	}
	return cfg, path, nil
}

// LoadBridgeConfigFrom loads and validates a bridge config from an explicit file path.
func LoadBridgeConfigFrom(path string) (*BridgeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("telegram bridge config missing: %s", path)
		}
		return nil, fmt.Errorf("cannot read telegram bridge config %s: %w", path, err)
	}
	var cfg BridgeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("cannot parse telegram bridge config %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid telegram bridge config %s: %w", path, err)
	}
	return &cfg, nil
}

// Validate checks bridge config fields strictly.
func (c *BridgeConfig) Validate() error {
	if strings.TrimSpace(c.BotToken) == "" {
		return fmt.Errorf("bot_token is required")
	}
	if c.AllowedChatID == 0 {
		return fmt.Errorf("allowed_chat_id is required")
	}
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
