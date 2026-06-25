// memories.go — custom memory discovery, parsing, validation, and active list.
// Discovers memory files from app_home/memories/, parses [DESCRIPTION] and [DETAILS]
// sections, validates required sections, and maintains an in-memory active memory list per session.
// Layer: memory management. Dependencies: internal/platform (app home path for custom memories).
package memories

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"blazeai/internal/platform"
)

// ErrMissingDescription is returned when a memory file lacks a [DESCRIPTION] section.
var ErrMissingDescription = errors.New("memory missing [DESCRIPTION] section")

// ErrMissingDetails is returned when a memory file lacks a [DETAILS] section.
var ErrMissingDetails = errors.New("memory missing [DETAILS] section")

// Memory represents one parsed memory file.
//
// WHAT:  Holds the parsed content of a single custom memory file.
// WHY:   The prompt builder needs description (available memories) and details (active memories).
// PARAMS: Name — file name without extension; Description — [DESCRIPTION] content; Details — [DETAILS] content.
type Memory struct {
	Name        string
	Description string
	Details     string
}

// ActiveList holds the in-memory list of active memory names for the current session.
// The list starts empty, is not persisted, and is not deduced from history.
//
// WHAT:  Tracks which memories are loaded in the current session.
// WHY:   load_memory and unload_memory modify this list; prompt builder injects details of active memories.
// HOW:   Simple slice with Load/Unload/Has/List methods.
type ActiveList struct {
	names []string
}

// Clear removes all active memories from the list.
//
// WHAT:  Resets the active memory list to empty.
// WHY:   /clear and similar reset flows need to remove all loaded memories at once.
func (a *ActiveList) Clear() {
	a.names = a.names[:0]
}

// NewActiveList returns an empty ActiveList for a new session.
//
// WHAT:  Creates a fresh active memory list.
// WHY:   The list starts empty at the beginning of every session.
// RETURNS: *ActiveList — empty list ready for Load/Unload operations.
func NewActiveList() *ActiveList {
	return &ActiveList{names: []string{}}
}

// Load adds a memory name to the active list if not already present.
//
// WHAT:  Activates a memory by name.
// WHY:   load_memory tool calls this to make a memory's [DETAILS] available in the prompt.
// PARAMS: name — the memory name to activate.
func (a *ActiveList) Load(name string) {
	for _, n := range a.names {
		if n == name {
			return
		}
	}
	a.names = append(a.names, name)
}

// Unload removes a memory name from the active list if present.
//
// WHAT:  Deactivates a memory by name.
// WHY:   unload_memory tool calls this to remove a memory's [DETAILS] from the prompt.
// PARAMS: name — the memory name to deactivate.
func (a *ActiveList) Unload(name string) {
	for i, n := range a.names {
		if n == name {
			a.names = append(a.names[:i], a.names[i+1:]...)
			return
		}
	}
}

// Has returns true if the given memory name is in the active list.
//
// WHAT:  Checks whether a memory is currently active.
// WHY:   Useful for conditional behavior and debugging.
// PARAMS: name — the memory name to check.
// RETURNS: bool — true if active.
func (a *ActiveList) Has(name string) bool {
	for _, n := range a.names {
		if n == name {
			return true
		}
	}
	return false
}

// List returns a copy of the active memory names.
//
// WHAT:  Returns all active memory names.
// WHY:   The prompt builder needs the list to inject [DETAILS] for each active memory.
// RETURNS: []string — copy of the active names list.
func (a *ActiveList) List() []string {
	result := make([]string, len(a.names))
	copy(result, a.names)
	return result
}

// Parse extracts [DESCRIPTION] and [DETAILS] sections from memory file content.
// Both sections are required. Returns an error if either is missing.
//
// WHAT:  Parses raw Markdown content into a Memory with description and details.
// WHY:   Memory files must have both sections to be valid.
// HOW:   Finds [DESCRIPTION] and [DETAILS] markers, extracts content between them.
// PARAMS: name — the memory name (file name without extension); content — raw file content.
// RETURNS: *Memory — parsed memory; error if a required section is missing.
func Parse(name, content string) (*Memory, error) {
	desc, err := extractSection(content, "DESCRIPTION")
	if err != nil {
		return nil, err
	}
	details, err := extractSection(content, "DETAILS")
	if err != nil {
		return nil, err
	}
	return &Memory{
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
// WHY:   Memories use bracketed section headers to delimit description and details.
// PARAMS: content — raw memory text; sectionName — the section to extract (without brackets).
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

// Discover returns all custom memories from app_home/memories/.
//
// WHAT:  Scans the custom memories directory, parses valid files, and returns results.
// WHY:   The prompt builder needs all available memories with no builtin source.
// HOW:   Reads .md files from app_home/memories, parses each, and stores them by name.
// RETURNS: map[string]*Memory — all discovered memories keyed by name; error on read failure.
func Discover() (map[string]*Memory, error) {
	home, err := platform.AppHome()
	if err != nil {
		return nil, err
	}
	return DiscoverFromDir(filepath.Join(home, "memories"))
}

// DiscoverFromDir is a test-friendly variant that accepts an explicit directory.
//
// WHAT:  Same as Discover but with an explicit directory path.
// WHY:   Enables testing with temp directories without depending on app home.
// PARAMS: dir — custom memories directory.
// RETURNS: map[string]*Memory — all discovered memories keyed by name; error on read failure.
func DiscoverFromDir(dir string) (map[string]*Memory, error) {
	discoveredMemories := make(map[string]*Memory)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return discoveredMemories, nil
		}
		return nil, fmt.Errorf("cannot read memories dir %s: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		if strings.EqualFold(name, "README.md") {
			continue
		}
		memoryName := strings.TrimSuffix(name, ".md")
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("cannot read memory %s: %w", path, err)
		}
		memory, err := Parse(memoryName, string(data))
		if err != nil {
			continue
		}
		discoveredMemories[memoryName] = memory
	}
	return discoveredMemories, nil
}

// SortedNames returns memory names from a map sorted alphabetically.
//
// WHAT:  Returns a sorted list of memory names.
// WHY:   Deterministic ordering for prompt building and display.
// PARAMS: memories — the memory map to extract names from.
// RETURNS: []string — memory names sorted alphabetically.
func SortedNames(memories map[string]*Memory) []string {
	names := make([]string, 0, len(memories))
	for name := range memories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
