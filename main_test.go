// main_test.go — tests for startup asset preparation.
// Verifies editable prompt templates are seeded into app-home storage without overwriting user files.
// Layer: application bootstrap. Dependencies: embedded prompts and standard filesystem APIs.
package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// TestSeedMissingPromptFilesSeedsAndPreservesTemplates verifies default seeding and edit preservation.
func TestSeedMissingPromptFilesSeedsAndPreservesTemplates(t *testing.T) {
	source, err := fs.Sub(embeddedPrompts, "prompts")
	if err != nil {
		t.Fatalf("fs.Sub() error: %v", err)
	}
	target := t.TempDir()
	custom := "custom universal prompt\n"
	if err := os.WriteFile(filepath.Join(target, "sysprompt.md"), []byte(custom), 0644); err != nil {
		t.Fatalf("write custom prompt: %v", err)
	}

	if err := seedMissingPromptFiles(source, target); err != nil {
		t.Fatalf("seedMissingPromptFiles() error: %v", err)
	}
	entries, err := fs.ReadDir(source, ".")
	if err != nil {
		t.Fatalf("ReadDir() error: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" || entry.Name() == "readme.md" {
			continue
		}
		if _, err := os.Stat(filepath.Join(target, entry.Name())); err != nil {
			t.Errorf("seeded prompt %s: %v", entry.Name(), err)
		}
	}
	data, err := os.ReadFile(filepath.Join(target, "sysprompt.md"))
	if err != nil {
		t.Fatalf("read custom prompt: %v", err)
	}
	if string(data) != custom {
		t.Fatalf("custom prompt overwritten: got %q want %q", string(data), custom)
	}
}
