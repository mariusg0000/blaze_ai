// skills.go — skill discovery, parsing, validation, scoping, active skills list, and seeding.
// At startup, embedded builtin skill templates are seeded into app_home/skills/ if missing.
// At runtime, skills are discovered from two scopes only: global (app_home/skills/) and
// project (workdir/.blazeai/skills/). Both use subdirectory layout: <scope>/<name>/skill.md.
// Skills are keyed with scope prefix: global/name, project/name.
// Parses [DESCRIPTION] (required), [BEHAVIOR] (optional), [DATA] (optional).
// At least one of Behavior or Data must be present.
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

// ErrMissingBehaviorOrData is returned when a skill file has neither [BEHAVIOR] nor [DATA].
var ErrMissingBehaviorOrData = errors.New("skill missing [BEHAVIOR] and [DATA]: at least one is required")

// Skill represents a parsed skill file.
//
// WHAT:  Holds the parsed content of a single skill file with optional behavior and data.
// WHY:   The prompt builder needs description (available skills), behavior, and data (active skills).
// Fields: Name — folder name; Description — [DESCRIPTION] content;
//
//	Behavior — [BEHAVIOR] content (optional); Data — [DATA] content (optional);
//	Dir — folder path; Scope — global or project.
type Skill struct {
	Name        string
	Description string
	Behavior    string
	Data        string
	Dir         string
	Scope       Scope
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

// Parse extracts [DESCRIPTION], [BEHAVIOR], and [DATA] sections from skill content.
// [DESCRIPTION] is required. At least one of [BEHAVIOR] or [DATA] must be present.
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

	if behavior == "" && data == "" {
		return nil, ErrMissingBehaviorOrData
	}

	return &Skill{
		Name:        name,
		Description: strings.TrimSpace(desc),
		Behavior:    strings.TrimSpace(behavior),
		Data:        strings.TrimSpace(data),
	}, nil
}

// extractSection finds a required [SECTION] block and returns its content.
// A section starts with [SECTION] on its own line and ends at the next [SECTION] marker or EOF.
func extractSection(content, sectionName string) (string, error) {
	marker := "[" + sectionName + "]"
	idx := strings.Index(content, marker)
	if idx < 0 {
		if sectionName == "DESCRIPTION" {
			return "", ErrMissingDescription
		}
		return "", fmt.Errorf("skill missing [%s] section", sectionName)
	}
	start := idx + len(marker)
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
// already exist. Each embedded .md file becomes app_home/skills/<name>/skill.md.
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
	}
	return nil
}

// DiscoverProject discovers project-scoped skills from workdir/.blazeai/skills/.
// Keys use project/ prefix.
//
// WHAT:  Scans project skill directory.
// WHY:   Project skills are separate from global, keyed with project/ prefix.
// PARAMS: workDir — the current working directory (project root).
// RETURNS: map[string]*Skill — project skills keyed as project/name; error on read failure.
func DiscoverProject(workDir string) (map[string]*Skill, error) {
	projectDir := filepath.Join(workDir, ".blazeai", "skills")
	skills := make(map[string]*Skill)
	if err := discoverFromSubdirs(projectDir, skills, ScopeProject); err != nil {
		return nil, fmt.Errorf("project skills: %w", err)
	}
	return skills, nil
}

// DiscoverAll discovers skills from both runtime scopes and returns a merged map.
// Global skills are read from app_home/skills/ (via platform.AppHome).
// Project skills are read from workdir/.blazeai/skills/.
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
// If name contains / (already scoped like global/foo or project/foo), exact lookup.
// If name has no prefix, it searches both scopes (global/name, project/name):
//   - single match → returns that canonical ID
//   - multiple matches → error with all candidates
//   - no match → error
//
// WHAT:  Resolves a user-provided name to a canonical scoped skill ID.
// WHY:   load_skill accepts unqualified names; ambiguity must be reported, not silently chosen.
// PARAMS: name — the name to resolve (may be short or scoped); skills — all discovered skills.
// RETURNS: string — canonical skill ID; error if not found or ambiguous.
func Resolve(name string, skills map[string]*Skill) (string, error) {
	name = strings.TrimSuffix(name, ".md")

	// Already scoped (contains /): exact match on canonical ID.
	if strings.Contains(name, "/") {
		if _, ok := skills[name]; ok {
			return name, nil
		}
		return "", fmt.Errorf("skill not found: %s", name)
	}

	// Unqualified: check both scopes.
	candidates := []string{"global/" + name, "project/" + name}
	var matches []string
	for _, c := range candidates {
		if _, ok := skills[c]; ok {
			matches = append(matches, c)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("skill not found: %s", name)
	case 1:
		return matches[0], nil
	default:
		sort.Strings(matches)
		return "", fmt.Errorf("ambiguous skill name %q: available: %s", name, strings.Join(matches, ", "))
	}
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
