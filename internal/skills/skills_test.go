// skills_test.go — tests for skill parsing, discovery, collision, and seeding.
// Uses temp directories to avoid touching the real app home.
package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"blazeai/internal/platform"
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
	content = strings.ReplaceAll(content, "[BEHAVIOR]", "[BODY]")
	content = strings.ReplaceAll(content, "[DATA]", "")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("cannot write custom skill %s: %v", path, err)
	}
	return path
}

// discoverFromRoots merges two explicit global-scope skill directories for test coverage.
func discoverFromRoots(primaryRoot, overrideRoot string) (map[string]*Skill, error) {
	merged := make(map[string]*Skill)
	primary, err := DiscoverGlobalFromDir(primaryRoot)
	if err != nil {
		return nil, err
	}
	for id, skill := range primary {
		merged[id] = skill
	}
	override, err := DiscoverGlobalFromDir(overrideRoot)
	if err != nil {
		return nil, err
	}
	for id, skill := range override {
		merged[id] = skill
	}
	return merged, nil
}

// TestParseValid verifies that a well-formed skill file parses correctly.
func TestParseValid(t *testing.T) {
	content := `[DESCRIPTION]
A test skill for testing purposes.

[BODY]
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
	if skill.Body == "" {
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

// TestParseMissingBody verifies error when [BODY] is absent.
func TestParseMissingBody(t *testing.T) {
	content := `[DESCRIPTION]
Only description here.`
	_, err := Parse("test", content)
	if err != ErrMissingBody {
		t.Errorf("Parse() err = %v, want ErrMissingBody", err)
	}
}

// TestParseBothMissing verifies error when neither section is present.
func TestParseBothMissing(t *testing.T) {
	_, err := Parse("test", "no sections at all")
	if err == nil {
		t.Fatal("Parse() expected error, got nil")
	}
}

// TestParseBodyAtEnd verifies that [BODY] as the last section is captured fully.
func TestParseBehaviorAtEnd(t *testing.T) {
	content := "[DESCRIPTION]\nShort desc.\n\n[BODY]\nLine 1\nLine 2\nLine 3"
	skill, err := Parse("test", content)
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}
	if !contains(skill.Body, "Line 1") || !contains(skill.Body, "Line 3") {
		t.Errorf("Body = %q, want all three lines", skill.Body)
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

// TestDiscoverFromDirs verifies discovery from two directories.
func TestDiscoverFromDirs(t *testing.T) {
	builtin := filepath.Join(t.TempDir(), "builtin")
	custom := filepath.Join(t.TempDir(), "custom")

	writeCustomSkill(t, builtin, "memory-manager", "[DESCRIPTION]\nBuiltin memory manager.\n\n[BEHAVIOR]\nBuiltin details.")
	writeCustomSkill(t, builtin, "skill-manager", "[DESCRIPTION]\nBuiltin skill manager.\n\n[BEHAVIOR]\nBuiltin skill manager details.")
	writeCustomSkill(t, custom, "my_skill", "[DESCRIPTION]\nCustom skill.\n\n[BEHAVIOR]\nCustom details.")

	skills, err := discoverFromRoots(builtin, custom)
	if err != nil {
		t.Fatalf("discoverFromRoots() unexpected error: %v", err)
	}
	if len(skills) != 3 {
		t.Fatalf("discovered %d skills, want 3", len(skills))
	}
	if skills["global/memory-manager"] == nil {
		t.Error("memory-manager skill not found")
	}
	if skills["global/skill-manager"] == nil {
		t.Error("skill-manager skill not found")
	}
	if skills["global/my_skill"] == nil {
		t.Error("my_skill skill not found")
	}
}

// TestDiscoverCollisionCustomWins verifies that custom skills override builtin by name.
func TestDiscoverCollisionCustomWins(t *testing.T) {
	builtin := filepath.Join(t.TempDir(), "builtin")
	custom := filepath.Join(t.TempDir(), "custom")

	writeCustomSkill(t, builtin, "memory-manager", "[DESCRIPTION]\nBuiltin memory manager.\n\n[BEHAVIOR]\nBuiltin details.")
	writeCustomSkill(t, custom, "memory-manager", "[DESCRIPTION]\nCustom memory manager.\n\n[BEHAVIOR]\nCustom details.")

	skills, err := discoverFromRoots(builtin, custom)
	if err != nil {
		t.Fatalf("discoverFromRoots() unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("discovered %d skills, want 1 (collision resolved)", len(skills))
	}
	if skills["global/memory-manager"].Description != "Custom memory manager." {
		t.Errorf("collision: Description = %q, want 'Custom memory manager.'", skills["global/memory-manager"].Description)
	}
}

// TestDiscoverSkipsInvalid verifies that invalid skill files are skipped silently.
func TestDiscoverSkipsInvalid(t *testing.T) {
	builtin := filepath.Join(t.TempDir(), "builtin")
	custom := filepath.Join(t.TempDir(), "custom")

	writeCustomSkill(t, builtin, "valid", "[DESCRIPTION]\nValid.\n\n[BEHAVIOR]\nValid details.")
	writeCustomSkill(t, builtin, "invalid", "no sections here")

	_, err := discoverFromRoots(builtin, custom)
	if err == nil || !strings.Contains(err.Error(), filepath.Join("invalid", "skill.md")) {
		t.Fatalf("discoverFromRoots() error = %v, want contextual malformed-file error", err)
	}
}

// TestDiscoverCustomSkillFolder verifies custom skills are loaded from folder/skill.md layout.
func TestDiscoverCustomSkillFolder(t *testing.T) {
	builtin := filepath.Join(t.TempDir(), "builtin")
	custom := filepath.Join(t.TempDir(), "custom")

	writeCustomSkill(t, custom, "project_hub", "[DESCRIPTION]\nFolder custom skill.\n\n[BEHAVIOR]\nFolder details.")

	discovered, err := discoverFromRoots(builtin, custom)
	if err != nil {
		t.Fatalf("discoverFromRoots() unexpected error: %v", err)
	}
	skill := discovered["global/project_hub"]
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

	discovered, err := discoverFromRoots(builtin, custom)
	if err != nil {
		t.Fatalf("discoverFromRoots() unexpected error: %v", err)
	}
	if len(discovered) != 0 {
		t.Fatalf("discovered %d skills, want 0", len(discovered))
	}
}

// TestDiscoverMissingDir verifies that a missing directory is not an error.
func TestDiscoverMissingDir(t *testing.T) {
	builtin := filepath.Join(t.TempDir(), "nonexistent_builtin")
	custom := filepath.Join(t.TempDir(), "nonexistent_custom")

	skills, err := discoverFromRoots(builtin, custom)
	if err != nil {
		t.Fatalf("discoverFromRoots() unexpected error: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("discovered %d skills from missing dirs, want 0", len(skills))
	}
}

// TestDiscoverSkipsNonMd verifies that non-.md files are skipped.
func TestDiscoverSkipsNonMd(t *testing.T) {
	builtin := filepath.Join(t.TempDir(), "builtin")
	custom := filepath.Join(t.TempDir(), "custom")

	writeCustomSkill(t, builtin, "valid", "[DESCRIPTION]\nValid.\n\n[BEHAVIOR]\nValid details.")
	writeSkill(t, builtin, "readme.txt", "not a skill")

	skills, err := discoverFromRoots(builtin, custom)
	if err != nil {
		t.Fatalf("discoverFromRoots() unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Errorf("discovered %d skills, want 1 (txt skipped)", len(skills))
	}
}

// TestDiscoverSkipsDirs verifies that subdirectories are skipped.
func TestDiscoverSkipsDirs(t *testing.T) {
	builtin := filepath.Join(t.TempDir(), "builtin")
	custom := filepath.Join(t.TempDir(), "custom")

	writeCustomSkill(t, builtin, "valid", "[DESCRIPTION]\nValid.\n\n[BEHAVIOR]\nValid details.")
	subdir := filepath.Join(builtin, "subdir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("cannot create subdir: %v", err)
	}

	skills, err := discoverFromRoots(builtin, custom)
	if err != nil {
		t.Fatalf("discoverFromRoots() unexpected error: %v", err)
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

func TestDiscoverAllBuiltinPrecedenceAndResolution(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	workDir := t.TempDir()
	home, _ := platform.AppHome()
	writeCustomSkill(t, filepath.Join(home, "skills"), "skill-manager", "[DESCRIPTION]\nGlobal.\n\n[BODY]\nGlobal body.")
	project, _ := platform.ProjectDir(workDir)
	writeCustomSkill(t, filepath.Join(project, "skills"), "skill-manager", "[DESCRIPTION]\nProject.\n\n[BODY]\nProject body.")
	builtin := fstest.MapFS{"skill-manager.md": &fstest.MapFile{Data: []byte("[DESCRIPTION]\nBuiltin.\n\n[BODY]\nBuiltin body.")}}
	got, err := DiscoverAll(workDir, builtin)
	if err != nil {
		t.Fatal(err)
	}
	if s := got["builtin/skill-manager"]; s == nil || s.Scope != ScopeBuiltin || s.Dir != "" || s.Body != "Builtin body." {
		t.Fatalf("builtin = %#v", s)
	}
	for _, id := range []string{"global/skill-manager", "project/skill-manager"} {
		if _, ok := got[id]; ok {
			t.Errorf("collision retained: %s", id)
		}
	}
	for _, name := range []string{"skill-manager", "skill-manager.md", "builtin/skill-manager"} {
		if id, err := Resolve(name, got); err != nil || id != "builtin/skill-manager" {
			t.Errorf("Resolve(%q) = %q, %v", name, id, err)
		}
	}
}

func TestDiscoverAllBuiltinErrorsAndIgnoredEntries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := DiscoverAll(t.TempDir(), nil); err == nil || !strings.Contains(err.Error(), "builtin skills filesystem is nil") {
		t.Fatalf("nil error = %v", err)
	}
	bad := fstest.MapFS{"broken.md": &fstest.MapFile{Data: []byte("invalid")}, "docs/readme.md": &fstest.MapFile{Data: []byte("invalid")}, "notes.txt": &fstest.MapFile{Data: []byte("invalid")}}
	if _, err := DiscoverAll(t.TempDir(), bad); err == nil || !strings.Contains(err.Error(), "broken.md") {
		t.Fatalf("malformed error = %v", err)
	}
	valid := fstest.MapFS{"valid.md": &fstest.MapFile{Data: []byte("[DESCRIPTION]\nValid.\n\n[BODY]\nBody.")}, "docs/readme.md": &fstest.MapFile{Data: []byte("invalid")}, "notes.txt": &fstest.MapFile{Data: []byte("invalid")}}
	got, err := DiscoverAll(t.TempDir(), valid)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got["builtin/valid"] == nil {
		t.Fatalf("got skills = %#v", got)
	}
}
