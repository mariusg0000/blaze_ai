// replace_block.go — replace_block tool implementation.
// Replaces a block of text in a file by matching an old block and writing a new block.
// Layer: tool execution. Dependencies: none (file IO only).
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReplaceBlockArgs are the arguments for the replace_block tool.
//
// WHAT:  Parsed arguments from the LLM tool call.
// PARAMS: FilePath — target file; OldBlock — exact text to find; NewBlock — replacement text.
type ReplaceBlockArgs struct {
	FilePath string `json:"file_path"`
	OldBlock string `json:"old_block"`
	NewBlock string `json:"new_block"`
	Purpose  string `json:"purpose,omitempty"`
}

// ReplaceBlockTool replaces a block of text in a file.
//
// WHAT:  Finds an exact block of text in a file and replaces it with new text.
// WHY:   Enables the LLM to make precise edits to existing files without rewriting them.
// PARAMS: workDir — function returning the current working directory for relative UI paths.
type ReplaceBlockTool struct {
	workDir func() string
}

// NewReplaceBlockTool creates a ReplaceBlockTool.
//
// PARAMS: workDir — closure returning the current working directory for display formatting.
// RETURNS: *ReplaceBlockTool — ready to execute.
func NewReplaceBlockTool(workDir func() string) *ReplaceBlockTool {
	return &ReplaceBlockTool{workDir: workDir}
}

// Name returns the tool's unique identifier.
func (t *ReplaceBlockTool) Name() string {
	return "replace_block"
}

// FormatArgs formats a concise UI label with relative path and edit purpose.
func (t *ReplaceBlockTool) FormatArgs(args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[ReplaceBlockArgs](args)
	if err != nil {
		return "Editing file"
	}
	path := t.displayPath(parsed.FilePath)
	purpose := strings.TrimSpace(parsed.Purpose)
	if path == "" && purpose == "" {
		return "Editing file"
	}
	if path == "" {
		return "Editing: " + purpose
	}
	if purpose == "" {
		return truncateDisplay("Editing: "+path, 50)
	}
	return "Editing: " + path + " — " + purpose
}

// Description returns the human-readable description for the LLM.
func (t *ReplaceBlockTool) Description() string {
	return "file_path + old_block + new_block → replace first exact match; old_block = exact text incl whitespace + newlines"
}

// Parameters returns the JSON schema for the tool's parameters.
func (t *ReplaceBlockTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"purpose": {
				"type": "string",
				"description": "purpose = exactly 3 user-visible sentences. Sentence 1 must name the target file and the specific code/text area being edited. Sentence 2 must explain why the edit is needed. Sentence 3 must explain what the replacement will change and how it solves or advances the task."
			},
			"file_path": {
				"type": "string",
				"description": "file_path = target file path"
			},
			"old_block": {
				"type": "string",
				"description": "old_block = exact existing text; match = exact incl whitespace + newlines"
			},
			"new_block": {
				"type": "string",
				"description": "new_block = replacement text; empty string → delete old_block"
			}
		},
		"required": ["purpose", "file_path", "old_block", "new_block"]
	}`)
}

// displayPath converts an absolute file path to a working-directory-relative display path when possible.
func (t *ReplaceBlockTool) displayPath(path string) string {
	if path == "" {
		return ""
	}
	if t.workDir == nil {
		return path
	}
	workDir := t.workDir()
	if workDir == "" {
		return path
	}
	rel, err := filepath.Rel(workDir, path)
	if err != nil {
		return path
	}
	return rel
}

// Execute reads the file, replaces old_block with new_block, and writes it back.
//
// WHAT:  Performs the text replacement in the target file.
// WHY:   Precise file editing without full rewrites.
// HOW:   Reads file, finds exact match of old_block, replaces first occurrence, writes back.
// PARAMS: ctx — turn cancellation context; args — raw JSON with file_path, old_block, new_block.
// RETURNS: string — success message or error description.
func (t *ReplaceBlockTool) Execute(ctx context.Context, args json.RawMessage) string {
	if ctx != nil && ctx.Err() != nil {
		return "aborted before execution by user"
	}
	parsed, err := ParseToolCallArgs[ReplaceBlockArgs](args)
	if err != nil {
		return fmt.Sprintf("error: invalid arguments: %v", err)
	}
	if parsed.FilePath == "" {
		return "error: file_path is required"
	}
	if parsed.OldBlock == "" {
		return "error: old_block is required"
	}

	data, err := os.ReadFile(parsed.FilePath)
	if err != nil {
		return fmt.Sprintf("error: cannot read file %s: %v", parsed.FilePath, err)
	}

	content := string(data)
	if !strings.Contains(content, parsed.OldBlock) {
		return fmt.Sprintf("error: old_block not found in %s", parsed.FilePath)
	}

	newContent := strings.Replace(content, parsed.OldBlock, parsed.NewBlock, 1)
	if err := os.WriteFile(parsed.FilePath, []byte(newContent), 0644); err != nil {
		return fmt.Sprintf("error: cannot write file %s: %v", parsed.FilePath, err)
	}

	return fmt.Sprintf("ok block replaced in %s", parsed.FilePath)
}
