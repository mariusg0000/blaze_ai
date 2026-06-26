// prompt_test.go — tests for variable injection, source reading, and prompt assembly.
// Uses temp directories to avoid touching the real app home and project files.
package prompt

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blazeai/internal/config"
	"blazeai/internal/helpers"
	"blazeai/internal/platform"
	"blazeai/internal/session"
	"blazeai/internal/skills"
)

// setupTestDirs creates temp directories with prompt and skill files for testing.
// Returns promptsFS and workDir. Sets HOME to temp dir for isolation.
func setupTestDirs(t *testing.T) (promptsFS fs.FS, workDir string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	root := t.TempDir()
	promptsDir := filepath.Join(root, "prompts")
	workDir = filepath.Join(root, "work")

	for _, dir := range []string{promptsDir, workDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("cannot create dir %s: %v", dir, err)
		}
	}

	// Universal prompt with {APP_HOME} variable.
	writePromptFixtures(t, promptsDir)

	appHome, err := platform.AppHome()
	if err != nil {
		t.Fatalf("platform.AppHome() error: %v", err)
	}

	// Global skill.
	writeFile(t, filepath.Join(appHome, "skills", "skill-manager", "skill.md"),
		"[DESCRIPTION]\nLoad when creating, reviewing, or modifying a skill.\n\n[BEHAVIOR]\n# Skill Manager\n\nManage skills at {APP_HOME}/skills/.\n")
	customSkillDir := filepath.Join(appHome, "skills", "project_hub")
	writeFile(t, filepath.Join(customSkillDir, "skill.md"),
		"[DESCRIPTION]\nProject Hub skill with local scripts at {SKILL_DIR}/scripts/run.py.\n\n[BEHAVIOR]\nUse local helper at {SKILL_DIR}/scripts/run.py.\n[DATA]\napi.url=https://example.com\n")

	// AGENTS.md in work dir.
	writeFile(t, filepath.Join(workDir, "AGENTS.md"),
		"# Project Rules\n\nUse {APP_HOME} for paths.\n")

	promptsFS = os.DirFS(promptsDir)
	return
}

// writeFile writes content to a path, failing the test on error.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("cannot create parent dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("cannot write %s: %v", path, err)
	}
}

// writePromptFixtures creates the prompt templates required by runtime prompt assembly.
func writePromptFixtures(t *testing.T, promptsDir string) {
	t.Helper()
	writeFile(t, filepath.Join(promptsDir, "sysprompt.md"), "# Universal System Prompt\n\nApp home is at {APP_HOME}.\nUnknown var: {UNKNOWN_VAR}.\n\n{OS_PROMPT}\n\n## Transport\n{TRANSPORT_CONTEXT}\n\n## Host Environment Helpers\n{HOST_HELPERS_ADVISORY}\n\nAvailable helpers:\n{HOST_HELPERS_AVAILABLE}\n\nOptional helpers:\n{HOST_HELPERS_OPTIONAL}\n\n## Skills\nAvailable skills:\n{SKILLS_AVAILABLE}\n\nActive skills:\n{SKILLS_ACTIVE}\n\n## Project Rules (AGENTS.md)\n{AGENTS_CONTENT}\n")
	writeFile(t, filepath.Join(promptsDir, "sysprompt.linux.md"), "# Linux System Prompt\n\nScripts at {APP_HOME}/scripts/.\n")
}

// TestInjectVariablesWorkDir verifies that {WORK_DIR} is replaced.
func TestInjectVariablesWorkDir(t *testing.T) {
	b := &Builder{WorkDir: "/some/path"}
	result, err := b.injectVariables("Work dir: {WORK_DIR}")
	if err != nil {
		t.Fatalf("injectVariables() error: %v", err)
	}
	expected := "Work dir: /some/path"
	if result != expected {
		t.Errorf("injectVariables() = %q, want %q", result, expected)
	}
}

// TestInjectVariablesOSInfo verifies that {OS_INFO} is replaced.
func TestInjectVariablesOSInfo(t *testing.T) {
	b := &Builder{OSInfo: "Ubuntu 24.04 LTS"}
	result, err := b.injectVariables("OS: {OS_INFO}")
	if err != nil {
		t.Fatalf("injectVariables() error: %v", err)
	}
	expected := "OS: Ubuntu 24.04 LTS"
	if result != expected {
		t.Errorf("injectVariables() = %q, want %q", result, expected)
	}
}

// TestInjectVariablesAll verifies all variables in one call.
func TestInjectVariablesAll(t *testing.T) {
	home, _ := platform.AppHome()
	b := &Builder{WorkDir: "/tmp", OSInfo: "Linux"}
	result, err := b.injectVariables("{APP_HOME} {WORK_DIR} {OS_INFO}")
	if err != nil {
		t.Fatalf("injectVariables() error: %v", err)
	}
	expected := home + " /tmp Linux"
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

// TestInjectVariablesEscapeBraces verifies that escaped braces remain literal.
func TestInjectVariablesEscapeBraces(t *testing.T) {
	b := &Builder{}
	home, _ := platform.AppHome()
	result, err := b.injectVariables("Literal: \\{APP_HOME\\}, injected: {APP_HOME}")
	if err != nil {
		t.Fatalf("injectVariables() error: %v", err)
	}
	expected := "Literal: {APP_HOME}, injected: " + home
	if result != expected {
		t.Errorf("injectVariables() = %q, want %q", result, expected)
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

// TestInjectVariablesForSkillEscapeBraces verifies escapes also work in skill text.
func TestInjectVariablesForSkillEscapeBraces(t *testing.T) {
	b := &Builder{}
	result, err := b.injectVariablesForSkill("Use \\{SKILL_DIR\\}/data and {SKILL_DIR}/scripts", "/tmp/project_hub")
	if err != nil {
		t.Fatalf("injectVariablesForSkill() error: %v", err)
	}
	expected := "Use {SKILL_DIR}/data and /tmp/project_hub/scripts"
	if result != expected {
		t.Errorf("injectVariablesForSkill() = %q, want %q", result, expected)
	}
}

// TestBuildRuntimePartFull verifies the full runtime part with all sources.
func TestBuildRuntimePartFull(t *testing.T) {
	promptsFS, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS: promptsFS,
		WorkDir:   workDir,
		OS:        platform.Linux,
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
	if !strings.Contains(result, "Available skills:") {
		t.Error("runtime part missing skills section")
	}
	if !strings.Contains(result, "skill-manager") {
		t.Error("runtime part missing skill name")
	}
}

// TestBuildRuntimePartMissingUniversal verifies error when universal prompt is missing.
func TestBuildRuntimePartMissingUniversal(t *testing.T) {
	root := t.TempDir()
	b := &Builder{
		PromptsFS: os.DirFS(filepath.Join(root, "noprompts")),
		WorkDir:   root,
		OS:        platform.Linux,
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
	writeFile(t, filepath.Join(promptsDir, "sysprompt.md"), "universal {OS_PROMPT}")

	b := &Builder{
		PromptsFS: os.DirFS(promptsDir),
		WorkDir:   root,
		OS:        platform.Linux,
	}
	_, err := b.BuildRuntimePart(skills.NewActiveList())
	if err != ErrOSPromptMissing {
		t.Errorf("BuildRuntimePart() err = %v, want ErrOSPromptMissing", err)
	}
}

// TestBuildRuntimePartNoAgentsMD verifies that missing AGENTS.md renders as NULL.
func TestBuildRuntimePartNoAgentsMD(t *testing.T) {
	promptsFS, _ := setupTestDirs(t)
	// Use a work dir without AGENTS.md.
	emptyWork := t.TempDir()

	b := &Builder{
		PromptsFS: promptsFS,
		WorkDir:   emptyWork,
		OS:        platform.Linux,
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}
	if !strings.Contains(result, "Project Rules (AGENTS.md)") {
		t.Error("runtime part missing AGENTS.md section")
	}
	if !strings.Contains(result, "Project Rules (AGENTS.md)\nNULL") {
		t.Error("runtime part should render NULL for missing AGENTS.md")
	}
}

// TestBuildRuntimePartActiveSkills verifies that active skills inject Behavior and Data.
func TestBuildRuntimePartActiveSkills(t *testing.T) {
	promptsFS, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS: promptsFS,
		WorkDir:   workDir,
		OS:        platform.Linux,
	}
	active := skills.NewActiveList()
	active.Load("global/project_hub")

	result, err := b.BuildRuntimePart(active)
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}
	if !strings.Contains(result, "Active skills:") {
		t.Error("runtime part missing Active skills section")
	}
	if !strings.Contains(result, "Use local helper") {
		t.Error("runtime part missing active skill behavior")
	}
	if !strings.Contains(result, "api.url") {
		t.Error("runtime part missing active skill data")
	}
}

// TestBuildRuntimePartTransportContext verifies optional transport guidance injection.
func TestBuildRuntimePartTransportContext(t *testing.T) {
	promptsFS, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS:        promptsFS,
		WorkDir:          workDir,
		OS:               platform.Linux,
		TransportContext: "Telegram bridge active.",
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}
	if !strings.Contains(result, "## Transport") {
		t.Fatal("runtime part missing transport section")
	}
	if !strings.Contains(result, "Telegram bridge active.") {
		t.Fatal("runtime part missing injected transport context")
	}
}

// TestInjectVariablesForSkillDir verifies {SKILL_DIR} is injected for skill-scoped content.
func TestInjectVariablesForSkillDir(t *testing.T) {
	b := &Builder{WorkDir: "/tmp/work", OSInfo: "Linux"}
	result, err := b.injectVariablesForSkill("Run {SKILL_DIR}/scripts/run.py", "/tmp/skill/project_hub")
	if err != nil {
		t.Fatalf("injectVariablesForSkill() error: %v", err)
	}
	expected := "Run /tmp/skill/project_hub/scripts/run.py"
	if result != expected {
		t.Fatalf("injectVariablesForSkill() = %q, want %q", result, expected)
	}
}

// TestBuildRuntimePartNoSkills verifies that missing skills are omitted silently.
func TestBuildRuntimePartNoSkills(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	promptsDir := filepath.Join(root, "prompts")
	os.MkdirAll(promptsDir, 0755)
	writePromptFixtures(t, promptsDir)

	b := &Builder{
		PromptsFS: os.DirFS(promptsDir),
		WorkDir:   root,
		OS:        platform.Linux,
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}
	if !strings.Contains(result, "Available skills:\nNULL") {
		t.Error("runtime part should render NULL for missing skills")
	}
}

// TestBuild verifies the full prompt assembly with session messages.
func TestBuild(t *testing.T) {
	promptsFS, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS: promptsFS,
		WorkDir:   workDir,
		OS:        platform.Linux,
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
	promptsFS, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS: promptsFS,
		WorkDir:   workDir,
		OS:        platform.Linux,
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

// TestBuildRuntimePartOrder verifies source order: universal → OS → helpers → skills → AGENTS.
func TestBuildRuntimePartOrder(t *testing.T) {
	promptsFS, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS:    promptsFS,
		WorkDir:      workDir,
		OS:           platform.Linux,
		HelperLookup: fakeHelperLookup([]string{"rg", "fd", "jq", "git", "curl"}),
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}

	universalIdx := strings.Index(result, "Universal System Prompt")
	osIdx := strings.Index(result, "Linux System Prompt")
	helpersIdx := strings.Index(result, "Host Environment Helpers")
	skillsIdx := strings.Index(result, "Available skills:")
	agentsIdx := strings.Index(result, "Project Rules")

	if universalIdx < 0 || osIdx < 0 || helpersIdx < 0 || skillsIdx < 0 || agentsIdx < 0 {
		t.Fatalf("missing sections: universal=%d os=%d helpers=%d skills=%d agents=%d",
			universalIdx, osIdx, helpersIdx, skillsIdx, agentsIdx)
	}
	if !(universalIdx < osIdx && osIdx < helpersIdx && helpersIdx < skillsIdx && skillsIdx < agentsIdx) {
		t.Errorf("wrong order: universal=%d os=%d helpers=%d skills=%d agents=%d",
			universalIdx, osIdx, helpersIdx, skillsIdx, agentsIdx)
	}
}

// fakeLookup returns a helpers.LookupFunc that reports only the given names as available.
func fakeHelperLookup(names []string) helpers.LookupFunc {
	avail := make(map[string]bool)
	for _, n := range names {
		avail[n] = true
	}
	return func(name string) (string, error) {
		if avail[name] {
			return fmt.Sprintf("/usr/bin/%s", name), nil
		}
		return "", fmt.Errorf("not found")
	}
}

// TestBuildRuntimePartHelperAvailable verifies available helpers appear in the runtime part.
func TestBuildRuntimePartHelperAvailable(t *testing.T) {
	promptsFS, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS:    promptsFS,
		WorkDir:      workDir,
		OS:           platform.Linux,
		HelperSetup:  config.HelperSetup{},
		HelperLookup: fakeHelperLookup([]string{"rg", "jq", "curl"}),
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}
	if !strings.Contains(result, "## Host Environment Helpers") {
		t.Error("runtime part missing Host Environment Helpers section")
	}
	if !strings.Contains(result, "rg") || !strings.Contains(result, "jq") {
		t.Error("runtime part missing available helper names")
	}
}

// TestBuildRuntimePartHelperMissingNotDismissed verifies missing core helpers appear when not dismissed.
func TestBuildRuntimePartHelperMissingNotDismissed(t *testing.T) {
	promptsFS, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS:    promptsFS,
		WorkDir:      workDir,
		OS:           platform.Linux,
		HelperSetup:  config.HelperSetup{},
		HelperLookup: fakeHelperLookup([]string{"git"}),
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}
	if !strings.Contains(result, "Optional helpers:\n") {
		t.Error("runtime part missing Optional helpers section")
	}
	if !strings.Contains(result, "rg") {
		t.Error("runtime part missing missing rg in optional section")
	}
}

// TestBuildRuntimePartHelperMissingDismissed verifies optional section renders NULL when dismissed.
func TestBuildRuntimePartHelperMissingDismissed(t *testing.T) {
	promptsFS, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS:    promptsFS,
		WorkDir:      workDir,
		OS:           platform.Linux,
		HelperSetup:  config.HelperSetup{Dismissed: true},
		HelperLookup: fakeHelperLookup([]string{"git"}),
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}
	if !strings.Contains(result, "Optional helpers:\nNULL") {
		t.Error("runtime part should render NULL for dismissed optional helpers")
	}
}

// TestBuildRuntimePartHelperDeclined verifies declined helpers don't appear in optional.
func TestBuildRuntimePartHelperDeclined(t *testing.T) {
	promptsFS, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS:    promptsFS,
		WorkDir:      workDir,
		OS:           platform.Linux,
		HelperSetup:  config.HelperSetup{Declined: []string{"rg", "fd"}},
		HelperLookup: fakeHelperLookup([]string{"git"}),
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}
	optIdx := strings.Index(result, "Optional helpers:")
	if optIdx < 0 {
		t.Fatal("expected Optional helpers section")
	}
	nextSectionIdx := strings.Index(result[optIdx+1:], "\n## ")
	if nextSectionIdx < 0 {
		nextSectionIdx = len(result) - optIdx - 1
	}
	optSection := result[optIdx+1 : optIdx+1+nextSectionIdx]
	if strings.Contains(optSection, "rg") {
		t.Error("optional section should not contain declined helper rg")
	}
	if !strings.Contains(optSection, "jq") {
		t.Error("optional section should contain non-declined missing jq")
	}
}

// TestBuildRuntimePartHelperOrder verifies helper section is after OS prompt, before skills and AGENTS.md.
func TestBuildRuntimePartHelperOrder(t *testing.T) {
	promptsFS, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS:    promptsFS,
		WorkDir:      workDir,
		OS:           platform.Linux,
		HelperSetup:  config.HelperSetup{},
		HelperLookup: fakeHelperLookup([]string{"rg", "fd", "jq", "git", "curl"}),
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}

	universalIdx := strings.Index(result, "Universal System Prompt")
	osIdx := strings.Index(result, "Linux System Prompt")
	helpersIdx := strings.Index(result, "Host Environment Helpers")
	skillsIdx := strings.Index(result, "Available skills:")
	agentsIdx := strings.Index(result, "Project Rules")

	if universalIdx < 0 || osIdx < 0 || helpersIdx < 0 || skillsIdx < 0 {
		t.Fatalf("missing sections: universal=%d os=%d helpers=%d skills=%d",
			universalIdx, osIdx, helpersIdx, skillsIdx)
	}
	if !(osIdx < helpersIdx && helpersIdx < skillsIdx && skillsIdx < agentsIdx) {
		t.Errorf("wrong order: os=%d helpers=%d skills=%d agents=%d (expected: OS < helpers < skills < AGENTS)",
			osIdx, helpersIdx, skillsIdx, agentsIdx)
	}
}

// TestBuildRuntimePartHelperNoHelpers verifies missing helpers render as NULL.
func TestBuildRuntimePartHelperNoHelpers(t *testing.T) {
	promptsFS, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS:    promptsFS,
		WorkDir:      workDir,
		OS:           platform.Linux,
		HelperSetup:  config.HelperSetup{Dismissed: true},
		HelperLookup: fakeHelperLookup(nil),
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}
	if !strings.Contains(result, "Available helpers:\nNULL") {
		t.Error("runtime part should render NULL for missing host helpers")
	}
	if !strings.Contains(result, "Optional helpers:\nNULL") {
		t.Error("runtime part should render NULL for missing optional host helpers")
	}
}
