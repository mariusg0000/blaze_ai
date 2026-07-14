// replace_block.go — replace_block tool implementation.
// Replaces exact text blocks in one file and reports failed blocks with live content.
// Layer: tool execution. Dependencies: file IO only.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const replaceBlockFileContentLimit = 500000

// ReplaceBlock is one exact text replacement requested for a file.
//
// WHAT:  Describes the source text and its replacement.
// PARAMS: OldBlock — exact text to find; NewBlock — replacement text.
type ReplaceBlock struct {
	OldBlock string `json:"old_block"`
	NewBlock string `json:"new_block"`
}

// ReplaceBlockArgs are the arguments for the replace_block tool.
//
// WHAT:  Parsed arguments from the LLM tool call.
// PARAMS: FilePath — target file; Blocks — exact replacements; OldBlock/NewBlock — legacy single replacement.
type ReplaceBlockArgs struct {
	FilePath string         `json:"file_path"`
	Blocks   []ReplaceBlock `json:"blocks,omitempty"`
	OldBlock string         `json:"old_block,omitempty"`
	NewBlock string         `json:"new_block,omitempty"`
	Purpose  string         `json:"purpose,omitempty"`
}

// ReplaceBlockTool replaces exact blocks of text in one file.
//
// WHAT:  Applies every uniquely matching block and reports unmatched blocks.
// WHY:   Enables one atomic edit request for several related changes.
// PARAMS: workDir — function returning the current working directory for relative paths.
type ReplaceBlockTool struct {
	workDir func() string
}

// NewReplaceBlockTool creates a ReplaceBlockTool.
//
// PARAMS: workDir — closure returning the current working directory for relative paths.
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
	return "file_path + blocks → replace every uniquely matching exact block in one file; MUST batch all independent edits to the same file into one call; prefer blocks; after partial success, applied blocks are already on disk and must never be resent; failed blocks are returned with authoritative live file content when under 500000 bytes, and retries must rebuild exact old_block text from that live version"
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
				"description": "file_path = one target file path; blocks must all apply to this file"
			},
			"blocks": {
				"type": "array",
				"description": "MANDATORY: Put all independent edits for this file in one blocks array and send one call. Each old_block must match exactly once, including whitespace, indentation, and newlines. Do not split independent same-file edits into sequential calls. If partial success occurs, successful blocks are already on disk: retry only failed block indices, rebuild old_block exactly from the AUTHORITATIVE LIVE FILE CONTENT returned by the tool, and never reuse stale text from the original context.",
				"items": {
					"type": "object",
					"properties": {
						"old_block": {"type": "string", "description": "Exact existing text, including whitespace, indentation, and newlines."},
						"new_block": {"type": "string", "description": "Replacement text; empty string deletes old_block."}
					},
					"required": ["old_block", "new_block"]
				}
			},
			"old_block": {
				"type": "string",
				"description": "Legacy single-block form. Prefer blocks. Exact existing text, including whitespace and newlines."
			},
			"new_block": {
				"type": "string",
				"description": "Legacy single-block replacement; empty string deletes old_block."
			}
		},
		"required": ["purpose", "file_path"],
		"oneOf": [
			{"required": ["blocks"]},
			{"required": ["old_block", "new_block"]}
		]
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

// replacementBlocks resolves the new multiblock format or the legacy single-block format.
//
// WHAT:  Converts parsed arguments into one uniform block list.
// HOW:   Rejects mixed formats and preserves the legacy format for existing callers.
func replacementBlocks(parsed ReplaceBlockArgs) ([]ReplaceBlock, error) {
	if len(parsed.Blocks) > 0 {
		if parsed.OldBlock != "" || parsed.NewBlock != "" {
			return nil, fmt.Errorf("blocks cannot be combined with old_block or new_block")
		}
		return parsed.Blocks, nil
	}
	if parsed.OldBlock == "" {
		return nil, fmt.Errorf("blocks or old_block is required")
	}
	return []ReplaceBlock{{OldBlock: parsed.OldBlock, NewBlock: parsed.NewBlock}}, nil
}

// countMatches returns all exact, including overlapping, match offsets.
//
// WHAT:  Detects missing and ambiguous blocks before replacement.
// HOW:   Advances one byte after each match so every exact occurrence is reported.
func countMatches(content, block string) []int {
	var offsets []int
	for offset := 0; offset <= len(content)-len(block); {
		index := strings.Index(content[offset:], block)
		if index < 0 {
			break
		}
		position := offset + index
		offsets = append(offsets, position)
		offset = position + 1
	}
	return offsets
}

// formatFailedBlock formats one failed replacement exactly as received.
//
// WHAT:  Returns the failure reason and both original block fields.
// HOW:   Keeps old_block and new_block available for a direct retry.
func formatFailedBlock(index int, reason string, block ReplaceBlock) string {
	return fmt.Sprintf("[block %d] reason: %s\nold_block:\n%s\nnew_block:\n%s", index, reason, block.OldBlock, block.NewBlock)
}

// failedBlockReport formats failed blocks and the current file content for the LLM.
//
// WHAT:  Returns exact failed input and the live file state.
// HOW:   Includes the full file only below the configured byte limit.
func failedBlockReport(path, content string, applied []int, failures []string) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("error: %d block(s) failed in %s\n", len(failures), path))
	builder.WriteString(fmt.Sprintf("applied_blocks: %v\n", applied))
	builder.WriteString("successful blocks are already on disk; do not resend them.\n\nFailed blocks:\n")
	for _, failure := range failures {
		builder.WriteString(failure)
		builder.WriteString("\n")
	}
	if len([]byte(content)) < replaceBlockFileContentLimit {
		builder.WriteString("\nAUTHORITATIVE LIVE FILE CONTENT — READ THIS VERSION FOR RETRY\n")
		builder.WriteString("The following content was read directly from disk after the successful replacements. It supersedes the old file content in the conversation context. Rebuild old_block values from this version only.\n")
		builder.WriteString("--- LIVE FILE START ---\n")
		builder.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			builder.WriteByte('\n')
		}
		builder.WriteString("--- LIVE FILE END ---\n")
		builder.WriteString("MANDATORY RETRY RULE: retry only the failed block indices above, using exact text copied from the authoritative live file; never resend applied blocks or reuse stale old_block text.")
	} else {
		builder.WriteString(fmt.Sprintf("\nAUTHORITATIVE LIVE FILE CONTENT OMITTED: file is %d bytes and exceeds the 500000-byte limit. Inspect the current file before retrying. Retry only the failed block indices above.", len([]byte(content))))
	}
	return builder.String()
}

// Execute reads, validates, and applies all requested replacements.
//
// WHAT:  Applies uniquely matching exact blocks and reports only failures.
// WHY:   Allows partial progress while keeping matching explicit and error-free.
// HOW:   Processes blocks against the evolving in-memory content, writes successful changes once, then reads the live file for failure reports.
// PARAMS: ctx — turn cancellation context; args — raw JSON with one file path and blocks.
// RETURNS: string — success message or detailed failure report.
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
	blocks, err := replacementBlocks(parsed)
	if err != nil {
		return "error: " + err.Error()
	}
	if len(blocks) == 0 {
		return "error: blocks must contain at least one block"
	}
	for index, block := range blocks {
		if block.OldBlock == "" {
			return fmt.Sprintf("error: block %d old_block is required", index+1)
		}
	}

	path := parsed.FilePath
	if !filepath.IsAbs(path) {
		if t.workDir == nil || t.workDir() == "" {
			return "error: replace_block requires absolute file_path (workdir not available)"
		}
		path = filepath.Join(t.workDir(), path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("error: cannot read file %s: %v", path, err)
	}
	content := string(data)
	originalInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Sprintf("error: cannot stat file %s: %v", path, err)
	}

	failures := make([]string, 0)
	applied := make([]int, 0, len(blocks))
	replaced := 0
	for index, block := range blocks {
		if ctx != nil && ctx.Err() != nil {
			return "aborted before writing by user"
		}
		matches := countMatches(content, block.OldBlock)
		if len(matches) == 0 {
			failures = append(failures, formatFailedBlock(index+1, "exact match not found", block))
			continue
		}
		if len(matches) != 1 {
			failures = append(failures, formatFailedBlock(index+1, fmt.Sprintf("exact match is ambiguous (%d matches)", len(matches)), block))
			continue
		}
		content = content[:matches[0]] + block.NewBlock + content[matches[0]+len(block.OldBlock):]
		applied = append(applied, index+1)
		replaced++
	}

	if replaced > 0 {
		if err := os.WriteFile(path, []byte(content), originalInfo.Mode().Perm()); err != nil {
			return fmt.Sprintf("error: cannot write file %s: %v", path, err)
		}
	}
	if len(failures) == 0 {
		return fmt.Sprintf("ok: block replaced %d block(s) in %s", replaced, path)
	}
	live, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("error: replacements applied but cannot read live file %s: %v", path, err)
	}
	return failedBlockReport(path, string(live), applied, failures)
}
