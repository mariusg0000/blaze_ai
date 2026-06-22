// replace_block.go — replace_block tool implementation.
// Replaces a block of text in a file by matching an old block and writing a new block.
// Layer: tool execution. Dependencies: none (file IO only).
package tools

import (
	"encoding/json"
	"fmt"
	"os"
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
}

// ReplaceBlockTool replaces a block of text in a file.
//
// WHAT:  Finds an exact block of text in a file and replaces it with new text.
// WHY:   Enables the LLM to make precise edits to existing files without rewriting them.
type ReplaceBlockTool struct{}

// NewReplaceBlockTool creates a ReplaceBlockTool.
//
// RETURNS: *ReplaceBlockTool — ready to execute.
func NewReplaceBlockTool() *ReplaceBlockTool {
	return &ReplaceBlockTool{}
}

// Name returns the tool's unique identifier.
func (t *ReplaceBlockTool) Name() string {
	return "replace_block"
}

// Description returns the human-readable description for the LLM.
func (t *ReplaceBlockTool) Description() string {
	return "Replace an exact block of text in a file with a new block. The old block must match exactly."
}

// Parameters returns the JSON schema for the tool's parameters.
func (t *ReplaceBlockTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {
				"type": "string",
				"description": "The path to the file to edit."
			},
			"old_block": {
				"type": "string",
				"description": "The exact text block to find and replace."
			},
			"new_block": {
				"type": "string",
				"description": "The new text block to write in place of the old block."
			}
		},
		"required": ["file_path", "old_block", "new_block"]
	}`)
}

// Execute reads the file, replaces old_block with new_block, and writes it back.
//
// WHAT:  Performs the text replacement in the target file.
// WHY:   Precise file editing without full rewrites.
// HOW:   Reads file, finds exact match of old_block, replaces first occurrence, writes back.
// PARAMS: args — raw JSON with file_path, old_block, new_block.
// RETURNS: string — success message or error description.
func (t *ReplaceBlockTool) Execute(args json.RawMessage) string {
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

	return fmt.Sprintf("block replaced in %s", parsed.FilePath)
}
