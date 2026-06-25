// skills.go — skill discovery, parsing, validation, scoping, and active skills list.
// Discovers skill files from builtin (embedded skills/), global (app_home/skills/),
// and project (workdir/.blazeai/skills/) locations. Parses [DESCRIPTION] (required),
// [BEHAVIOR] (optional), and [DATA] (optional) sections. At least one of Behavior
// or Data must be present. Project-scoped skills use project/ prefix in their IDs.
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
// Fields: Name — file/dir name without extension; Description — [DESCRIPTION] content;
//
//	Behavior — [BEHAVIOR] content (optional); Data — [DATA] content (optional);
//	Dir — folder path for custom skills (empty for builtin); Scope — global or project.
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
// PARAMS: name — the skill name (file name without extension); content — raw file content.
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

// DiscoverFromFS discovers skills from builtin (embedded FS) and global (app_home/skills/).
// Builtin skills are read from builtinFS; global custom skills from disk override builtin by name.
// Project skills are NOT loaded here — use DiscoverProject for workdir-scoped skills.
//
// WHAT:  Scans builtin and global skill sources.
// WHY:   The prompt builder needs all available non-project skills.
// PARAMS: builtinFS — filesystem containing builtin skill .md files.
// RETURNS: map[string]*Skill — discovered skills keyed by ID; error on read failure.
func DiscoverFromFS(builtinFS fs.FS) (map[string]*Skill, error) {
	skills := make(map[string]*Skill)

	if err := discoverFromFS(builtinFS, skills); err != nil {
		return nil, fmt.Errorf("builtin skills: %w", err)
	}

	home, err := platform.AppHome()
	if err != nil {
		return nil, err
	}
	globalDir := filepath.Join(home, "skills")
	if err := discoverGlobalFromDir(globalDir, skills); err != nil {
		return nil, fmt.Errorf("global skills: %w", err)
	}

	return skills, nil
}

// DiscoverProject discovers project-scoped skills from workdir/.blazeai/skills/.
// Keys use project/ prefix to avoid collision with global skills.
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

// DiscoverAll discovers skills from all scopes and returns a merged map.
// Project skills have project/ prefix keys; global/builtin have bare name keys.
//
// WHAT:  Full discovery across builtin, global, and project.
// WHY:   Prompt building and skill resolution need all available skills.
// PARAMS: builtinFS — builtin skill filesystem; workDir — current working directory.
// RETURNS: map[string]*Skill — all skills; error on discovery failure.
func DiscoverAll(builtinFS fs.FS, workDir string) (map[string]*Skill, error) {
	all, err := DiscoverFromFS(builtinFS)
	if err != nil {
		return nil, err
	}
	project, err := DiscoverProject(workDir)
	if err != nil {
		return nil, err
	}
	for k, v := range project {
		all[k] = v
	}
	return all, nil
}

// Resolve finds the canonical skill ID for a given name.
// If name contains / (already scoped), exact lookup is performed.
// If name has no / prefix, it searches all skills:
//   - single match → returns that ID
//   - multiple matches → error with all candidates
//   - no match → error
//
// WHAT:  Resolves a user-provided name to a canonical skill ID.
// WHY:   load_skill accepts unqualified names; ambiguity must be reported, not silently chosen.
// PARAMS: name — the name to resolve (may be short or scoped); skills — all discovered skills.
// RETURNS: string — canonical skill ID; error if not found or ambiguous.
func Resolve(name string, skills map[string]*Skill) (string, error) {
	name = strings.TrimSuffix(name, ".md")

	// Already scoped: exact match.
	if strings.Contains(name, "/") {
		if _, ok := skills[name]; ok {
			return name, nil
		}
		return "", fmt.Errorf("skill not found: %s", name)
	}

	// Unqualified: find all matches.
	var matches []string
	for key := range skills {
		baseName := key
		// Strip project/ prefix for matching.
		if before, after, ok := strings.Cut(key, "/"); ok {
			if before == "project" && after == name {
				matches = append(matches, key)
			}
			continue
		}
		if baseName == name {
			matches = append(matches, key)
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

// discoverFromFS reads .md files from an fs.FS and adds valid skills to the map.
func discoverFromFS(fsys fs.FS, skills map[string]*Skill) error {
	entries, err := fs.ReadDir(fsys, ".")
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
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("cannot read skill %s: %w", name, err)
		}
		skill, err := Parse(skillName, string(data))
		if err != nil {
			continue
		}
		skills[skillName] = skill
	}
	return nil
}

// discoverGlobalFromDir reads global custom skills from app_home/skills/<name>/skill.md.
// Custom skills override builtin skills by name (collision: custom wins).
func discoverGlobalFromDir(dir string, skills map[string]*Skill) error {
	return discoverFromSubdirs(dir, skills, ScopeGlobal)
}

// discoverFromSubdirs reads skills from subdirectory layout: <dir>/<name>/skill.md.
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

		var key string
		if scope == ScopeProject {
			key = "project/" + name
		} else {
			key = name
		}
		skills[key] = skill
	}
	return nil
}

// DiscoverFromDirs is a test-friendly variant accepting explicit directories.
// Builtin skills are flat .md files; custom skills are folder/<name>/skill.md layout.
// Custom overrides builtin by name.
func DiscoverFromDirs(builtinDir, customDir string) (map[string]*Skill, error) {
	skills := make(map[string]*Skill)

	if err := discoverBuiltinFromDir(builtinDir, skills); err != nil {
		return nil, fmt.Errorf("builtin skills: %w", err)
	}
	if err := discoverFromSubdirs(customDir, skills, ScopeGlobal); err != nil {
		return nil, fmt.Errorf("custom skills: %w", err)
	}

	return skills, nil
}

// discoverBuiltinFromDir reads flat .md files from a directory (like embedded builtin layout).
func discoverBuiltinFromDir(dir string, skills map[string]*Skill) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
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
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("cannot read skill %s: %w", path, err)
		}
		skill, err := Parse(skillName, string(data))
		if err != nil {
			continue
		}
		skills[skillName] = skill
	}
	return nil
}
