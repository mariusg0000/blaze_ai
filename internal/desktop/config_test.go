// config_test.go — desktop singleton config and state validation tests.
// Verifies strict missing-file and field validation for the desktop transport
// config.json and state.json files.
// Layer: transport tests. Dependencies: internal/config, internal/desktop.
package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	runtimecfg "blazeai/internal/config"
)

func TestLoadConfigFromRequiresAbsoluteExistingWorkDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"workdir":"relative/path"}`), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	_, err := LoadConfigFrom(path)
	if err == nil {
		t.Fatal("LoadConfigFrom() error = nil, want invalid config error")
	}
	if !strings.Contains(err.Error(), "workdir must be an absolute path") {
		t.Fatalf("LoadConfigFrom() error = %v, want absolute path validation", err)
	}
}

func TestLoadConfigFromLoadsValidConfig(t *testing.T) {
	workDir := t.TempDir()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"workdir":`+"\""+workDir+"\""+`,"toggle_hotkey":{"enabled":true,"shortcut":"Ctrl+Shift+Space"}}`), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	cfg, err := LoadConfigFrom(path)
	if err != nil {
		t.Fatalf("LoadConfigFrom() error: %v", err)
	}
	if cfg.WorkDir != workDir {
		t.Fatalf("cfg.WorkDir = %q, want %q", cfg.WorkDir, workDir)
	}
	if !cfg.ToggleHotkey.Enabled || cfg.ToggleHotkey.Shortcut != "Ctrl+Shift+Space" {
		t.Fatalf("cfg.ToggleHotkey = %+v, want enabled Ctrl+Shift+Space", cfg.ToggleHotkey)
	}
}

func TestLoadConfigFromRejectsEnabledHotkeyWithoutShortcut(t *testing.T) {
	workDir := t.TempDir()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"workdir":`+"\""+workDir+"\""+`,"toggle_hotkey":{"enabled":true}}`), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	_, err := LoadConfigFrom(path)
	if err == nil {
		t.Fatal("LoadConfigFrom() error = nil, want missing hotkey shortcut error")
	}
	if !strings.Contains(err.Error(), "toggle_hotkey.shortcut is required") {
		t.Fatalf("LoadConfigFrom() error = %v, want hotkey shortcut validation", err)
	}
}

func TestLoadStateFromRejectsUnknownProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"selected_model":"missing/model"}`), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	globalCfg := &runtimecfg.Config{}
	_, err := LoadStateFrom(path, globalCfg)
	if err == nil {
		t.Fatal("LoadStateFrom() error = nil, want provider validation error")
	}
	if !strings.Contains(err.Error(), "selected_model provider not found") {
		t.Fatalf("LoadStateFrom() error = %v, want provider validation", err)
	}
}

func TestLoadStateFromLoadsValidState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"selected_model":"openai/gpt-5","window":{"initialized":true,"x":10,"y":20,"width":900,"height":700}}`), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	globalCfg := &runtimecfg.Config{
		Providers: []runtimecfg.Provider{{Name: "openai", Endpoint: "https://example.com", APIKey: "k"}},
	}
	state, err := LoadStateFrom(path, globalCfg)
	if err != nil {
		t.Fatalf("LoadStateFrom() error: %v", err)
	}
	if state.SelectedModel != "openai/gpt-5" {
		t.Fatalf("state.SelectedModel = %q, want %q", state.SelectedModel, "openai/gpt-5")
	}
	bounds, ok := state.WindowBoundsValue()
	if !ok {
		t.Fatal("state.WindowBoundsValue() ok = false, want true")
	}
	if bounds.Width != 900 || bounds.Height != 700 || bounds.X != 10 || bounds.Y != 20 {
		t.Fatalf("state.WindowBoundsValue() = %+v, want {X:10 Y:20 Width:900 Height:700}", bounds)
	}
}

func TestLoadStateFromRejectsInvalidWindowGeometry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"selected_model":"openai/gpt-5","window":{"initialized":true,"width":0,"height":700}}`), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	globalCfg := &runtimecfg.Config{
		Providers: []runtimecfg.Provider{{Name: "openai", Endpoint: "https://example.com", APIKey: "k"}},
	}
	_, err := LoadStateFrom(path, globalCfg)
	if err == nil {
		t.Fatal("LoadStateFrom() error = nil, want invalid window geometry error")
	}
	if !strings.Contains(err.Error(), "window.width must be greater than zero") {
		t.Fatalf("LoadStateFrom() error = %v, want window geometry validation", err)
	}
}

func TestParseHotkeyConfigNormalizesShortcut(t *testing.T) {
	spec, err := ParseHotkeyConfig(HotkeyConfig{Enabled: true, Shortcut: " ctrl + alt + b "})
	if err != nil {
		t.Fatalf("ParseHotkeyConfig() error: %v", err)
	}
	if spec.Display != "Ctrl+Alt+B" {
		t.Fatalf("spec.Display = %q, want %q", spec.Display, "Ctrl+Alt+B")
	}
}
