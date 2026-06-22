// skills_test.go — tests for skill parsing, discovery, collision, and active list.
// Uses temp directories to avoid touching the real app home.
package skills

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSkill writes a skill file to a temp directory.
func writeSkill(t *testing.T, dir, filename, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("cannot create dir %s: %v", dir, err)
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("cannot write skill %s: %v", path, err)
	}
	return path
}

// TestParseValid verifies that a well-formed skill file parses correctly.
func TestParseValid(t *testing.T) {
	content := `[DESCRIPTION]
A test skill for testing purposes.

[DETAILS]
# Test Skill

This is the full detail of the test skill.
It has multiple lines.`
	skill, err := Parse("test", content)
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}
	if skill.Name != "test" {
		t.Errorf("Name = %q, want 'test'", skill.Name)
	}
	if skill.Description != "A test skill for testing purposes." {
		t.Errorf("Description = %q, want 'A test skill for testing purposes.'", skill.Description)
	}
	if skill.Details == "" {
		t.Error("Details is empty, want content")
	}
}

// TestParseMissingDescription verifies error when [DESCRIPTION] is absent.
func TestParseMissingDescription(t *testing.T) {
	content := `[DETAILS]
Only details here.`
	_, err := Parse("test", content)
	if err != ErrMissingDescription {
		t.Errorf("Parse() err = %v, want ErrMissingDescription", err)
	}
}

// TestParseMissingDetails verifies error when [DETAILS] is absent.
func TestParseMissingDetails(t *testing.T) {
	content := `[DESCRIPTION]
Only description here.`
	_, err := Parse("test", content)
	if err != ErrMissingDetails {
		t.Errorf("Parse() err = %v, want ErrMissingDetails", err)
	}
}

// TestParseBothMissing verifies error when neither section is present.
func TestParseBothMissing(t *testing.T) {
	_, err := Parse("test", "no sections at all")
	if err == nil {
		t.Fatal("Parse() expected error, got nil")
	}
}

// TestParseDetailsAtEnd verifies that [DETAILS] as the last section is captured fully.
func TestParseDetailsAtEnd(t *testing.T) {
	content := "[DESCRIPTION]\nShort desc.\n\n[DETAILS]\nLine 1\nLine 2\nLine 3"
	skill, err := Parse("test", content)
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}
	if !contains(skill.Details, "Line 1") || !contains(skill.Details, "Line 3") {
		t.Errorf("Details = %q, want all three lines", skill.Details)
	}
}

// contains is a simple substring check helper.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestActiveListLoad verifies adding skills to the active list.
func TestActiveListLoad(t *testing.T) {
	a := NewActiveList()
	a.Load("memory")
	a.Load("create_skill")
	if !a.Has("memory") {
		t.Error("Has(memory) = false, want true")
	}
	if !a.Has("create_skill") {
		t.Error("Has(create_skill) = false, want true")
	}
}

// TestActiveListLoadDuplicate verifies that loading the same skill twice does not duplicate.
func TestActiveListLoadDuplicate(t *testing.T) {
	a := NewActiveList()
	a.Load("memory")
	a.Load("memory")
	if len(a.List()) != 1 {
		t.Errorf("List() = %d items, want 1", len(a.List()))
	}
}

// TestActiveListUnload verifies removing a skill from the active list.
func TestActiveListUnload(t *testing.T) {
	a := NewActiveList()
	a.Load("memory")
	a.Load("create_skill")
	a.Unload("memory")
	if a.Has("memory") {
		t.Error("Has(memory) = true after Unload, want false")
	}
	if !a.Has("create_skill") {
		t.Error("Has(create_skill) = false after Unload(memory), want true")
	}
}

// TestActiveListUnloadNotPresent verifies that unloading a non-active skill is a no-op.
func TestActiveListUnloadNotPresent(t *testing.T) {
	a := NewActiveList()
	a.Load("memory")
	a.Unload("ghost")
	if len(a.List()) != 1 {
		t.Errorf("List() = %d items, want 1", len(a.List()))
	}
}

// TestActiveListEmpty verifies that a new list is empty.
func TestActiveListEmpty(t *testing.T) {
	a := NewActiveList()
	if len(a.List()) != 0 {
		t.Errorf("NewActiveList().List() = %d items, want 0", len(a.List()))
	}
}

// TestActiveListListCopy verifies that List returns a copy, not the internal slice.
func TestActiveListListCopy(t *testing.T) {
	a := NewActiveList()
	a.Load("memory")
	l := a.List()
	l[0] = "modified"
	if a.Has("modified") {
		t.Error("modifying List() result affected internal state")
	}
	if !a.Has("memory") {
		t.Error("internal state was corrupted by List() modification")
	}
}

// TestDiscoverFromDirs verifies discovery from two directories.
func TestDiscoverFromDirs(t *testing.T) {
	builtin := filepath.Join(t.TempDir(), "builtin")
	custom := filepath.Join(t.TempDir(), "custom")

	writeSkill(t, builtin, "memory.md", "[DESCRIPTION]\nBuiltin memory.\n\n[DETAILS]\nBuiltin details.")
	writeSkill(t, builtin, "create_skill.md", "[DESCRIPTION]\nBuiltin create.\n\n[DETAILS]\nBuiltin create details.")
	writeSkill(t, custom, "my_skill.md", "[DESCRIPTION]\nCustom skill.\n\n[DETAILS]\nCustom details.")

	skills, err := DiscoverFromDirs(builtin, custom)
	if err != nil {
		t.Fatalf("DiscoverFromDirs() unexpected error: %v", err)
	}
	if len(skills) != 3 {
		t.Fatalf("discovered %d skills, want 3", len(skills))
	}
	if skills["memory"] == nil {
		t.Error("memory skill not found")
	}
	if skills["create_skill"] == nil {
		t.Error("create_skill skill not found")
	}
	if skills["my_skill"] == nil {
		t.Error("my_skill skill not found")
	}
}

// TestDiscoverCollisionCustomWins verifies that custom skills override builtin by name.
func TestDiscoverCollisionCustomWins(t *testing.T) {
	builtin := filepath.Join(t.TempDir(), "builtin")
	custom := filepath.Join(t.TempDir(), "custom")

	writeSkill(t, builtin, "memory.md", "[DESCRIPTION]\nBuiltin memory.\n\n[DETAILS]\nBuiltin details.")
	writeSkill(t, custom, "memory.md", "[DESCRIPTION]\nCustom memory.\n\n[DETAILS]\nCustom details.")

	skills, err := DiscoverFromDirs(builtin, custom)
	if err != nil {
		t.Fatalf("DiscoverFromDirs() unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("discovered %d skills, want 1 (collision resolved)", len(skills))
	}
	if skills["memory"].Description != "Custom memory." {
		t.Errorf("collision: Description = %q, want 'Custom memory.'", skills["memory"].Description)
	}
}

// TestDiscoverSkipsInvalid verifies that invalid skill files are skipped silently.
func TestDiscoverSkipsInvalid(t *testing.T) {
	builtin := filepath.Join(t.TempDir(), "builtin")
	custom := filepath.Join(t.TempDir(), "custom")

	writeSkill(t, builtin, "valid.md", "[DESCRIPTION]\nValid.\n\n[DETAILS]\nValid details.")
	writeSkill(t, builtin, "invalid.md", "no sections here")

	skills, err := DiscoverFromDirs(builtin, custom)
	if err != nil {
		t.Fatalf("DiscoverFromDirs() unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("discovered %d skills, want 1 (invalid skipped)", len(skills))
	}
	if skills["valid"] == nil {
		t.Error("valid skill not found")
	}
	if skills["invalid"] != nil {
		t.Error("invalid skill should have been skipped")
	}
}

// TestDiscoverMissingDir verifies that a missing directory is not an error.
func TestDiscoverMissingDir(t *testing.T) {
	builtin := filepath.Join(t.TempDir(), "nonexistent_builtin")
	custom := filepath.Join(t.TempDir(), "nonexistent_custom")

	skills, err := DiscoverFromDirs(builtin, custom)
	if err != nil {
		t.Fatalf("DiscoverFromDirs() unexpected error: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("discovered %d skills from missing dirs, want 0", len(skills))
	}
}

// TestDiscoverSkipsNonMd verifies that non-.md files are skipped.
func TestDiscoverSkipsNonMd(t *testing.T) {
	builtin := filepath.Join(t.TempDir(), "builtin")
	custom := filepath.Join(t.TempDir(), "custom")

	writeSkill(t, builtin, "valid.md", "[DESCRIPTION]\nValid.\n\n[DETAILS]\nValid details.")
	writeSkill(t, builtin, "readme.txt", "not a skill")

	skills, err := DiscoverFromDirs(builtin, custom)
	if err != nil {
		t.Fatalf("DiscoverFromDirs() unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Errorf("discovered %d skills, want 1 (txt skipped)", len(skills))
	}
}

// TestDiscoverSkipsDirs verifies that subdirectories are skipped.
func TestDiscoverSkipsDirs(t *testing.T) {
	builtin := filepath.Join(t.TempDir(), "builtin")
	custom := filepath.Join(t.TempDir(), "custom")

	writeSkill(t, builtin, "valid.md", "[DESCRIPTION]\nValid.\n\n[DETAILS]\nValid details.")
	subdir := filepath.Join(builtin, "subdir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("cannot create subdir: %v", err)
	}

	skills, err := DiscoverFromDirs(builtin, custom)
	if err != nil {
		t.Fatalf("DiscoverFromDirs() unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Errorf("discovered %d skills, want 1 (subdir skipped)", len(skills))
	}
}

// TestSortedNames verifies alphabetical ordering.
func TestSortedNames(t *testing.T) {
	skills := map[string]*Skill{
		"zebra":  {},
		"apple":  {},
		"mango":  {},
	}
	names := SortedNames(skills)
	if len(names) != 3 {
		t.Fatalf("SortedNames() = %d items, want 3", len(names))
	}
	if names[0] != "apple" || names[1] != "mango" || names[2] != "zebra" {
		t.Errorf("SortedNames() = %v, want [apple mango zebra]", names)
	}
}
