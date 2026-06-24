// skills.go — skill discovery, parsing, validation, and active skills list.
// Discovers skill files from builtin (project skills/) and custom (app_home/skills/) locations,
// parses [DESCRIPTION] and [DETAILS] sections, validates required sections, resolves collisions
// (custom wins over builtin), and maintains an in-memory active skills list per session.
// Layer: skill management. Dependencies: internal/platform (app home path for custom skills).
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

// ErrMissingDescription is returned when a skill file lacks a [DESCRIPTION] section.
var ErrMissingDescription = errors.New("skill missing [DESCRIPTION] section")

// ErrMissingDetails is returned when a skill file lacks a [DETAILS] section.
var ErrMissingDetails = errors.New("skill missing [DETAILS] section")

// Skill represents a parsed skill file with its name, description, and details.
//
// WHAT:  Holds the parsed content of a single skill file.
// WHY:   The prompt builder needs description (available skills) and details (active skills).
// PARAMS: Name — file name without extension; Description — [DESCRIPTION] content; Details — [DETAILS] content.
type Skill struct {
	Name        string
	Description string
	Details     string
	Dir         string
}

// ActiveList holds the in-memory list of active skill names for the current session.
// The list starts empty, is not persisted, and is not deduced from history.
//
// WHAT:  Tracks which skills are loaded in the current session.
// WHY:   load_skill and unload_skill modify this list; prompt builder injects details of active skills.
// HOW:   Simple slice with Load/Unload/Has/List methods.
type ActiveList struct {
	names []string
}

// Clear removes all active skills from the list.
//
// WHAT:  Resets the active skill list to empty.
// WHY:   /clear and similar reset flows need to remove all loaded skills at once.
func (a *ActiveList) Clear() {
	a.names = a.names[:0]
}

// NewActiveList returns an empty ActiveList for a new session.
//
// WHAT:  Creates a fresh active skills list.
// WHY:   The list starts empty at the beginning of every session per spec.
// RETURNS: *ActiveList — empty list ready for Load/Unload operations.
func NewActiveList() *ActiveList {
	return &ActiveList{names: []string{}}
}

// Load adds a skill name to the active list if not already present.
//
// WHAT:  Activates a skill by name.
// WHY:   load_skill tool calls this to make a skill's [DETAILS] available in the prompt.
// PARAMS: name — the skill name to activate.
func (a *ActiveList) Load(name string) {
	for _, n := range a.names {
		if n == name {
			return
		}
	}
	a.names = append(a.names, name)
}

// Unload removes a skill name from the active list if present.
//
// WHAT:  Deactivates a skill by name.
// WHY:   unload_skill tool calls this to remove a skill's [DETAILS] from the prompt.
// PARAMS: name — the skill name to deactivate.
func (a *ActiveList) Unload(name string) {
	for i, n := range a.names {
		if n == name {
			a.names = append(a.names[:i], a.names[i+1:]...)
			return
		}
	}
}

// Has returns true if the given skill name is in the active list.
//
// WHAT:  Checks whether a skill is currently active.
// WHY:   Useful for conditional behavior and debugging.
// PARAMS: name — the skill name to check.
// RETURNS: bool — true if active.
func (a *ActiveList) Has(name string) bool {
	for _, n := range a.names {
		if n == name {
			return true
		}
	}
	return false
}

// List returns a copy of the active skill names.
//
// WHAT:  Returns all active skill names.
// WHY:   The prompt builder needs the list to inject [DETAILS] for each active skill.
// RETURNS: []string — copy of the active names list.
func (a *ActiveList) List() []string {
	result := make([]string, len(a.names))
	copy(result, a.names)
	return result
}

// Parse extracts [DESCRIPTION] and [DETAILS] sections from skill file content.
// Both sections are required. Returns an error if either is missing.
//
// WHAT:  Parses raw Markdown content into a Skill with description and details.
// WHY:   Skill files must have both sections to be valid per spec.
// HOW:   Finds [DESCRIPTION] and [DETAILS] markers, extracts content between them.
// PARAMS: name — the skill name (file name without extension); content — raw file content.
// RETURNS: *Skill — parsed skill; error if a required section is missing.
func Parse(name, content string) (*Skill, error) {
	desc, err := extractSection(content, "DESCRIPTION")
	if err != nil {
		return nil, err
	}
	details, err := extractSection(content, "DETAILS")
	if err != nil {
		return nil, err
	}
	return &Skill{
		Name:        name,
		Description: strings.TrimSpace(desc),
		Details:     strings.TrimSpace(details),
	}, nil
}

// extractSection finds a [SECTION] block and returns its content.
// A section starts with [SECTION] on its own line and ends at the next [SECTION]
// marker or end of content.
//
// WHAT:  Extracts the text between [sectionName] and the next section marker.
// WHY:   Skills use bracketed section headers to delimit description and details.
// PARAMS: content — raw skill text; sectionName — the section to extract (without brackets).
// RETURNS: string — section content; error if the section marker is not found.
func extractSection(content, sectionName string) (string, error) {
	marker := "[" + sectionName + "]"
	idx := strings.Index(content, marker)
	if idx < 0 {
		if sectionName == "DESCRIPTION" {
			return "", ErrMissingDescription
		}
		return "", ErrMissingDetails
	}
	start := idx + len(marker)
	rest := content[start:]
	nextIdx := strings.Index(rest, "\n[")
	if nextIdx < 0 {
		return strings.TrimSpace(rest), nil
	}
	return strings.TrimSpace(rest[:nextIdx]), nil
}

// Discover finds and parses all skill files from builtin and custom locations.
// Builtin skills are read from the builtinDir; custom skills from app_home/skills/.
// If a custom skill has the same name as a builtin, the custom skill wins.
//
// WHAT:  Scans both skill directories, parses valid files, and returns merged results.
// WHY:   The prompt builder needs all available skills with collision resolution.
// HOW:   Reads builtin first, then custom; custom entries override builtin by name.
// PARAMS: builtinDir — path to the project's builtin skills directory.
// RETURNS: map[string]*Skill — all discovered skills keyed by name; error on read failure.
func Discover(builtinDir string) (map[string]*Skill, error) {
	skills := make(map[string]*Skill)

	if err := discoverFromDir(builtinDir, skills); err != nil {
		return nil, fmt.Errorf("builtin skills: %w", err)
	}

	home, err := platform.AppHome()
	if err != nil {
		return nil, err
	}
	customDir := filepath.Join(home, "skills")
	if err := discoverCustomFromDir(customDir, skills); err != nil {
		return nil, fmt.Errorf("custom skills: %w", err)
	}

	return skills, nil
}

// discoverFromDir reads builtin .md skill files from one directory and adds valid skills to the map.
// Invalid files (missing sections) are skipped silently — only errors from directory
// listing or file reading are returned.
//
// WHAT:  Scans one directory for skill files and adds them to the skills map.
// WHY:   Both builtin and custom directories are scanned the same way.
// HOW:   Lists .md files, reads each, parses, adds to map. Skips invalid files.
// PARAMS: dir — directory to scan; skills — map to populate (existing entries are overridden).
func discoverFromDir(dir string, skills map[string]*Skill) error {
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

// discoverCustomFromDir reads custom skills from {APP_HOME}/skills/<name>/skill.md folders.
// Invalid skill folders are skipped silently — only directory listing or file read errors return.
//
// WHAT:  Scans custom skill directories and reads each folder's skill.md file.
// WHY:   Custom skills may carry scripts, data, or other resources alongside the main skill file.
// HOW:   Lists subdirectories, reads skill.md from each, parses it, and records the folder path.
// PARAMS: dir — custom skills root; skills — map to populate (existing entries are overridden).
func discoverCustomFromDir(dir string, skills map[string]*Skill) error {
	entries, err := os.ReadDir(dir)
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
		skillName := entry.Name()
		skillDir := filepath.Join(dir, skillName)
		path := filepath.Join(skillDir, "skill.md")
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("cannot read skill %s: %w", path, err)
		}
		skill, err := Parse(skillName, string(data))
		if err != nil {
			continue
		}
		skill.Dir = skillDir
		skills[skillName] = skill
	}
	return nil
}

// SortedNames returns skill names from a map sorted alphabetically.
//
// WHAT:  Returns a sorted list of skill names.
// WHY:   Deterministic ordering for prompt building and display.
// PARAMS: skills — the skills map to extract names from.
// RETURNS: []string — skill names sorted alphabetically.
func SortedNames(skills map[string]*Skill) []string {
	names := make([]string, 0, len(skills))
	for name := range skills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DiscoverFromDirs is a test-friendly variant of Discover that accepts explicit directories.
// Custom skills override builtin skills by name.
//
// WHAT:  Same as Discover but with explicit directory paths.
// WHY:   Enables testing with temp directories without depending on app home.
// PARAMS: builtinDir — builtin skills path; customDir — custom skills path.
// RETURNS: map[string]*Skill — all discovered skills keyed by name; error on read failure.
func DiscoverFromDirs(builtinDir, customDir string) (map[string]*Skill, error) {
	skills := make(map[string]*Skill)

	if err := discoverFromDir(builtinDir, skills); err != nil {
		return nil, fmt.Errorf("builtin skills: %w", err)
	}
	if err := discoverCustomFromDir(customDir, skills); err != nil {
		return nil, fmt.Errorf("custom skills: %w", err)
	}

	return skills, nil
}

// DiscoverFromFS discovers builtin skills from an fs.FS (e.g. embedded assets)
// and custom skills from disk (app_home/skills/). Custom skills override builtin by name.
//
// WHAT:  Scans builtin skills from an FS and custom skills from disk.
// WHY:   Enables embed.FS usage for builtin skills.
// HOW:   Reads from builtinFS first, then app_home/skills via discoverFromDir.
// PARAMS: builtinFS — filesystem containing builtin skill .md files.
// RETURNS: map[string]*Skill — all discovered skills keyed by name; error on read failure.
func DiscoverFromFS(builtinFS fs.FS) (map[string]*Skill, error) {
	skills := make(map[string]*Skill)

	if err := discoverFromFS(builtinFS, skills); err != nil {
		return nil, fmt.Errorf("builtin skills: %w", err)
	}

	home, err := platform.AppHome()
	if err != nil {
		return nil, err
	}
	customDir := filepath.Join(home, "skills")
	if err := discoverCustomFromDir(customDir, skills); err != nil {
		return nil, fmt.Errorf("custom skills: %w", err)
	}

	return skills, nil
}

// discoverFromFS reads all .md files from an fs.FS and adds valid skills to the map.
//
// WHAT:  Scans an fs.FS for skill files and adds them to the skills map.
// WHY:   Works with embedded filesystems (embed.FS), os.DirFS, or any fs.FS.
// HOW:   Lists .md files, reads each, parses, adds to map. Skips invalid files.
// PARAMS: fsys — the filesystem to scan; skills — map to populate.
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
