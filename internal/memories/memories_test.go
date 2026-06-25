// memories_test.go — tests for custom memory parsing, discovery, and active list.
package memories

import (
	"os"
	"path/filepath"
	"testing"
)

// writeMemory writes a memory file to a temp directory.
func writeMemory(t *testing.T, dir, filename, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("cannot create dir %s: %v", dir, err)
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("cannot write memory %s: %v", path, err)
	}
	return path
}

// TestParseValid verifies that a well-formed memory file parses correctly.
func TestParseValid(t *testing.T) {
	content := `[DESCRIPTION]
A test memory for testing purposes.

[DETAILS]
# Test Memory

This is the full detail of the test memory.`
	bank, err := Parse("test", content)
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}
	if bank.Name != "test" {
		t.Errorf("Name = %q, want 'test'", bank.Name)
	}
	if bank.Description != "A test memory for testing purposes." {
		t.Errorf("Description = %q, want 'A test memory for testing purposes.'", bank.Description)
	}
	if bank.Details == "" {
		t.Error("Details is empty, want content")
	}
}

// TestParseMissingDescription verifies error when [DESCRIPTION] is absent.
func TestParseMissingDescription(t *testing.T) {
	_, err := Parse("test", "[DETAILS]\nOnly details here.")
	if err != ErrMissingDescription {
		t.Errorf("Parse() err = %v, want ErrMissingDescription", err)
	}
}

// TestParseMissingDetails verifies error when [DETAILS] is absent.
func TestParseMissingDetails(t *testing.T) {
	_, err := Parse("test", "[DESCRIPTION]\nOnly description here.")
	if err != ErrMissingDetails {
		t.Errorf("Parse() err = %v, want ErrMissingDetails", err)
	}
}

// TestActiveList verifies basic load/unload behavior.
func TestActiveList(t *testing.T) {
	a := NewActiveList()
	a.Load("my-network")
	if !a.Has("my-network") {
		t.Error("Has(my-network) = false, want true")
	}
	a.Unload("my-network")
	if a.Has("my-network") {
		t.Error("Has(my-network) = true after Unload, want false")
	}
}

// TestDiscoverFromDir verifies discovery from a custom directory.
func TestDiscoverFromDir(t *testing.T) {
	root := t.TempDir()
	writeMemory(t, root, "my-network.md", "[DESCRIPTION]\nNetwork inventory.\n\n[DETAILS]\nIPs and roles.")
	writeMemory(t, root, "project-deploy.md", "[DESCRIPTION]\nDeploy notes.\n\n[DETAILS]\nCI and rollout details.")

	banks, err := DiscoverFromDir(root)
	if err != nil {
		t.Fatalf("DiscoverFromDir() unexpected error: %v", err)
	}
	if len(banks) != 2 {
		t.Fatalf("discovered %d memories, want 2", len(banks))
	}
	if banks["my-network"] == nil || banks["project-deploy"] == nil {
		t.Fatal("expected discovered memories not found")
	}
}

// TestDiscoverFromDirSkipsInvalid verifies invalid memory files are skipped.
func TestDiscoverFromDirSkipsInvalid(t *testing.T) {
	root := t.TempDir()
	writeMemory(t, root, "valid.md", "[DESCRIPTION]\nValid.\n\n[DETAILS]\nValid details.")
	writeMemory(t, root, "invalid.md", "no sections here")

	banks, err := DiscoverFromDir(root)
	if err != nil {
		t.Fatalf("DiscoverFromDir() unexpected error: %v", err)
	}
	if len(banks) != 1 {
		t.Fatalf("discovered %d memories, want 1", len(banks))
	}
	if banks["valid"] == nil {
		t.Fatal("valid memory not found")
	}
	if banks["invalid"] != nil {
		t.Fatal("invalid memory should have been skipped")
	}
}

// TestDiscoverFromDirIgnoresReadme verifies folder documentation is never treated as a memory.
func TestDiscoverFromDirIgnoresReadme(t *testing.T) {
	root := t.TempDir()
	writeMemory(t, root, "README.md", "[DESCRIPTION]\nFolder docs.\n\n[DETAILS]\nThis should not load.")
	writeMemory(t, root, "real-memory.md", "[DESCRIPTION]\nReal memory.\n\n[DETAILS]\nReal details.")

	banks, err := DiscoverFromDir(root)
	if err != nil {
		t.Fatalf("DiscoverFromDir() unexpected error: %v", err)
	}
	if len(banks) != 1 {
		t.Fatalf("discovered %d memories, want 1", len(banks))
	}
	if banks["README"] != nil || banks["readme"] != nil {
		t.Fatal("README.md should have been ignored explicitly")
	}
	if banks["real-memory"] == nil {
		t.Fatal("real memory not found")
	}
}
