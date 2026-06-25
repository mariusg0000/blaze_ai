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

// writeCustomSkill writes a custom skill folder with skill.md.
func writeCustomSkill(t *testing.T, root, name, content string) string {
	t.Helper()
	skillDir := filepath.Join(root, name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("cannot create custom skill dir %s: %v", skillDir, err)
	}
	path := filepath.Join(skillDir, "skill.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("cannot write custom skill %s: %v", path, err)
	}
	return path
}

// TestParseValid verifies that a well-formed skill file parses correctly.
func TestParseValid(t *testing.T) {
	content := `[DESCRIPTION]
A test skill for testing purposes.

[BEHAVIOR]
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
	if skill.Behavior == "" {
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

// TestParseMissingBehaviorOrData verifies error when neither [BEHAVIOR] nor [DATA] is present.
func TestParseMissingBehaviorOrData(t *testing.T) {
	content := `[DESCRIPTION]
Only description here.`
	_, err := Parse("test", content)
	if err != ErrMissingBehaviorOrData {
		t.Errorf("Parse() err = %v, want ErrMissingBehaviorOrData", err)
	}
}

// TestParseBothMissing verifies error when neither section is present.
func TestParseBothMissing(t *testing.T) {
	_, err := Parse("test", "no sections at all")
	if err == nil {
		t.Fatal("Parse() expected error, got nil")
	}
}

// TestParseBehaviorAtEnd verifies that [BEHAVIOR] as the last section is captured fully.
func TestParseBehaviorAtEnd(t *testing.T) {
	content := "[DESCRIPTION]\nShort desc.\n\n[BEHAVIOR]\nLine 1\nLine 2\nLine 3"
	skill, err := Parse("test", content)
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}
	if !contains(skill.Behavior, "Line 1") || !contains(skill.Behavior, "Line 3") {
		t.Errorf("Details = %q, want all three lines", skill.Behavior)
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
	a.Load("memory-manager")
	a.Load("skill-manager")
	if !a.Has("memory-manager") {
		t.Error("Has(memory-manager) = false, want true")
	}
	if !a.Has("skill-manager") {
		t.Error("Has(skill-manager) = false, want true")
	}
}

// TestActiveListLoadDuplicate verifies that loading the same skill twice does not duplicate.
func TestActiveListLoadDuplicate(t *testing.T) {
	a := NewActiveList()
	a.Load("memory-manager")
	a.Load("memory-manager")
	if len(a.List()) != 1 {
		t.Errorf("List() = %d items, want 1", len(a.List()))
	}
}

// TestActiveListUnload verifies removing a skill from the active list.
func TestActiveListUnload(t *testing.T) {
	a := NewActiveList()
	a.Load("memory-manager")
	a.Load("skill-manager")
	a.Unload("memory-manager")
	if a.Has("memory-manager") {
		t.Error("Has(memory-manager) = true after Unload, want false")
	}
	if !a.Has("skill-manager") {
		t.Error("Has(skill-manager) = false after Unload(memory-manager), want true")
	}
}

// TestActiveListUnloadNotPresent verifies that unloading a non-active skill is a no-op.
func TestActiveListUnloadNotPresent(t *testing.T) {
	a := NewActiveList()
	a.Load("memory-manager")
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
	a.Load("memory-manager")
	l := a.List()
	l[0] = "modified"
	if a.Has("modified") {
		t.Error("modifying List() result affected internal state")
	}
	if !a.Has("memory-manager") {
		t.Error("internal state was corrupted by List() modification")
	}
}

// TestDiscoverFromDirs verifies discovery from two directories.
func TestDiscoverFromDirs(t *testing.T) {
	builtin := filepath.Join(t.TempDir(), "builtin")
	custom := filepath.Join(t.TempDir(), "custom")

	writeSkill(t, builtin, "memory-manager.md", "[DESCRIPTION]\nBuiltin memory manager.\n\n[BEHAVIOR]\nBuiltin details.")
	writeSkill(t, builtin, "skill-manager.md", "[DESCRIPTION]\nBuiltin skill manager.\n\n[BEHAVIOR]\nBuiltin skill manager details.")
	writeCustomSkill(t, custom, "my_skill", "[DESCRIPTION]\nCustom skill.\n\n[BEHAVIOR]\nCustom details.")

	skills, err := DiscoverFromDirs(builtin, custom)
	if err != nil {
		t.Fatalf("DiscoverFromDirs() unexpected error: %v", err)
	}
	if len(skills) != 3 {
		t.Fatalf("discovered %d skills, want 3", len(skills))
	}
	if skills["memory-manager"] == nil {
		t.Error("memory-manager skill not found")
	}
	if skills["skill-manager"] == nil {
		t.Error("skill-manager skill not found")
	}
	if skills["my_skill"] == nil {
		t.Error("my_skill skill not found")
	}
}

// TestDiscoverCollisionCustomWins verifies that custom skills override builtin by name.
func TestDiscoverCollisionCustomWins(t *testing.T) {
	builtin := filepath.Join(t.TempDir(), "builtin")
	custom := filepath.Join(t.TempDir(), "custom")

	writeSkill(t, builtin, "memory-manager.md", "[DESCRIPTION]\nBuiltin memory manager.\n\n[BEHAVIOR]\nBuiltin details.")
	writeCustomSkill(t, custom, "memory-manager", "[DESCRIPTION]\nCustom memory manager.\n\n[BEHAVIOR]\nCustom details.")

	skills, err := DiscoverFromDirs(builtin, custom)
	if err != nil {
		t.Fatalf("DiscoverFromDirs() unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("discovered %d skills, want 1 (collision resolved)", len(skills))
	}
	if skills["memory-manager"].Description != "Custom memory manager." {
		t.Errorf("collision: Description = %q, want 'Custom memory manager.'", skills["memory-manager"].Description)
	}
}

// TestDiscoverSkipsInvalid verifies that invalid skill files are skipped silently.
func TestDiscoverSkipsInvalid(t *testing.T) {
	builtin := filepath.Join(t.TempDir(), "builtin")
	custom := filepath.Join(t.TempDir(), "custom")

	writeSkill(t, builtin, "valid.md", "[DESCRIPTION]\nValid.\n\n[BEHAVIOR]\nValid details.")
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

// TestDiscoverCustomSkillFolder verifies custom skills are loaded from folder/skill.md layout.
func TestDiscoverCustomSkillFolder(t *testing.T) {
	builtin := filepath.Join(t.TempDir(), "builtin")
	custom := filepath.Join(t.TempDir(), "custom")

	writeCustomSkill(t, custom, "project_hub", "[DESCRIPTION]\nFolder custom skill.\n\n[BEHAVIOR]\nFolder details.")

	discovered, err := DiscoverFromDirs(builtin, custom)
	if err != nil {
		t.Fatalf("DiscoverFromDirs() unexpected error: %v", err)
	}
	skill := discovered["project_hub"]
	if skill == nil {
		t.Fatal("project_hub skill not found")
	}
	if skill.Dir != filepath.Join(custom, "project_hub") {
		t.Errorf("Dir = %q, want %q", skill.Dir, filepath.Join(custom, "project_hub"))
	}
}

// TestDiscoverCustomSkillFolderMissingMainFile verifies missing skill.md is skipped.
func TestDiscoverCustomSkillFolderMissingMainFile(t *testing.T) {
	builtin := filepath.Join(t.TempDir(), "builtin")
	custom := filepath.Join(t.TempDir(), "custom")
	if err := os.MkdirAll(filepath.Join(custom, "broken_skill"), 0755); err != nil {
		t.Fatalf("cannot create broken skill dir: %v", err)
	}

	discovered, err := DiscoverFromDirs(builtin, custom)
	if err != nil {
		t.Fatalf("DiscoverFromDirs() unexpected error: %v", err)
	}
	if len(discovered) != 0 {
		t.Fatalf("discovered %d skills, want 0", len(discovered))
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

	writeSkill(t, builtin, "valid.md", "[DESCRIPTION]\nValid.\n\n[BEHAVIOR]\nValid details.")
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

	writeSkill(t, builtin, "valid.md", "[DESCRIPTION]\nValid.\n\n[BEHAVIOR]\nValid details.")
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
		"zebra": {},
		"apple": {},
		"mango": {},
	}
	names := SortedNames(skills)
	if len(names) != 3 {
		t.Fatalf("SortedNames() = %d items, want 3", len(names))
	}
	if names[0] != "apple" || names[1] != "mango" || names[2] != "zebra" {
		t.Errorf("SortedNames() = %v, want [apple mango zebra]", names)
	}
}
