// prompt_test.go — tests for variable injection, source reading, and prompt assembly.
// Uses temp directories to avoid touching the real app home and project files.
package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blazeai/internal/platform"
	"blazeai/internal/session"
	"blazeai/internal/skills"
)

// setupTestDirs creates temp directories with prompt and skill files for testing.
// Returns promptsDir, builtinSkillsDir, workDir.
func setupTestDirs(t *testing.T) (promptsDir, builtinSkillsDir, workDir string) {
	t.Helper()
	root := t.TempDir()
	promptsDir = filepath.Join(root, "prompts")
	builtinSkillsDir = filepath.Join(root, "skills")
	workDir = filepath.Join(root, "work")

	for _, dir := range []string{promptsDir, builtinSkillsDir, workDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("cannot create dir %s: %v", dir, err)
		}
	}

	// Universal prompt with {APP_HOME} variable.
	writeFile(t, filepath.Join(promptsDir, "sysprompt.md"),
		"# Universal System Prompt\n\nApp home is at {APP_HOME}.\nUnknown var: {UNKNOWN_VAR}.\n")

	// OS prompt.
	writeFile(t, filepath.Join(promptsDir, "sysprompt.linux.md"),
		"# Linux System Prompt\n\nScripts at {APP_HOME}/scripts/.\n")

	// Builtin skill.
	writeFile(t, filepath.Join(builtinSkillsDir, "memory.md"),
		"[DESCRIPTION]\nMemory management skill.\n\n[DETAILS]\nMemory lives at {APP_HOME}/memory/memory.md.\n")

	// AGENTS.md in work dir.
	writeFile(t, filepath.Join(workDir, "AGENTS.md"),
		"# Project Rules\n\nUse {APP_HOME} for paths.\n")

	return
}

// writeFile writes content to a path, failing the test on error.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("cannot write %s: %v", path, err)
	}
}

// TestInjectVariablesAPP_HOME verifies that {APP_HOME} is replaced.
func TestInjectVariablesAPP_HOME(t *testing.T) {
	b := &Builder{}
	home, _ := platform.AppHome()
	result, err := b.injectVariables("Path: {APP_HOME}/scripts")
	if err != nil {
		t.Fatalf("injectVariables() error: %v", err)
	}
	expected := "Path: " + home + "/scripts"
	if result != expected {
		t.Errorf("injectVariables() = %q, want %q", result, expected)
	}
}

// TestInjectVariablesUnknownLeftAsIs verifies that unknown variables are preserved.
func TestInjectVariablesUnknownLeftAsIs(t *testing.T) {
	b := &Builder{}
	result, err := b.injectVariables("Unknown: {MY_VAR} and {APP_HOME}")
	if err != nil {
		t.Fatalf("injectVariables() error: %v", err)
	}
	if !strings.Contains(result, "{MY_VAR}") {
		t.Errorf("injectVariables() = %q, unknown var was replaced", result)
	}
}

// TestInjectVariablesMultiple verifies multiple {APP_HOME} in one text.
func TestInjectVariablesMultiple(t *testing.T) {
	b := &Builder{}
	home, _ := platform.AppHome()
	result, err := b.injectVariables("{APP_HOME} and {APP_HOME} and {APP_HOME}")
	if err != nil {
		t.Fatalf("injectVariables() error: %v", err)
	}
	expected := home + " and " + home + " and " + home
	if result != expected {
		t.Errorf("injectVariables() = %q, want %q", result, expected)
	}
}

// TestInjectVariablesNoPlaceholders verifies plain text passes through unchanged.
func TestInjectVariablesNoPlaceholders(t *testing.T) {
	b := &Builder{}
	input := "plain text without placeholders"
	result, err := b.injectVariables(input)
	if err != nil {
		t.Fatalf("injectVariables() error: %v", err)
	}
	if result != input {
		t.Errorf("injectVariables() = %q, want %q", result, input)
	}
}

// TestBuildRuntimePartFull verifies the full runtime part with all sources.
func TestBuildRuntimePartFull(t *testing.T) {
	promptsDir, builtinSkillsDir, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsDir:       promptsDir,
		BuiltinSkillsDir: builtinSkillsDir,
		WorkDir:          workDir,
		OS:               platform.Linux,
	}
	active := skills.NewActiveList()
	result, err := b.BuildRuntimePart(active)
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}

	home, _ := platform.AppHome()

	// Universal prompt present with APP_HOME replaced.
	if !strings.Contains(result, "Universal System Prompt") {
		t.Error("runtime part missing universal prompt")
	}
	if !strings.Contains(result, home) {
		t.Error("runtime part missing replaced APP_HOME")
	}
	if strings.Contains(result, "{APP_HOME}") {
		t.Error("runtime part still contains unreplaced {APP_HOME}")
	}

	// OS prompt present.
	if !strings.Contains(result, "Linux System Prompt") {
		t.Error("runtime part missing OS prompt")
	}

	// AGENTS.md present.
	if !strings.Contains(result, "Project Rules") {
		t.Error("runtime part missing AGENTS.md")
	}

	// Unknown variable preserved.
	if !strings.Contains(result, "{UNKNOWN_VAR}") {
		t.Error("runtime part missing unknown variable (should be left as-is)")
	}

	// Skills section present.
	if !strings.Contains(result, "Available Skills") {
		t.Error("runtime part missing skills section")
	}
	if !strings.Contains(result, "memory.md") {
		t.Error("runtime part missing skill file name")
	}
}

// TestBuildRuntimePartMissingUniversal verifies error when universal prompt is missing.
func TestBuildRuntimePartMissingUniversal(t *testing.T) {
	root := t.TempDir()
	b := &Builder{
		PromptsDir:       filepath.Join(root, "noprompts"),
		BuiltinSkillsDir: filepath.Join(root, "skills"),
		WorkDir:          root,
		OS:               platform.Linux,
	}
	_, err := b.BuildRuntimePart(skills.NewActiveList())
	if err != ErrUniversalPromptMissing {
		t.Errorf("BuildRuntimePart() err = %v, want ErrUniversalPromptMissing", err)
	}
}

// TestBuildRuntimePartMissingOSPrompt verifies error when OS prompt is missing.
func TestBuildRuntimePartMissingOSPrompt(t *testing.T) {
	root := t.TempDir()
	promptsDir := filepath.Join(root, "prompts")
	os.MkdirAll(promptsDir, 0755)
	writeFile(t, filepath.Join(promptsDir, "sysprompt.md"), "universal")

	b := &Builder{
		PromptsDir:       promptsDir,
		BuiltinSkillsDir: filepath.Join(root, "skills"),
		WorkDir:          root,
		OS:               platform.Linux,
	}
	_, err := b.BuildRuntimePart(skills.NewActiveList())
	if err != ErrOSPromptMissing {
		t.Errorf("BuildRuntimePart() err = %v, want ErrOSPromptMissing", err)
	}
}

// TestBuildRuntimePartNoAgentsMD verifies that missing AGENTS.md is omitted silently.
func TestBuildRuntimePartNoAgentsMD(t *testing.T) {
	promptsDir, builtinSkillsDir, _ := setupTestDirs(t)
	// Use a work dir without AGENTS.md.
	emptyWork := t.TempDir()

	b := &Builder{
		PromptsDir:       promptsDir,
		BuiltinSkillsDir: builtinSkillsDir,
		WorkDir:          emptyWork,
		OS:               platform.Linux,
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}
	if strings.Contains(result, "Project Rules") {
		t.Error("runtime part should not contain AGENTS.md content")
	}
}

// TestBuildRuntimePartActiveSkills verifies that active skills inject [DETAILS].
func TestBuildRuntimePartActiveSkills(t *testing.T) {
	promptsDir, builtinSkillsDir, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsDir:       promptsDir,
		BuiltinSkillsDir: builtinSkillsDir,
		WorkDir:          workDir,
		OS:               platform.Linux,
	}
	active := skills.NewActiveList()
	active.Load("memory")

	result, err := b.BuildRuntimePart(active)
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}
	if !strings.Contains(result, "Active Skills") {
		t.Error("runtime part missing Active Skills section")
	}
	if !strings.Contains(result, "Memory lives at") {
		t.Error("runtime part missing active skill details")
	}
}

// TestBuildRuntimePartNoSkills verifies that missing skills are omitted silently.
func TestBuildRuntimePartNoSkills(t *testing.T) {
	root := t.TempDir()
	promptsDir := filepath.Join(root, "prompts")
	os.MkdirAll(promptsDir, 0755)
	writeFile(t, filepath.Join(promptsDir, "sysprompt.md"), "universal")
	writeFile(t, filepath.Join(promptsDir, "sysprompt.linux.md"), "linux")

	b := &Builder{
		PromptsDir:       promptsDir,
		BuiltinSkillsDir: filepath.Join(root, "noskills"),
		WorkDir:          root,
		OS:               platform.Linux,
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}
	if strings.Contains(result, "Available Skills") {
		t.Error("runtime part should not contain skills section when no skills exist")
	}
}

// TestBuild verifies the full prompt assembly with session messages.
func TestBuild(t *testing.T) {
	promptsDir, builtinSkillsDir, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsDir:       promptsDir,
		BuiltinSkillsDir: builtinSkillsDir,
		WorkDir:          workDir,
		OS:               platform.Linux,
	}

	sess := &session.Session{
		Messages: []session.Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi there"},
		},
	}

	messages, err := b.Build(sess, skills.NewActiveList())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Expect: 1 system + 2 session = 3 messages.
	if len(messages) != 3 {
		t.Fatalf("Build() returned %d messages, want 3", len(messages))
	}
	if messages[0].Role != "system" {
		t.Errorf("messages[0].Role = %q, want 'system'", messages[0].Role)
	}
	if messages[1].Role != "user" {
		t.Errorf("messages[1].Role = %q, want 'user'", messages[1].Role)
	}
	if messages[2].Role != "assistant" {
		t.Errorf("messages[2].Role = %q, want 'assistant'", messages[2].Role)
	}
}

// TestBuildEmptySession verifies that Build works with an empty session.
func TestBuildEmptySession(t *testing.T) {
	promptsDir, builtinSkillsDir, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsDir:       promptsDir,
		BuiltinSkillsDir: builtinSkillsDir,
		WorkDir:          workDir,
		OS:               platform.Linux,
	}

	sess := &session.Session{Messages: []session.Message{}}

	messages, err := b.Build(sess, skills.NewActiveList())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("Build() returned %d messages, want 1 (system only)", len(messages))
	}
	if messages[0].Role != "system" {
		t.Errorf("messages[0].Role = %q, want 'system'", messages[0].Role)
	}
}

// TestBuildRuntimePartOrder verifies source order: universal → OS → AGENTS → memory → skills.
func TestBuildRuntimePartOrder(t *testing.T) {
	promptsDir, builtinSkillsDir, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsDir:       promptsDir,
		BuiltinSkillsDir: builtinSkillsDir,
		WorkDir:          workDir,
		OS:               platform.Linux,
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}

	// Check relative positions.
	universalIdx := strings.Index(result, "Universal System Prompt")
	osIdx := strings.Index(result, "Linux System Prompt")
	agentsIdx := strings.Index(result, "Project Rules")
	skillsIdx := strings.Index(result, "Available Skills")

	if universalIdx < 0 || osIdx < 0 || agentsIdx < 0 || skillsIdx < 0 {
		t.Fatalf("missing sections: universal=%d os=%d agents=%d skills=%d",
			universalIdx, osIdx, agentsIdx, skillsIdx)
	}
	if !(universalIdx < osIdx && osIdx < agentsIdx && agentsIdx < skillsIdx) {
		t.Errorf("wrong order: universal=%d os=%d agents=%d skills=%d",
			universalIdx, osIdx, agentsIdx, skillsIdx)
	}
}
