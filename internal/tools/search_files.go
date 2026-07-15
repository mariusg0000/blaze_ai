// search_files.go — rg-based read-only text search tool.
// Runs rg directly (not via shell) and returns bounded matching output.
// Layer: tool execution. Dependencies: os/exec and helper_exec.go.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const maxSearchFilesOutputBytes = 100 * 1024

// SearchFilesArgs are the typed arguments for the search_files tool.
type SearchFilesArgs struct {
	Purpose       string `json:"purpose"`
	Pattern       string `json:"pattern"`
	Path          string `json:"path,omitempty"`
	Glob          string `json:"glob,omitempty"`
	CaseSensitive *bool  `json:"case_sensitive,omitempty"`
	MaxOutput     *int   `json:"max_output,omitempty"`
	Timeout       *int   `json:"timeout,omitempty"`
}

// SearchFilesTool searches text with the rg helper.
type SearchFilesTool struct {
	workDir func() string
}

// NewSearchFilesTool creates a tool wrapping rg.
func NewSearchFilesTool(workDir func() string) *SearchFilesTool {
	return &SearchFilesTool{workDir: workDir}
}

// Name returns the tool's unique identifier.
func (t *SearchFilesTool) Name() string { return "search_files" }

// Description returns the human-readable description for the LLM.
func (t *SearchFilesTool) Description() string {
	return "Search text or regular expressions in files with rg. Read-only and does not use a shell."
}

// FormatArgs formats the purpose or pattern for activity output.
func (t *SearchFilesTool) FormatArgs(args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[SearchFilesArgs](args)
	if err != nil {
		return "Searching files"
	}
	if purpose := strings.TrimSpace(parsed.Purpose); purpose != "" {
		return purpose
	}
	if parsed.Pattern != "" {
		return truncateDisplay("Searching: "+parsed.Pattern, 100)
	}
	return "Searching files"
}

// Parameters returns the JSON schema for search_files arguments.
func (t *SearchFilesTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
        "type": "object",
        "properties": {
            "purpose": {"type": "string", "description": "Exactly 3 user-visible sentences explaining the target, search scope, and how the result advances the task."},
            "pattern": {"type": "string", "description": "Text or regular expression to search for."},
            "path": {"type": "string", "description": "Starting directory or file; defaults to work_dir."},
            "glob": {"type": "string", "description": "Optional rg glob restricting searched files."},
            "case_sensitive": {"type": "boolean", "description": "Whether matching is case-sensitive; defaults to true."},
            "max_output": {"type": "integer", "description": "Maximum result lines; defaults to 500."},
            "timeout": {"type": "integer", "description": "Timeout in seconds; defaults to 60."}
        },
        "required": ["purpose", "pattern"]
    }`)
}

// Execute runs rg directly and returns matching lines and exit status.
func (t *SearchFilesTool) Execute(ctx context.Context, args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[SearchFilesArgs](args)
	if err != nil {
		return fmt.Sprintf("error: invalid arguments: %v", err)
	}
	if strings.TrimSpace(parsed.Pattern) == "" {
		return "error: pattern is required"
	}
	rgPath, err := exec.LookPath("rg")
	if err != nil {
		return "error: rg is not installed. Install it now, then retry: on Ubuntu/Debian run 'sudo apt install ripgrep'; on macOS run 'brew install ripgrep'."
	}
	workDir := ""
	if t.workDir != nil {
		workDir = t.workDir()
	}
	path := parsed.Path
	if path == "" {
		path = "."
	}
	cmdArgs := []string{"--line-number", "--color=never", "--hidden", "--glob", "!.git"}
	if parsed.CaseSensitive != nil && !*parsed.CaseSensitive {
		cmdArgs = append(cmdArgs, "--ignore-case")
	}
	if parsed.Glob != "" {
		cmdArgs = append(cmdArgs, "--glob", parsed.Glob)
	}
	cmdArgs = append(cmdArgs, "--", parsed.Pattern, path)
	return executeHelper(ctx, rgPath, cmdArgs, workDir, maxSearchFilesOutputBytes, parsed.MaxOutput, parsed.Timeout)
}
