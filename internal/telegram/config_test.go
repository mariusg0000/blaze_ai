// config_test.go — tests for Telegram bridge config and state validation.
// Uses temp directories and explicit file paths to avoid touching the real app home.
package telegram

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"blazeai/internal/config"
)

func writeJSONFile(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir all: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write json: %v", err)
	}
}

func testRuntimeConfig() *config.Config {
	return &config.Config{
		Providers:      []config.Provider{{Name: "test", Endpoint: "https://example.com", APIKey: "sk-test"}},
		FavoriteModels: []string{"test/main", "test/other"},
		Roles:          config.Roles{Default: "test/main"},
		Compaction:     config.DefaultCompaction(),
		StripReasoning: config.DefaultStripReasoning(),
	}
}

func TestLoadBridgeConfigFromValid(t *testing.T) {
	workDir := t.TempDir()
	path := filepath.Join(t.TempDir(), bridgeFileName)
	writeJSONFile(t, path, BridgeConfig{BotToken: "123:abc", AllowedChatID: 42, WorkDir: workDir})
	loaded, err := LoadBridgeConfigFrom(path)
	if err != nil {
		t.Fatalf("LoadBridgeConfigFrom() error: %v", err)
	}
	if loaded.AllowedChatID != 42 {
		t.Fatalf("AllowedChatID = %d, want 42", loaded.AllowedChatID)
	}
}

func TestLoadBridgeConfigFromMissingWorkDirFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), bridgeFileName)
	writeJSONFile(t, path, BridgeConfig{BotToken: "123:abc", AllowedChatID: 42})
	if _, err := LoadBridgeConfigFrom(path); err == nil {
		t.Fatal("LoadBridgeConfigFrom() expected error for missing workdir")
	}
}

func TestLoadBridgeConfigFromRelativeWorkDirFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), bridgeFileName)
	writeJSONFile(t, path, BridgeConfig{BotToken: "123:abc", AllowedChatID: 42, WorkDir: "relative/path"})
	if _, err := LoadBridgeConfigFrom(path); err == nil {
		t.Fatal("LoadBridgeConfigFrom() expected error for relative workdir")
	}
}

func TestLoadBridgeConfigFromMissingFileFails(t *testing.T) {
	if _, err := LoadBridgeConfigFrom(filepath.Join(t.TempDir(), bridgeFileName)); err == nil {
		t.Fatal("LoadBridgeConfigFrom() expected error for missing file")
	}
}

func TestLoadStateFromValid(t *testing.T) {
	path := filepath.Join(t.TempDir(), stateFileName)
	writeJSONFile(t, path, State{SelectedModel: "test/main"})
	loaded, err := LoadStateFrom(path, testRuntimeConfig())
	if err != nil {
		t.Fatalf("LoadStateFrom() error: %v", err)
	}
	if loaded.SelectedModel != "test/main" {
		t.Fatalf("SelectedModel = %q, want test/main", loaded.SelectedModel)
	}
}

func TestLoadStateFromBadModelFormatFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), stateFileName)
	writeJSONFile(t, path, State{SelectedModel: "badformat"})
	if _, err := LoadStateFrom(path, testRuntimeConfig()); err == nil {
		t.Fatal("LoadStateFrom() expected error for invalid model format")
	}
}

func TestLoadStateFromUnknownProviderFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), stateFileName)
	writeJSONFile(t, path, State{SelectedModel: "ghost/model"})
	if _, err := LoadStateFrom(path, testRuntimeConfig()); err == nil {
		t.Fatal("LoadStateFrom() expected error for unknown provider")
	}
}

func TestInstanceDirRejectsPathTraversal(t *testing.T) {
	if _, err := InstanceDir("../escape"); err == nil {
		t.Fatal("InstanceDir() expected error for path traversal")
	}
}
