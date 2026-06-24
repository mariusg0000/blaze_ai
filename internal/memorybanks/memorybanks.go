// memorybanks.go — custom memory-bank discovery, parsing, validation, and active list.
// Discovers memory-bank files from app_home/memorybanks/, parses [DESCRIPTION] and [DETAILS]
// sections, validates required sections, and maintains an in-memory active memory-bank list per session.
// Layer: memory-bank management. Dependencies: internal/platform (app home path for custom banks).
package memorybanks

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"blazeai/internal/platform"
)

// ErrMissingDescription is returned when a memory-bank file lacks a [DESCRIPTION] section.
var ErrMissingDescription = errors.New("memory-bank missing [DESCRIPTION] section")

// ErrMissingDetails is returned when a memory-bank file lacks a [DETAILS] section.
var ErrMissingDetails = errors.New("memory-bank missing [DETAILS] section")

// MemoryBank represents one parsed memory-bank file.
//
// WHAT:  Holds the parsed content of a single custom memory-bank file.
// WHY:   The prompt builder needs description (available memory-banks) and details (active memory-banks).
// PARAMS: Name — file name without extension; Description — [DESCRIPTION] content; Details — [DETAILS] content.
type MemoryBank struct {
	Name        string
	Description string
	Details     string
}

// ActiveList holds the in-memory list of active memory-bank names for the current session.
// The list starts empty, is not persisted, and is not deduced from history.
//
// WHAT:  Tracks which memory-banks are loaded in the current session.
// WHY:   load_memory_bank and unload_memory_bank modify this list; prompt builder injects details of active memory-banks.
// HOW:   Simple slice with Load/Unload/Has/List methods.
type ActiveList struct {
	names []string
}

// NewActiveList returns an empty ActiveList for a new session.
//
// WHAT:  Creates a fresh active memory-bank list.
// WHY:   The list starts empty at the beginning of every session.
// RETURNS: *ActiveList — empty list ready for Load/Unload operations.
func NewActiveList() *ActiveList {
	return &ActiveList{names: []string{}}
}

// Load adds a memory-bank name to the active list if not already present.
//
// WHAT:  Activates a memory-bank by name.
// WHY:   load_memory_bank tool calls this to make a memory-bank's [DETAILS] available in the prompt.
// PARAMS: name — the memory-bank name to activate.
func (a *ActiveList) Load(name string) {
	for _, n := range a.names {
		if n == name {
			return
		}
	}
	a.names = append(a.names, name)
}

// Unload removes a memory-bank name from the active list if present.
//
// WHAT:  Deactivates a memory-bank by name.
// WHY:   unload_memory_bank tool calls this to remove a memory-bank's [DETAILS] from the prompt.
// PARAMS: name — the memory-bank name to deactivate.
func (a *ActiveList) Unload(name string) {
	for i, n := range a.names {
		if n == name {
			a.names = append(a.names[:i], a.names[i+1:]...)
			return
		}
	}
}

// Has returns true if the given memory-bank name is in the active list.
//
// WHAT:  Checks whether a memory-bank is currently active.
// WHY:   Useful for conditional behavior and debugging.
// PARAMS: name — the memory-bank name to check.
// RETURNS: bool — true if active.
func (a *ActiveList) Has(name string) bool {
	for _, n := range a.names {
		if n == name {
			return true
		}
	}
	return false
}

// List returns a copy of the active memory-bank names.
//
// WHAT:  Returns all active memory-bank names.
// WHY:   The prompt builder needs the list to inject [DETAILS] for each active memory-bank.
// RETURNS: []string — copy of the active names list.
func (a *ActiveList) List() []string {
	result := make([]string, len(a.names))
	copy(result, a.names)
	return result
}

// Parse extracts [DESCRIPTION] and [DETAILS] sections from memory-bank file content.
// Both sections are required. Returns an error if either is missing.
//
// WHAT:  Parses raw Markdown content into a MemoryBank with description and details.
// WHY:   Memory-bank files must have both sections to be valid.
// HOW:   Finds [DESCRIPTION] and [DETAILS] markers, extracts content between them.
// PARAMS: name — the memory-bank name (file name without extension); content — raw file content.
// RETURNS: *MemoryBank — parsed memory-bank; error if a required section is missing.
func Parse(name, content string) (*MemoryBank, error) {
	desc, err := extractSection(content, "DESCRIPTION")
	if err != nil {
		return nil, err
	}
	details, err := extractSection(content, "DETAILS")
	if err != nil {
		return nil, err
	}
	return &MemoryBank{
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
// WHY:   Memory-banks use bracketed section headers to delimit description and details.
// PARAMS: content — raw memory-bank text; sectionName — the section to extract (without brackets).
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

// Discover returns all custom memory-banks from app_home/memorybanks/.
//
// WHAT:  Scans the custom memory-banks directory, parses valid files, and returns results.
// WHY:   The prompt builder needs all available memory-banks with no builtin source.
// HOW:   Reads .md files from app_home/memorybanks, parses each, and stores them by name.
// RETURNS: map[string]*MemoryBank — all discovered memory-banks keyed by name; error on read failure.
func Discover() (map[string]*MemoryBank, error) {
	home, err := platform.AppHome()
	if err != nil {
		return nil, err
	}
	return DiscoverFromDir(filepath.Join(home, "memorybanks"))
}

// DiscoverFromDir is a test-friendly variant that accepts an explicit directory.
//
// WHAT:  Same as Discover but with an explicit directory path.
// WHY:   Enables testing with temp directories without depending on app home.
// PARAMS: dir — custom memory-banks directory.
// RETURNS: map[string]*MemoryBank — all discovered memory-banks keyed by name; error on read failure.
func DiscoverFromDir(dir string) (map[string]*MemoryBank, error) {
	memoryBanks := make(map[string]*MemoryBank)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return memoryBanks, nil
		}
		return nil, fmt.Errorf("cannot read memory-banks dir %s: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		bankName := strings.TrimSuffix(name, ".md")
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("cannot read memory-bank %s: %w", path, err)
		}
		bank, err := Parse(bankName, string(data))
		if err != nil {
			continue
		}
		memoryBanks[bankName] = bank
	}
	return memoryBanks, nil
}

// SortedNames returns memory-bank names from a map sorted alphabetically.
//
// WHAT:  Returns a sorted list of memory-bank names.
// WHY:   Deterministic ordering for prompt building and display.
// PARAMS: memoryBanks — the memory-bank map to extract names from.
// RETURNS: []string — memory-bank names sorted alphabetically.
func SortedNames(memoryBanks map[string]*MemoryBank) []string {
	names := make([]string, 0, len(memoryBanks))
	for name := range memoryBanks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
