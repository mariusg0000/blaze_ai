// list_files.go — fd-based file and directory discovery tool.
// Runs fd directly (not via shell) and returns structured output with line limits.
// Returns an explicit install instruction when fd is missing — no fallback to find or shell.
// Layer: tool execution. Dependencies: os/exec only.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const maxListFilesOutputBytes = 100 * 1024

// ListFilesArgs are the typed arguments for the list_files tool.
type ListFilesArgs struct {
	Purpose   string `json:"purpose"`
	Path      string `json:"path,omitempty"`
	Pattern   string `json:"pattern,omitempty"`
	Type      string `json:"type,omitempty"`
	MaxDepth  *int   `json:"max_depth,omitempty"`
	MaxOutput *int   `json:"max_output,omitempty"`
	Timeout   *int   `json:"timeout,omitempty"`
}

// ListFilesTool discovers files and directories using the fd helper.
type ListFilesTool struct {
	workDir func() string
}

// NewListFilesTool creates a tool wrapping fd.
func NewListFilesTool(workDir func() string) *ListFilesTool {
	return &ListFilesTool{workDir: workDir}
}

func (t *ListFilesTool) Name() string { return "list_files" }

func (t *ListFilesTool) Description() string {
	return "Discover files and directories with fd. Fast native discovery without shell overhead. Returns file paths one per line."
}

func (t *ListFilesTool) FormatArgs(args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[ListFilesArgs](args)
	if err != nil {
		return "Listing files"
	}
	if purpose := strings.TrimSpace(parsed.Purpose); purpose != "" {
		return purpose
	}
	return "Listing files"
}

func (t *ListFilesTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"purpose": {
				"type": "string",
				"description": "purpose = exactly 3 user-visible sentences. Sentence 1 must name the target directory or project area and what is being sought. Sentence 2 must describe the scope — directory, pattern, depth, or type filter. Sentence 3 must explain how the listing result advances the task."
			},
			"path": {
				"type": "string",
				"description": "path = starting directory; optional = true; default = work_dir"
			},
			"pattern": {
				"type": "string",
				"description": "pattern = fd pattern (glob, exact substring, or regex); optional = true; default = all files"
			},
			"type": {
				"type": "string",
				"enum": ["file", "directory", "any"],
				"description": "type = file | directory | any; optional = true; default = any"
			},
			"max_depth": {
				"type": "integer",
				"description": "max_depth = maximum directory depth; optional = true; default = unlimited"
			},
			"max_output": {
				"type": "integer",
				"description": "max_output = maximum output lines returned; optional = true; default = 500"
			},
			"timeout": {
				"type": "integer",
				"description": "timeout = seconds; optional = true; default = 30"
			}
		},
		"required": ["purpose"]
	}`)
}

func (t *ListFilesTool) Execute(ctx context.Context, args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[ListFilesArgs](args)
	if err != nil {
		return fmt.Sprintf("error: invalid arguments: %v", err)
	}

	fdPath, err := exec.LookPath("fd")
	if err != nil {
		return "error: fd is not installed. Install it: on Ubuntu/Debian run 'sudo apt install fd-find && ln -sf $(which fdfind) ~/.local/bin/fd'. On macOS: 'brew install fd'. Then retry."
	}

	workDir := ""
	if t.workDir != nil {
		workDir = t.workDir()
	}
	if parsed.Path != "" {
		workDir = parsed.Path
	}

	cmdArgs := []string{"--hidden", "--exclude", ".git"}
	if parsed.MaxDepth != nil && *parsed.MaxDepth >= 0 {
		cmdArgs = append(cmdArgs, "--max-depth", fmt.Sprintf("%d", *parsed.MaxDepth))
	}
	switch parsed.Type {
	case "file":
		cmdArgs = append(cmdArgs, "--type", "f")
	case "directory":
		cmdArgs = append(cmdArgs, "--type", "d")
	case "", "any":
	default:
		return fmt.Sprintf("error: type must be file, directory, or any; got %q", parsed.Type)
	}
	if parsed.Pattern != "" {
		cmdArgs = append(cmdArgs, parsed.Pattern, workDir)
	} else {
		cmdArgs = append(cmdArgs, ".", workDir)
	}

	return executeHelper(ctx, fdPath, cmdArgs, workDir, maxListFilesOutputBytes, parsed.MaxOutput, parsed.Timeout)
}
