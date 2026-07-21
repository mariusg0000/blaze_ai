// skills.go — skill discovery, parsing, validation, and scoping.
// Builtin skills remain in the embedded filesystem; disk skills are discovered from
// project (app_home/projects/<project>/skills/). Both use subdirectory layout:
// <scope>/<name>/skill.md. Skills are keyed with scope prefix: global/name, project/name.
// Parses the required [DESCRIPTION] and [BODY] sections.
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
	ScopeBuiltin Scope = "builtin"
)

// ErrMissingDescription is returned when a skill file lacks a [DESCRIPTION] section.
var ErrMissingDescription = errors.New("skill missing [DESCRIPTION] section")

// ErrMissingBody is returned when a skill file lacks a non-empty [BODY] section.
var ErrMissingBody = errors.New("skill missing [BODY] section")

// CompatDiag describes a legacy disk skill that could not be parsed.
// The diagnostic preserves the file path and original parse error so the
// runtime can report it while keeping valid skills loadable.
type CompatDiag struct {
	Path  string // absolute path to the malformed skill.md
	Name  string // skill folder name
	Scope Scope  // scope under which the skill was found
	Err   error  // original parse error (usually ErrMissingBody or ErrMissingDescription)
}

// isCompatError returns true when the error is a known section-missing error
// from a disk skill that uses legacy section names.
func isCompatError(err error) bool {
	return errors.Is(err, ErrMissingBody) || errors.Is(err, ErrMissingDescription)
}

// Skill represents a parsed skill file.
//
// WHAT:  Holds the parsed content of a single prompt skill.
// HOW:   The prompt builder uses description while load_skill renders the body.
// Fields: Name — folder name; Description — [DESCRIPTION] content;
//
//	Body — [BODY] content;
//	Dir — folder path; Scope — global or project.
type Skill struct {
	Name        string
	Description string
	Body        string
	Dir         string
	Scope       Scope
}

// Parse extracts the required [DESCRIPTION] and [BODY] sections from skill content.
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

	body, err := extractSection(content, "BODY")
	if err != nil || strings.TrimSpace(body) == "" {
		return nil, ErrMissingBody
	}

	return &Skill{
		Name:        name,
		Description: strings.TrimSpace(desc),
		Body:        strings.TrimSpace(body),
	}, nil
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

// DiscoverProject discovers project-scoped skills from app_home/projects/<project>/skills/.
// Keys use project/ prefix.
//
// WHAT:  Scans project skill directory under the app-home project folder.
// WHY:   Project skills are stored alongside sessions under app_home/projects/.
// PARAMS: workDir — the current working directory (project root).
// RETURNS: map[string]*Skill — project skills keyed as project/name; error on read failure.
func DiscoverProject(workDir string, diags *[]CompatDiag) (map[string]*Skill, error) {
	projectDir, err := platform.ProjectDir(workDir)
	if err != nil {
		return nil, err
	}
	skillsDir := filepath.Join(projectDir, "skills")
	sk := make(map[string]*Skill)
	if err := discoverFromSubdirs(skillsDir, sk, ScopeProject, diags); err != nil {
		return nil, fmt.Errorf("project skills: %w", err)
	}
	return sk, nil
}

// DiscoverAll discovers skills from both runtime scopes and returns a merged map.
// Global skills are read from app_home/skills/ (via platform.AppHome).
// Project skills are read from app_home/projects/<project>/skills/ (via platform.ProjectDir).
// Keys use global/ or project/ prefix.
//
// Legacy disk skills with missing [BODY] or [DESCRIPTION] sections produce
// compatibility diagnostics instead of fatal errors. The returned diags slice
// may be non-nil even when err is nil.
//
// WHAT:  Full discovery across global and project.
// WHY:   Prompt building and skill resolution need all available skills.
// PARAMS: workDir — current working directory.
// RETURNS: map[string]*Skill — all skills; []CompatDiag — legacy skill diagnostics; error on fatal discovery failure.
func DiscoverAll(workDir string, builtinFS fs.FS) (map[string]*Skill, []CompatDiag, error) {
	if builtinFS == nil {
		return nil, nil, fmt.Errorf("builtin skills filesystem is nil")
	}
	skills := make(map[string]*Skill)
	entries, err := fs.ReadDir(builtinFS, ".")
	if err != nil {
		return nil, nil, fmt.Errorf("builtin skills: cannot list embedded skills: %w", err)
	}
	builtinNames := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".md")
		data, err := fs.ReadFile(builtinFS, entry.Name())
		if err != nil {
			return nil, nil, fmt.Errorf("builtin skills: cannot read embedded skill %s: %w", entry.Name(), err)
		}
		skill, err := Parse(name, string(data))
		if err != nil {
			return nil, nil, fmt.Errorf("builtin skills: cannot parse embedded skill %s: %w", entry.Name(), err)
		}
		skill.Scope = ScopeBuiltin
		builtinNames[name] = true
		skills["builtin/"+name] = skill
	}
	home, err := platform.AppHome()
	if err != nil {
		return nil, nil, err
	}
	globalDir := filepath.Join(home, "skills")

	var diags []CompatDiag
	diskSkills := make(map[string]*Skill)
	if err := discoverFromSubdirs(globalDir, diskSkills, ScopeGlobal, &diags); err != nil {
		return nil, nil, fmt.Errorf("global skills: %w", err)
	}

	project, err := DiscoverProject(workDir, &diags)
	if err != nil {
		return nil, nil, err
	}
	for k, v := range project {
		diskSkills[k] = v
	}
	for k, v := range diskSkills {
		if builtinNames[v.Name] {
			continue
		}
		skills[k] = v
	}
	return skills, diags, nil
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
	if strings.HasPrefix(name, "builtin/") || strings.HasPrefix(name, "global/") || strings.HasPrefix(name, "project/") {
		if _, ok := skills[name]; ok {
			return name, nil
		}
		return "", fmt.Errorf("skill not found: %s", name)
	}

	if _, ok := skills["builtin/"+name]; ok {
		return "builtin/" + name, nil
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
	var diags []CompatDiag
	skills := make(map[string]*Skill)
	if err := discoverFromSubdirs(dir, skills, ScopeGlobal, &diags); err != nil {
		return nil, err
	}
	// discoverFromSubdirs continues on compat errors in the new flow, so
	// DiscoverGlobalFromDir keeps its original contract: no diags returned.
	return skills, nil
}

// discoverFromSubdirs reads skills from subdirectory layout: <dir>/<name>/skill.md.
// Skills are stored with scope prefix as canonical ID (global/name or project/name).
// Parse errors caused by legacy section names (missing [BODY] or [DESCRIPTION])
// are collected as compatibility diagnostics instead of failing fatally.
func discoverFromSubdirs(root string, skills map[string]*Skill, scope Scope, diags *[]CompatDiag) error {
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
			// Legacy section-format errors are compatibility diagnostics,
			// not fatal failures. Non-compatibility errors still fail fast.
			if isCompatError(err) {
				if diags != nil {
					*diags = append(*diags, CompatDiag{
						Path:  skillFile,
						Name:  name,
						Scope: scope,
						Err:   err,
					})
				}
				continue
			}
			return fmt.Errorf("cannot parse skill %s: %w", skillFile, err)
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
