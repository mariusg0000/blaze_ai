// read_file.go — read_file tool implementation.
// Reads file contents and returns them to the LLM.
// Layer: tool execution. Dependencies: none (file IO only).
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ReadFileArgs are the arguments for the read_file tool.
//
// WHAT:  Parsed arguments from the LLM tool call.
// PARAMS: FilePath — absolute or relative path to the file to read.
type ReadFileArgs struct {
	FilePath string `json:"file_path"`
}

// ReadFileTool reads a file and returns its contents to the LLM.
//
// WHAT:  Reads a file at file_path and returns "ok\n<content>".
// WHY:   The LLM needs to inspect existing files before making edits.
// PARAMS: workDir — function returning the current working directory for relative path resolution.
type ReadFileTool struct {
	workDir func() string
}

// NewReadFileTool creates a ReadFileTool.
//
// PARAMS: workDir — closure returning the current working directory.
// RETURNS: *ReadFileTool — ready to execute.
func NewReadFileTool(workDir func() string) *ReadFileTool {
	return &ReadFileTool{workDir: workDir}
}

// Name returns the tool's unique identifier.
func (t *ReadFileTool) Name() string {
	return "read_file"
}

// FormatArgs formats a concise UI label with the file path.
func (t *ReadFileTool) FormatArgs(args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[ReadFileArgs](args)
	if err != nil {
		return "Reading file"
	}
	if parsed.FilePath == "" {
		return "Reading file"
	}
	return truncateDisplay("Reading: "+parsed.FilePath, 50)
}

// Description returns the human-readable description for the LLM.
func (t *ReadFileTool) Description() string {
	return "file_path → read file contents"
}

// Parameters returns the JSON schema for the tool's parameters.
func (t *ReadFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {
				"type": "string",
				"description": "file_path = absolute or relative path to the file to read"
			}
		},
		"required": ["file_path"]
	}`)
}

// Execute reads the file at file_path and returns its contents.
//
// WHAT:  Reads and returns file contents.
// WHY:   Provides the LLM with current file state before editing.
// HOW:   Resolves relative paths against workDir, reads file with os.ReadFile.
// PARAMS: ctx — turn cancellation context; args — raw JSON with file_path.
// RETURNS: string — "ok\n<content>" or error description.
func (t *ReadFileTool) Execute(ctx context.Context, args json.RawMessage) string {
	if ctx != nil && ctx.Err() != nil {
		return "aborted before execution by user"
	}
	parsed, err := ParseToolCallArgs[ReadFileArgs](args)
	if err != nil {
		return fmt.Sprintf("error: invalid arguments: %v", err)
	}
	if parsed.FilePath == "" {
		return "error: file_path is required"
	}

	path := parsed.FilePath
	if !filepath.IsAbs(path) {
		if t.workDir == nil {
			return "error: read_file requires absolute file_path (workdir not available)"
		}
		wd := t.workDir()
		if wd == "" {
			return "error: read_file requires absolute file_path (workdir not available)"
		}
		path = filepath.Join(wd, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("error: file not found: %s", path)
		}
		return fmt.Sprintf("error: cannot read file %s: %v", path, err)
	}
	if len(data) == 0 {
		return "ok (empty)"
	}
	return "ok\n" + string(data)
}
