// main_test.go — tests for embedded startup asset resolution.
// Layer: application bootstrap. Dependencies: embedded filesystems.
package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blazeai/internal/platform"
)

// TestPrepareBuiltinAssetsResolvesEmbeddedFiles verifies startup performs no seeding.
func TestPrepareBuiltinAssetsResolvesEmbeddedFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	prompts, builtin, err := prepareBuiltinAssets()
	if err != nil {
		t.Fatalf("prepareBuiltinAssets() error: %v", err)
	}
	for _, name := range []string{"sysprompt.md"} {
		if _, err := fs.ReadFile(prompts, name); err != nil {
			t.Fatalf("read embedded prompt %s: %v", name, err)
		}
	}
	data, err := fs.ReadFile(builtin, "skill-manager.md")
	if err != nil || !strings.Contains(string(data), "[DESCRIPTION]") {
		t.Fatalf("embedded builtin skill unavailable: %v", err)
	}
	home, err := platform.AppHome()
	if err != nil {
		t.Fatalf("AppHome() error: %v", err)
	}
	for _, path := range []string{"prompts", "skills/skill-manager", "skills/config-manager", "skills/audit-manager"} {
		if _, err := os.Stat(filepath.Join(home, path)); !os.IsNotExist(err) {
			t.Errorf("prepareBuiltinAssets created %s, stat error=%v", path, err)
		}
	}
}
