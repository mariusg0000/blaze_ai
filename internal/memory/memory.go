// memory.go — persistent memory file reader.
// Reads app_home/memory/memory.md fresh on every call. If the file is missing,
// returns an empty string silently — memory is optional per spec.
// Layer: memory access. Dependencies: internal/platform (app home path resolution).
package memory

import (
	"fmt"
	"os"
	"path/filepath"

	"blazeai/internal/platform"
)

// Read loads the memory file from app_home/memory/memory.md.
// Returns the file content as a string. If the file does not exist,
// returns an empty string and no error — memory is optional.
//
// WHAT:  Reads the persistent memory file fresh from disk.
// WHY:   Memory is injected into the runtime prompt on every LLM call.
// HOW:   Resolves app home path, reads memory.md, returns content.
// RETURNS: string — file content or empty if missing; error only on read failure (not missing).
func Read() (string, error) {
	home, err := platform.AppHome()
	if err != nil {
		return "", err
	}
	return ReadFrom(filepath.Join(home, "memory", "memory.md"))
}

// ReadFrom reads a memory file from an explicit path.
// If the file does not exist, returns an empty string and no error.
//
// WHAT:  Same as Read but for an explicit file path.
// WHY:   Enables testing with temp directories.
// PARAMS: path — the memory file path to read.
// RETURNS: string — file content or empty if missing; error only on actual read failure.
func ReadFrom(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("cannot read memory file %s: %w", path, err)
	}
	return string(data), nil
}
