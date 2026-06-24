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
	"blazeai/internal/memories"
	"blazeai/internal/platform"
	"blazeai/internal/session"
	"blazeai/internal/skills"
)

// setupTestDirs creates temp directories with prompt and skill files for testing.
// Returns promptsFS, builtinSkillsFS, workDir. Sets HOME to temp dir for isolation.
func setupTestDirs(t *testing.T) (promptsFS, builtinSkillsFS fs.FS, workDir string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	root := t.TempDir()
	promptsDir := filepath.Join(root, "prompts")
	builtinSkillsDir := filepath.Join(root, "skills")
	workDir = filepath.Join(root, "work")

	for _, dir := range []string{promptsDir, builtinSkillsDir, workDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("cannot create dir %s: %v", dir, err)
		}
	}

	// Universal prompt with {APP_HOME} variable.
	writePromptFixtures(t, promptsDir)

	// Builtin skill.
	writeFile(t, filepath.Join(builtinSkillsDir, "memory-manager.md"),
		"[DESCRIPTION]\nLoad when the user or the model decides something should be stored in persistent memory banks. Use for creating, updating, compacting, and cleaning memory banks.\n\n[DETAILS]\n# Memory Manager\n\nUse the `shell` tool with an absolute path to create, edit, or remove memory-bank files under {APP_HOME}/memories/. Example: `{APP_HOME}/memories/my-network.md`.\n")

	appHome, err := platform.AppHome()
	if err != nil {
		t.Fatalf("platform.AppHome() error: %v", err)
	}
	customSkillDir := filepath.Join(appHome, "skills", "project_hub")
	writeFile(t, filepath.Join(customSkillDir, "skill.md"),
		"[DESCRIPTION]\nProject Hub skill with local scripts at {SKILL_DIR}/scripts/run.py.\n\n[DETAILS]\nUse local helper at {SKILL_DIR}/scripts/run.py.\n")

	writeFile(t, filepath.Join(appHome, "memories", "my-network.md"),
		"[DESCRIPTION]\nNetwork inventory memory.\n\n[DETAILS]\nIPs, servers, and roles for the network.\n")

	// AGENTS.md in work dir.
	writeFile(t, filepath.Join(workDir, "AGENTS.md"),
		"# Project Rules\n\nUse {APP_HOME} for paths.\n")

	promptsFS = os.DirFS(promptsDir)
	builtinSkillsFS = os.DirFS(builtinSkillsDir)
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
	writeFile(t, filepath.Join(promptsDir, "sysprompt.md"), "# Universal System Prompt\n\nApp home is at {APP_HOME}.\nUnknown var: {UNKNOWN_VAR}.\n\n## Tool Discipline\n- Keep relevant loaded skills active across follow-up turns on the same topic or task.\n- Do not unload a skill immediately after one successful action if the user is likely to continue in the same domain.\n- Unload a skill only when the user clearly changes topic or task, or when the loaded skill would interfere with the next turn.\n\n## Active State Rules\n- Only skills listed under `## Active Skills` are active right now. Do not infer current active skills from older `load_skill` or `unload_skill` tool results in the conversation history. If there is no `## Active Skills` section below, then no skills are currently active.\n- Only memories listed under `## Active Memories` are active right now. Do not infer current active memories from older `load_memory` or `unload_memory` tool results in the conversation history. If there is no `## Active Memories` section below, then no memories are currently active.\n\n{OS_PROMPT}\n\n{HOST_HELPERS_SECTION}\n\n{SKILLS_SECTION}\n\n{MEMORIES_SECTION}\n\n{AGENTS_SECTION}\n")
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

// TestInjectVariablesAll verifies all four variables in one call.
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
	promptsFS, builtinSkillsFS, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS:       promptsFS,
		BuiltinSkillsFS: builtinSkillsFS,
		WorkDir:         workDir,
		OS:              platform.Linux,
	}
	active := skills.NewActiveList()
	result, err := b.BuildRuntimePart(active, memories.NewActiveList())
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
	if !strings.Contains(result, "Keep relevant loaded skills active across follow-up turns on the same topic or task.") {
		t.Error("runtime part missing keep-loaded skills guidance")
	}
	if !strings.Contains(result, "Unload a skill only when the user clearly changes topic or task") {
		t.Error("runtime part missing unload-on-topic-shift guidance")
	}

	// Memory-banks section present.
	if !strings.Contains(result, "Available Memories") {
		t.Error("runtime part missing memories section")
	}
	if !strings.Contains(result, "my-network.md") {
		t.Error("runtime part missing memory file name")
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
	if !strings.Contains(result, "memory-manager.md") {
		t.Error("runtime part missing skill file name")
	}
	if !strings.Contains(result, "Only skills listed under `## Active Skills` are active right now") {
		t.Error("runtime part missing explicit active skill state guidance")
	}
}

// TestBuildRuntimePartMissingUniversal verifies error when universal prompt is missing.
func TestBuildRuntimePartMissingUniversal(t *testing.T) {
	root := t.TempDir()
	b := &Builder{
		PromptsFS:       os.DirFS(filepath.Join(root, "noprompts")),
		BuiltinSkillsFS: os.DirFS(filepath.Join(root, "skills")),
		WorkDir:         root,
		OS:              platform.Linux,
	}
	_, err := b.BuildRuntimePart(skills.NewActiveList(), memories.NewActiveList())
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
		PromptsFS:       os.DirFS(promptsDir),
		BuiltinSkillsFS: os.DirFS(filepath.Join(root, "skills")),
		WorkDir:         root,
		OS:              platform.Linux,
	}
	_, err := b.BuildRuntimePart(skills.NewActiveList(), memories.NewActiveList())
	if err != ErrOSPromptMissing {
		t.Errorf("BuildRuntimePart() err = %v, want ErrOSPromptMissing", err)
	}
}

// TestBuildRuntimePartNoAgentsMD verifies that missing AGENTS.md is omitted silently.
func TestBuildRuntimePartNoAgentsMD(t *testing.T) {
	promptsFS, builtinSkillsFS, _ := setupTestDirs(t)
	// Use a work dir without AGENTS.md.
	emptyWork := t.TempDir()

	b := &Builder{
		PromptsFS:       promptsFS,
		BuiltinSkillsFS: builtinSkillsFS,
		WorkDir:         emptyWork,
		OS:              platform.Linux,
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList(), memories.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}
	if strings.Contains(result, "Project Rules") {
		t.Error("runtime part should not contain AGENTS.md content")
	}
}

// TestBuildRuntimePartActiveSkills verifies that active skills inject [DETAILS].
func TestBuildRuntimePartActiveSkills(t *testing.T) {
	promptsFS, builtinSkillsFS, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS:       promptsFS,
		BuiltinSkillsFS: builtinSkillsFS,
		WorkDir:         workDir,
		OS:              platform.Linux,
	}
	active := skills.NewActiveList()
	active.Load("memory-manager")

	result, err := b.BuildRuntimePart(active, memories.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}
	if !strings.Contains(result, "Active Skills") {
		t.Error("runtime part missing Active Skills section")
	}
	if !strings.Contains(result, "Use the `shell` tool with an absolute path") {
		t.Error("runtime part missing active skill details")
	}
	if !strings.Contains(result, "Do not infer current active skills from older `load_skill` or `unload_skill` tool results") {
		t.Error("runtime part missing history-versus-state guidance")
	}
}

// TestBuildRuntimePartActiveMemories verifies that active memories inject [DETAILS].
func TestBuildRuntimePartActiveMemories(t *testing.T) {
	promptsFS, builtinSkillsFS, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS:       promptsFS,
		BuiltinSkillsFS: builtinSkillsFS,
		WorkDir:         workDir,
		OS:              platform.Linux,
	}
	active := memories.NewActiveList()
	active.Load("my-network")

	result, err := b.BuildRuntimePart(skills.NewActiveList(), active)
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}
	if !strings.Contains(result, "Active Memories") {
		t.Error("runtime part missing Active Memories section")
	}
	if !strings.Contains(result, "IPs, servers, and roles") {
		t.Error("runtime part missing active memory details")
	}
	if !strings.Contains(result, "Do not infer current active memories from older `load_memory` or `unload_memory` tool results") {
		t.Error("runtime part missing memory history-versus-state guidance")
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
		PromptsFS:       os.DirFS(promptsDir),
		BuiltinSkillsFS: os.DirFS(filepath.Join(root, "noskills")),
		WorkDir:         root,
		OS:              platform.Linux,
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList(), memories.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}
	if strings.Contains(result, "Available Skills") {
		t.Error("runtime part should not contain skills section when no skills exist")
	}
}

// TestBuild verifies the full prompt assembly with session messages.
func TestBuild(t *testing.T) {
	promptsFS, builtinSkillsFS, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS:       promptsFS,
		BuiltinSkillsFS: builtinSkillsFS,
		WorkDir:         workDir,
		OS:              platform.Linux,
	}

	sess := &session.Session{
		Messages: []session.Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi there"},
		},
	}

	messages, err := b.Build(sess, skills.NewActiveList(), memories.NewActiveList())
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
	promptsFS, builtinSkillsFS, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS:       promptsFS,
		BuiltinSkillsFS: builtinSkillsFS,
		WorkDir:         workDir,
		OS:              platform.Linux,
	}

	sess := &session.Session{Messages: []session.Message{}}

	messages, err := b.Build(sess, skills.NewActiveList(), memories.NewActiveList())
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

// TestBuildRuntimePartOrder verifies source order: universal → OS → helpers → skills → memories → AGENTS.
func TestBuildRuntimePartOrder(t *testing.T) {
	promptsFS, builtinSkillsFS, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS:       promptsFS,
		BuiltinSkillsFS: builtinSkillsFS,
		WorkDir:         workDir,
		OS:              platform.Linux,
		HelperLookup:    fakeHelperLookup([]string{"rg", "fd", "jq", "git", "curl"}),
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList(), memories.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}

	// Check relative positions.
	universalIdx := strings.Index(result, "Universal System Prompt")
	osIdx := strings.Index(result, "Linux System Prompt")
	helpersIdx := strings.Index(result, "Host Environment Helpers")
	skillsIdx := strings.Index(result, "Available Skills")
	memoryIdx := strings.Index(result, "Available Memories")
	agentsIdx := strings.Index(result, "Project Rules")

	if universalIdx < 0 || osIdx < 0 || helpersIdx < 0 || skillsIdx < 0 || memoryIdx < 0 || agentsIdx < 0 {
		t.Fatalf("missing sections: universal=%d os=%d helpers=%d skills=%d memory=%d agents=%d",
			universalIdx, osIdx, helpersIdx, skillsIdx, memoryIdx, agentsIdx)
	}
	if !(universalIdx < osIdx && osIdx < helpersIdx && helpersIdx < skillsIdx && skillsIdx < memoryIdx && memoryIdx < agentsIdx) {
		t.Errorf("wrong order: universal=%d os=%d helpers=%d skills=%d memory=%d agents=%d",
			universalIdx, osIdx, helpersIdx, skillsIdx, memoryIdx, agentsIdx)
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
	promptsFS, _, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS:       promptsFS,
		BuiltinSkillsFS: os.DirFS(filepath.Join(t.TempDir(), "noskills")),
		WorkDir:         workDir,
		OS:              platform.Linux,
		HelperSetup:     config.HelperSetup{},
		HelperLookup:    fakeHelperLookup([]string{"rg", "jq", "curl"}),
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList(), memories.NewActiveList())
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
	promptsFS, _, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS:       promptsFS,
		BuiltinSkillsFS: os.DirFS(filepath.Join(t.TempDir(), "noskills")),
		WorkDir:         workDir,
		OS:              platform.Linux,
		HelperSetup:     config.HelperSetup{},
		HelperLookup:    fakeHelperLookup([]string{"git"}),
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList(), memories.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}
	if !strings.Contains(result, "## Optional Host Environment Helpers") {
		t.Error("runtime part missing Optional Host Environment Helpers section")
	}
	if !strings.Contains(result, "rg") {
		t.Error("runtime part missing missing rg in optional section")
	}
}

// TestBuildRuntimePartHelperMissingDismissed verifies optional section suppressed when dismissed.
func TestBuildRuntimePartHelperMissingDismissed(t *testing.T) {
	promptsFS, _, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS:       promptsFS,
		BuiltinSkillsFS: os.DirFS(filepath.Join(t.TempDir(), "noskills")),
		WorkDir:         workDir,
		OS:              platform.Linux,
		HelperSetup:     config.HelperSetup{Dismissed: true},
		HelperLookup:    fakeHelperLookup([]string{"git"}),
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList(), memories.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}
	if strings.Contains(result, "Optional Host Environment Helpers") {
		t.Error("runtime part should not contain Optional Host Environment Helpers when dismissed=true")
	}
}

// TestBuildRuntimePartHelperDeclined verifies declined helpers don't appear in optional.
func TestBuildRuntimePartHelperDeclined(t *testing.T) {
	promptsFS, _, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS:       promptsFS,
		BuiltinSkillsFS: os.DirFS(filepath.Join(t.TempDir(), "noskills")),
		WorkDir:         workDir,
		OS:              platform.Linux,
		HelperSetup:     config.HelperSetup{Declined: []string{"rg", "fd"}},
		HelperLookup:    fakeHelperLookup([]string{"git"}),
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList(), memories.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}
	// Only check the optional section for declined helpers; full prompt may contain
	// unrelated system data from the host memory file.
	optIdx := strings.Index(result, "## Optional Host Environment Helpers")
	if optIdx < 0 {
		t.Fatal("expected Optional Host Environment Helpers section")
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

// TestBuildRuntimePartHelperOrder verifies helper section is after OS prompt, before skills, memories, and AGENTS.md.
func TestBuildRuntimePartHelperOrder(t *testing.T) {
	promptsFS, builtinSkillsFS, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS:       promptsFS,
		BuiltinSkillsFS: builtinSkillsFS,
		WorkDir:         workDir,
		OS:              platform.Linux,
		HelperSetup:     config.HelperSetup{},
		HelperLookup:    fakeHelperLookup([]string{"rg", "fd", "jq", "git", "curl"}),
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList(), memories.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}

	universalIdx := strings.Index(result, "Universal System Prompt")
	osIdx := strings.Index(result, "Linux System Prompt")
	helpersIdx := strings.Index(result, "Host Environment Helpers")
	skillsIdx := strings.Index(result, "Available Skills")
	memoryIdx := strings.Index(result, "Available Memories")
	agentsIdx := strings.Index(result, "Project Rules")

	if universalIdx < 0 || osIdx < 0 || helpersIdx < 0 || skillsIdx < 0 || memoryIdx < 0 {
		t.Fatalf("missing sections: universal=%d os=%d helpers=%d skills=%d memory=%d",
			universalIdx, osIdx, helpersIdx, skillsIdx, memoryIdx)
	}
	if !(osIdx < helpersIdx && helpersIdx < skillsIdx && skillsIdx < memoryIdx && memoryIdx < agentsIdx) {
		t.Errorf("wrong order: os=%d helpers=%d skills=%d memory=%d agents=%d (expected: OS < helpers < skills < memory < AGENTS)",
			osIdx, helpersIdx, skillsIdx, memoryIdx, agentsIdx)
	}
}

// TestBuildRuntimePartHelperNoHelpers verifies no helpers section when nothing is available.
func TestBuildRuntimePartHelperNoHelpers(t *testing.T) {
	promptsFS, _, workDir := setupTestDirs(t)

	b := &Builder{
		PromptsFS:       promptsFS,
		BuiltinSkillsFS: os.DirFS(filepath.Join(t.TempDir(), "noskills")),
		WorkDir:         workDir,
		OS:              platform.Linux,
		HelperSetup:     config.HelperSetup{Dismissed: true},
		HelperLookup:    fakeHelperLookup(nil),
	}
	result, err := b.BuildRuntimePart(skills.NewActiveList(), memories.NewActiveList())
	if err != nil {
		t.Fatalf("BuildRuntimePart() error: %v", err)
	}
	if strings.Contains(result, "Host") {
		t.Error("runtime part should not contain any Host Helpers section when nothing to show")
	}
}
