// write_file.go — write_file tool implementation.
// Writes content to a file, creating parent directories when needed.
// Layer: tool execution. Dependencies: none (file IO only).
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileArgs are the arguments for the write_file tool.
//
// WHAT:  Parsed arguments from the LLM tool call.
// PARAMS: FilePath — absolute or relative target path; Content — the text to write.
type WriteFileArgs struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

// WriteFileTool writes content to a file, creating parent directories as needed.
//
// WHAT:  Writes or overwrites a file at file_path, auto-creating missing parent directories.
// WHY:   The LLM needs to create and update files; mkdir -p equivalent avoids errors.
// PARAMS: workDir — function returning the current working directory for relative path resolution.
type WriteFileTool struct {
	workDir func() string
}

// NewWriteFileTool creates a WriteFileTool.
//
// PARAMS: workDir — closure returning the current working directory.
// RETURNS: *WriteFileTool — ready to execute.
func NewWriteFileTool(workDir func() string) *WriteFileTool {
	return &WriteFileTool{workDir: workDir}
}

// Name returns the tool's unique identifier.
func (t *WriteFileTool) Name() string {
	return "write_file"
}

// FormatArgs formats a concise UI label with the file path.
func (t *WriteFileTool) FormatArgs(args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[WriteFileArgs](args)
	if err != nil {
		return "Writing file"
	}
	if parsed.FilePath == "" {
		return "Writing file"
	}
	return truncateDisplay("Writing: "+parsed.FilePath, 50)
}

// Description returns the human-readable description for the LLM.
func (t *WriteFileTool) Description() string {
	return "file_path + content → write/overwrite file, auto-create parent dirs"
}

// Parameters returns the JSON schema for the tool's parameters.
func (t *WriteFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {
				"type": "string",
				"description": "file_path = absolute or relative path to the file to write"
			},
			"content": {
				"type": "string",
				"description": "content = full text to write to the file"
			}
		},
		"required": ["file_path", "content"]
	}`)
}

// Execute writes content to the file at file_path, creating parent directories.
//
// WHAT:  Creates or overwrites a file, ensuring parent directories exist.
// WHY:   The LLM must be able to create new files in arbitrary paths.
// HOW:   Resolves relative paths against workDir, calls os.MkdirAll on the parent dir, then os.WriteFile.
// PARAMS: ctx — turn cancellation context; args — raw JSON with file_path and content.
// RETURNS: string — "ok wrote <path>" or error description.
func (t *WriteFileTool) Execute(ctx context.Context, args json.RawMessage) string {
	if ctx != nil && ctx.Err() != nil {
		return "aborted before execution by user"
	}
	parsed, err := ParseToolCallArgs[WriteFileArgs](args)
	if err != nil {
		return fmt.Sprintf("error: invalid arguments: %v", err)
	}
	if parsed.FilePath == "" {
		return "error: file_path is required"
	}

	path := parsed.FilePath
	if !filepath.IsAbs(path) {
		if t.workDir == nil {
			return "error: write_file requires absolute file_path (workdir not available)"
		}
		wd := t.workDir()
		if wd == "" {
			return "error: write_file requires absolute file_path (workdir not available)"
		}
		path = filepath.Join(wd, path)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Sprintf("error: cannot create directory %s: %v", dir, err)
	}

	if err := os.WriteFile(path, []byte(parsed.Content), 0644); err != nil {
		return fmt.Sprintf("error: cannot write file %s: %v", path, err)
	}
	return fmt.Sprintf("ok wrote %s", path)
}
