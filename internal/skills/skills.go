// skills.go — skill discovery, parsing, validation, scoping, active skills list, and seeding.
// At startup, embedded builtin skill templates are seeded into app_home/skills/ if missing.
// At runtime, skills are discovered from two scopes: global (app_home/skills/) and
// project (app_home/projects/<project>/skills/). Both use subdirectory layout:
// <scope>/<name>/skill.md. Skills are keyed with scope prefix: global/name, project/name.
// Parses [DESCRIPTION] (required), [BEHAVIOR] (optional), [DATA] (optional),
// [SYNTAX] (optional), and [CODE] (optional).
// A skill must provide prompt content ([BEHAVIOR] or [DATA]) or a runnable pair
// ([SYNTAX] and [CODE]).
// Resolution: unqualified names resolve if unique across scopes; ambiguous names error.
// Layer: skill management. Dependencies: internal/platform.
package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"blazeai/internal/platform"
)

// Scope identifies the source of a skill.
type Scope string

const (
	ScopeGlobal  Scope = "global"
	ScopeProject Scope = "project"
)

// ErrMissingDescription is returned when a skill file lacks a [DESCRIPTION] section.
var ErrMissingDescription = errors.New("skill missing [DESCRIPTION] section")

// ErrMissingBehaviorOrData is returned when a skill file has neither prompt content nor a runnable pair.
var ErrMissingBehaviorOrData = errors.New("skill missing [BEHAVIOR] and [DATA], and no runnable [SYNTAX] + [CODE] pair")

// Skill represents a parsed skill file.
//
// WHAT:  Holds the parsed content of a single skill file with prompt and runnable metadata.
// WHY:   The prompt builder needs description, behavior, data, and runnable syntax; the
// HOW:   runtime also needs parsed runnable code for run_skill execution.
// Fields: Name — folder name; Description — [DESCRIPTION] content;
//
//	Behavior — [BEHAVIOR] content (optional); Data — [DATA] content (optional);
//	Syntax — [SYNTAX] content (optional); CodeLang/Code — parsed [CODE] fence content
//	(optional); CodeError — [CODE] parse failure message when present; Dir — folder
//	path; Scope — global or project.
type Skill struct {
	Name        string
	Description string
	Behavior    string
	Data        string
	Syntax      string
	CodeLang    string
	Code        string
	CodeError   string
	Dir         string
	Scope       Scope
}

// HasPromptContent reports whether the skill contributes prompt content when loaded.
func (s *Skill) HasPromptContent() bool {
	return strings.TrimSpace(s.Behavior) != "" || strings.TrimSpace(s.Data) != ""
}

// IsRunnable reports whether the skill is runnable in v1.
func (s *Skill) IsRunnable() bool {
	return strings.TrimSpace(s.Syntax) != "" && strings.TrimSpace(s.Code) != "" && s.CodeLang == "shell"
}

// ActiveList holds the in-memory list of active skill IDs for the current session.
// The list starts empty, is not persisted, and is not deduced from history.
//
// WHAT:  Tracks which skills are loaded in the current session.
// WHY:   load_skill and unload_skill modify this list; prompt builder injects active skill content.
// HOW:   Simple slice with Load/Unload/Has/List methods.
type ActiveList struct {
	names []string
}

// Clear removes all active skills from the list.
func (a *ActiveList) Clear() {
	a.names = a.names[:0]
}

// NewActiveList returns an empty ActiveList for a new session.
func NewActiveList() *ActiveList {
	return &ActiveList{names: []string{}}
}

// Load adds a skill ID to the active list if not already present.
func (a *ActiveList) Load(name string) {
	for _, n := range a.names {
		if n == name {
			return
		}
	}
	a.names = append(a.names, name)
}

// Unload removes a skill ID from the active list if present.
func (a *ActiveList) Unload(name string) {
	for i, n := range a.names {
		if n == name {
			a.names = append(a.names[:i], a.names[i+1:]...)
			return
		}
	}
}

// Has returns true if the given skill ID is in the active list.
func (a *ActiveList) Has(name string) bool {
	for _, n := range a.names {
		if n == name {
			return true
		}
	}
	return false
}

// List returns a copy of the active skill IDs.
func (a *ActiveList) List() []string {
	result := make([]string, len(a.names))
	copy(result, a.names)
	return result
}

// Parse extracts [DESCRIPTION], [BEHAVIOR], [DATA], [SYNTAX], and [CODE] sections from skill content.
// [DESCRIPTION] is required. A skill must provide [BEHAVIOR] or [DATA], or a runnable
// [SYNTAX] + [CODE] pair.
// Section markers must appear at the start of a line (after newline or at position 0).
// References to [SECTION] names inside body text (e.g., in backticks or prose) are ignored.
// Escaped markers like \[BEHAVIOR\] and \[DATA\] remain literal text and do not open sections.
//
// WHAT:  Parses raw Markdown content into a Skill.
// PARAMS: name — the skill name (folder name); content — raw file content.
// RETURNS: *Skill — parsed skill; error if required sections are missing.
func Parse(name, content string) (*Skill, error) {
	desc, err := extractSection(content, "DESCRIPTION")
	if err != nil {
		return nil, err
	}

	behavior, _ := extractOptionalSection(content, "BEHAVIOR")
	data, _ := extractOptionalSection(content, "DATA")
	syntax, _ := extractOptionalSection(content, "SYNTAX")
	codeSection, _ := extractOptionalSection(content, "CODE")
	codeLang, code, codeErr := parseCodeFence(codeSection)

	if strings.TrimSpace(behavior) == "" && strings.TrimSpace(data) == "" && (strings.TrimSpace(syntax) == "" || strings.TrimSpace(code) == "") {
		return nil, ErrMissingBehaviorOrData
	}

	return &Skill{
		Name:        name,
		Description: strings.TrimSpace(desc),
		Behavior:    strings.TrimSpace(behavior),
		Data:        strings.TrimSpace(data),
		Syntax:      compactLines(syntax),
		CodeLang:    codeLang,
		Code:        code,
		CodeError:   codeErr,
	}, nil
}

// parseCodeFence extracts a fenced code block language and body from a [CODE] section.
func parseCodeFence(section string) (string, string, string) {
	section = strings.TrimSpace(section)
	if section == "" {
		return "", "", ""
	}
	if !strings.HasPrefix(section, "```") {
		return "", "", "[CODE] must start with a fenced code block"
	}
	newlineIdx := strings.IndexByte(section, '\n')
	if newlineIdx < 0 {
		return "", "", "[CODE] fence must include a language line and body"
	}
	lang := strings.TrimSpace(section[len("```"):newlineIdx])
	if lang == "" {
		return "", "", "[CODE] fence language is required"
	}
	body := section[newlineIdx+1:]
	closeIdx := strings.LastIndex(body, "\n```")
	if closeIdx < 0 {
		return "", "", "[CODE] fence must end with ```"
	}
	trailing := strings.TrimSpace(body[closeIdx+len("\n```"):])
	if trailing != "" {
		return "", "", "[CODE] must not contain text after the closing fence"
	}
	return lang, strings.TrimSpace(body[:closeIdx]), ""
}

// compactLines collapses a multi-line section into a single trimmed line for prompt-efficient display.
func compactLines(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// extractSection finds a required [SECTION] block and returns its content.
// The marker must appear at the start of a line (after \n or at position 0).
// A section ends at the next [SECTION] marker (also at start of line) or EOF.
// Escaped markers like \[DATA\] are treated as normal content.
func extractSection(content, sectionName string) (string, error) {
	marker := "\n[" + sectionName + "]"
	idx := strings.Index(content, marker)
	if idx < 0 {
		if strings.HasPrefix(content, "["+sectionName+"]") {
			idx = 0
		} else {
			if sectionName == "DESCRIPTION" {
				return "", ErrMissingDescription
			}
			return "", fmt.Errorf("skill missing [%s] section", sectionName)
		}
	}
	if idx > 0 {
		idx++ // skip the leading \n
	}
	start := idx + len("["+sectionName+"]")
	rest := content[start:]
	nextIdx := strings.Index(rest, "\n[")
	if nextIdx < 0 {
		return strings.TrimSpace(rest), nil
	}
	return strings.TrimSpace(rest[:nextIdx]), nil
}

// extractOptionalSection finds an optional [SECTION] block. Returns empty string if missing.
func extractOptionalSection(content, sectionName string) (string, error) {
	result, err := extractSection(content, sectionName)
	if err != nil {
		return "", nil
	}
	return result, nil
}

// SeedBuiltins copies embedded builtin skill templates into app_home/skills/ if they do not
// already exist. Each embedded .md file becomes app_home/skills/<name>/skill.md. If the
// builtin template also has a same-named support subtree (for example <name>/docs/), that
// subtree is copied alongside the skill file during the first seed.
// Existing files are never overwritten — the user can delete a seeded skill to restore the
// original on the next restart.
//
// WHAT:  One-time seeding of embedded skill templates into the global skills directory.
// WHY:   Builtins are templates, not a runtime scope. They become editable global skills
//
//	after seeding. Deletion + restart restores the original.
//
// PARAMS: templatesFS — filesystem with embedded .md skill templates (e.g., read from
//
//	embed.FS); appHomeSkillsDir — path to app_home/skills/.
//
// RETURNS: error if a read or write operation fails.
func SeedBuiltins(templatesFS fs.FS, appHomeSkillsDir string) error {
	entries, err := fs.ReadDir(templatesFS, ".")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		skillName := strings.TrimSuffix(name, ".md")
		skillDir := filepath.Join(appHomeSkillsDir, skillName)
		skillFile := filepath.Join(skillDir, "skill.md")

		if _, err := os.Stat(skillFile); err == nil {
			continue // already exists, user may have customised it
		}

		data, err := fs.ReadFile(templatesFS, name)
		if err != nil {
			return fmt.Errorf("cannot read builtin template %s: %w", name, err)
		}

		if err := os.MkdirAll(skillDir, 0755); err != nil {
			return fmt.Errorf("cannot create skill directory %s: %w", skillDir, err)
		}
		if err := os.WriteFile(skillFile, data, 0644); err != nil {
			return fmt.Errorf("cannot write skill file %s: %w", skillFile, err)
		}
		if err := copyBuiltinSubtree(templatesFS, skillName, skillDir); err != nil {
			return fmt.Errorf("cannot copy builtin subtree for %s: %w", skillName, err)
		}
	}
	return nil
}

// copyBuiltinSubtree copies an optional same-named subtree from the embedded builtin skill
// templates into the seeded skill directory. It is used for auxiliary docs such as
// config-manager/docs/*.md.
func copyBuiltinSubtree(templatesFS fs.FS, sourceDir, targetDir string) error {
	entries, err := fs.ReadDir(templatesFS, sourceDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.ToSlash(filepath.Join(sourceDir, entry.Name()))
		targetPath := filepath.Join(targetDir, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("cannot create builtin subtree directory %s: %w", targetPath, err)
			}
			if err := copyBuiltinSubtree(templatesFS, sourcePath, targetPath); err != nil {
				return err
			}
			continue
		}
		data, err := fs.ReadFile(templatesFS, sourcePath)
		if err != nil {
			return fmt.Errorf("cannot read builtin subtree file %s: %w", sourcePath, err)
		}
		if err := os.WriteFile(targetPath, data, 0644); err != nil {
			return fmt.Errorf("cannot write builtin subtree file %s: %w", targetPath, err)
		}
	}
	return nil
}

// DiscoverProject discovers project-scoped skills from app_home/projects/<project>/skills/.
// Keys use project/ prefix.
//
// WHAT:  Scans project skill directory under the app-home project folder.
// WHY:   Project skills are stored alongside sessions under app_home/projects/.
// PARAMS: workDir — the current working directory (project root).
// RETURNS: map[string]*Skill — project skills keyed as project/name; error on read failure.
func DiscoverProject(workDir string) (map[string]*Skill, error) {
	projectDir, err := platform.ProjectDir(workDir)
	if err != nil {
		return nil, err
	}
	skillsDir := filepath.Join(projectDir, "skills")
	sk := make(map[string]*Skill)
	if err := discoverFromSubdirs(skillsDir, sk, ScopeProject); err != nil {
		return nil, fmt.Errorf("project skills: %w", err)
	}
	return sk, nil
}

// DiscoverAll discovers skills from both runtime scopes and returns a merged map.
// Global skills are read from app_home/skills/ (via platform.AppHome).
// Project skills are read from app_home/projects/<project>/skills/ (via platform.ProjectDir).
// Keys use global/ or project/ prefix.
//
// WHAT:  Full discovery across global and project.
// WHY:   Prompt building and skill resolution need all available skills.
// PARAMS: workDir — current working directory.
// RETURNS: map[string]*Skill — all skills; error on discovery failure.
func DiscoverAll(workDir string) (map[string]*Skill, error) {
	home, err := platform.AppHome()
	if err != nil {
		return nil, err
	}
	globalDir := filepath.Join(home, "skills")

	skills := make(map[string]*Skill)
	if err := discoverFromSubdirs(globalDir, skills, ScopeGlobal); err != nil {
		return nil, fmt.Errorf("global skills: %w", err)
	}

	project, err := DiscoverProject(workDir)
	if err != nil {
		return nil, err
	}
	for k, v := range project {
		skills[k] = v
	}
	return skills, nil
}

// Resolve finds the canonical skill ID for a given name.
// If name is prefixed with "project/", exact lookup on project/name.
// If name is bare (no prefix), it resolves to global/name by default.
// The "global/" prefix is never used for loading — global skills are the default.
//
// WHAT:  Resolves a user-provided name to a canonical scoped skill ID.
// WHY:   load_skill accepts bare names for global skills and project/name for project skills.
// PARAMS: name — the name to resolve (bare or project/name); skills — all discovered skills.
// RETURNS: string — canonical skill ID; error if not found.
func Resolve(name string, skills map[string]*Skill) (string, error) {
	name = strings.TrimSuffix(name, ".md")

	// Project-scoped: project/ prefix.
	if strings.HasPrefix(name, "project/") {
		if _, ok := skills[name]; ok {
			return name, nil
		}
		return "", fmt.Errorf("skill not found: %s", name)
	}

	// Bare name: resolve as global by default.
	id := "global/" + name
	if _, ok := skills[id]; ok {
		return id, nil
	}

	return "", fmt.Errorf("skill not found: %s", name)
}

// SortedNames returns skill IDs from a map sorted alphabetically.
func SortedNames(skills map[string]*Skill) []string {
	names := make([]string, 0, len(skills))
	for name := range skills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DiscoverGlobalFromDir discovers global-scoped skills from a directory with
// subdirectory layout: <dir>/<name>/skill.md. Keys use global/ prefix.
//
// WHAT:  Test-friendly discovery from an explicit global skills directory.
// WHY:   Tests can point to a temp directory without setting HOME.
// PARAMS: dir — path to the skills directory containing skill subdirectories.
// RETURNS: map[string]*Skill — discovered skills keyed as global/name; error on read failure.
func DiscoverGlobalFromDir(dir string) (map[string]*Skill, error) {
	skills := make(map[string]*Skill)
	if err := discoverFromSubdirs(dir, skills, ScopeGlobal); err != nil {
		return nil, err
	}
	return skills, nil
}

// discoverFromSubdirs reads skills from subdirectory layout: <dir>/<name>/skill.md.
// Skills are stored with scope prefix as canonical ID (global/name or project/name).
func discoverFromSubdirs(root string, skills map[string]*Skill, scope Scope) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		skillDir := filepath.Join(root, name)
		skillFile := filepath.Join(skillDir, "skill.md")
		data, err := os.ReadFile(skillFile)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("cannot read skill %s: %w", skillFile, err)
		}
		skill, err := Parse(name, string(data))
		if err != nil {
			continue
		}
		skill.Dir = skillDir
		skill.Scope = scope

		var prefix string
		switch scope {
		case ScopeProject:
			prefix = "project/"
		default:
			prefix = "global/"
		}
		skills[prefix+name] = skill
	}
	return nil
}
