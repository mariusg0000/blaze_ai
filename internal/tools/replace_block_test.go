// replace_block_test.go — tests for the replace_block tool.
package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestFile writes content to a temp file and returns the path.
func writeTestFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("cannot write test file: %v", err)
	}
	return path
}

// TestReplaceBlockSuccess verifies a basic block replacement.
func TestReplaceBlockSuccess(t *testing.T) {
	path := writeTestFile(t, "line1\nold block\nline3")
	tool := NewReplaceBlockTool()
	args := json.RawMessage(`{"file_path":"` + path + `","old_block":"old block","new_block":"new block"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "block replaced") {
		t.Errorf("Execute() = %q, want 'block replaced'", result)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "new block") {
		t.Errorf("file content = %q, want 'new block'", string(data))
	}
	if strings.Contains(string(data), "old block") {
		t.Errorf("file still contains 'old block'")
	}
}

// TestReplaceBlockNotFound verifies error when old_block is not in the file.
func TestReplaceBlockNotFound(t *testing.T) {
	path := writeTestFile(t, "line1\nline2")
	tool := NewReplaceBlockTool()
	args := json.RawMessage(`{"file_path":"` + path + `","old_block":"nonexistent","new_block":"replacement"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "not found") {
		t.Errorf("Execute() = %q, want 'not found' error", result)
	}
}

// TestReplaceBlockFileMissing verifies error when file does not exist.
func TestReplaceBlockFileMissing(t *testing.T) {
	tool := NewReplaceBlockTool()
	args := json.RawMessage(`{"file_path":"/nonexistent/path/file.txt","old_block":"old","new_block":"new"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "error") {
		t.Errorf("Execute() = %q, want error message", result)
	}
}

// TestReplaceBlockEmptyFilePath verifies error on empty file_path.
func TestReplaceBlockEmptyFilePath(t *testing.T) {
	tool := NewReplaceBlockTool()
	args := json.RawMessage(`{"file_path":"","old_block":"old","new_block":"new"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "error") {
		t.Errorf("Execute() = %q, want error message", result)
	}
}

// TestReplaceBlockEmptyOldBlock verifies error on empty old_block.
func TestReplaceBlockEmptyOldBlock(t *testing.T) {
	path := writeTestFile(t, "content")
	tool := NewReplaceBlockTool()
	args := json.RawMessage(`{"file_path":"` + path + `","old_block":"","new_block":"new"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "error") {
		t.Errorf("Execute() = %q, want error message", result)
	}
}

// TestReplaceBlockMultiline verifies replacement of a multiline block.
func TestReplaceBlockMultiline(t *testing.T) {
	path := writeTestFile(t, "header\nold line 1\nold line 2\nfooter")
	tool := NewReplaceBlockTool()
	args := json.RawMessage(`{"file_path":"` + path + `","old_block":"old line 1\nold line 2","new_block":"new line 1\nnew line 2"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "block replaced") {
		t.Errorf("Execute() = %q, want 'block replaced'", result)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "new line 1") || !strings.Contains(string(data), "new line 2") {
		t.Errorf("file content = %q, want new lines", string(data))
	}
}

// TestReplaceBlockOnlyFirstOccurrence verifies only the first match is replaced.
func TestReplaceBlockOnlyFirstOccurrence(t *testing.T) {
	path := writeTestFile(t, "target\ntarget\ntarget")
	tool := NewReplaceBlockTool()
	args := json.RawMessage(`{"file_path":"` + path + `","old_block":"target","new_block":"replaced"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "block replaced") {
		t.Errorf("Execute() = %q, want 'block replaced'", result)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	count := strings.Count(content, "target")
	if count != 2 {
		t.Errorf("file has %d 'target' remaining, want 2 (only first replaced)", count)
	}
}

// TestReplaceBlockInvalidArgs verifies error on invalid JSON.
func TestReplaceBlockInvalidArgs(t *testing.T) {
	tool := NewReplaceBlockTool()
	args := json.RawMessage(`{invalid}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "error") {
		t.Errorf("Execute() = %q, want error message", result)
	}
}

// TestReplaceBlockName verifies the tool name.
func TestReplaceBlockName(t *testing.T) {
	tool := NewReplaceBlockTool()
	if tool.Name() != "replace_block" {
		t.Errorf("Name() = %q, want 'replace_block'", tool.Name())
	}
}

// TestReplaceBlockParameters verifies parameters is valid JSON.
func TestReplaceBlockParameters(t *testing.T) {
	tool := NewReplaceBlockTool()
	params := tool.Parameters()
	if !json.Valid(params) {
		t.Error("Parameters() is not valid JSON")
	}
	schemaIncludesRequiredPurpose(t, params)
}
