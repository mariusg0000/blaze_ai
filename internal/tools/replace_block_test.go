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
	tool := NewReplaceBlockTool(func() string { return filepath.Dir(path) })
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
	tool := NewReplaceBlockTool(nil)
	args := json.RawMessage(`{"file_path":"` + path + `","old_block":"nonexistent","new_block":"replacement"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "not found") {
		t.Errorf("Execute() = %q, want 'not found' error", result)
	}
}

// TestReplaceBlockFileMissing verifies error when file does not exist.
func TestReplaceBlockFileMissing(t *testing.T) {
	tool := NewReplaceBlockTool(nil)
	args := json.RawMessage(`{"file_path":"/nonexistent/path/file.txt","old_block":"old","new_block":"new"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "error") {
		t.Errorf("Execute() = %q, want error message", result)
	}
}

// TestReplaceBlockEmptyFilePath verifies error on empty file_path.
func TestReplaceBlockEmptyFilePath(t *testing.T) {
	tool := NewReplaceBlockTool(nil)
	args := json.RawMessage(`{"file_path":"","old_block":"old","new_block":"new"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "error") {
		t.Errorf("Execute() = %q, want error message", result)
	}
}

// TestReplaceBlockEmptyOldBlock verifies error on empty old_block.
func TestReplaceBlockEmptyOldBlock(t *testing.T) {
	path := writeTestFile(t, "content")
	tool := NewReplaceBlockTool(func() string { return filepath.Dir(path) })
	args := json.RawMessage(`{"file_path":"` + path + `","old_block":"","new_block":"new"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "error") {
		t.Errorf("Execute() = %q, want error message", result)
	}
}

// TestReplaceBlockMultiline verifies replacement of a multiline block.
func TestReplaceBlockMultiline(t *testing.T) {
	path := writeTestFile(t, "header\nold line 1\nold line 2\nfooter")
	tool := NewReplaceBlockTool(func() string { return filepath.Dir(path) })
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

// TestReplaceBlockDuplicateMatch verifies duplicate exact matches are rejected.
func TestReplaceBlockDuplicateMatch(t *testing.T) {
	path := writeTestFile(t, "target\ntarget\ntarget")
	tool := NewReplaceBlockTool(func() string { return filepath.Dir(path) })
	args := json.RawMessage(`{"file_path":"` + path + `","old_block":"target","new_block":"replaced"}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "ambiguous") {
		t.Errorf("Execute() = %q, want ambiguous-match error", result)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "target\ntarget\ntarget" {
		t.Errorf("file content changed after ambiguous match: %q", string(data))
	}
}

// TestReplaceBlockMultiplePartialSuccess verifies successful blocks persist while failed blocks are reported.
func TestReplaceBlockMultiplePartialSuccess(t *testing.T) {
	path := writeTestFile(t, "first old\nsecond old\nfooter")
	tool := NewReplaceBlockTool(func() string { return filepath.Dir(path) })
	args, err := json.Marshal(map[string]any{
		"file_path": path,
		"blocks": []map[string]string{
			{"old_block": "first old", "new_block": "first new"},
			{"old_block": "missing old", "new_block": "missing new"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "1 block(s) failed") || !strings.Contains(result, "first new") || !strings.Contains(result, "missing old") {
		t.Errorf("Execute() = %q, want partial result with failed block and live file", result)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "first new\nsecond old\nfooter" {
		t.Errorf("file content = %q, want successful replacement only", string(data))
	}
}

// TestReplaceBlockMultipleSuccess verifies all uniquely matching blocks are applied.
func TestReplaceBlockMultipleSuccess(t *testing.T) {
	path := writeTestFile(t, "first old\nsecond old")
	tool := NewReplaceBlockTool(func() string { return filepath.Dir(path) })
	args, err := json.Marshal(map[string]any{
		"file_path": path,
		"blocks": []map[string]string{
			{"old_block": "first old", "new_block": "first new"},
			{"old_block": "second old", "new_block": "second new"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "replaced 2 block(s)") {
		t.Errorf("Execute() = %q, want two replacements", result)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "first new\nsecond new" {
		t.Errorf("file content = %q, want both replacements", string(data))
	}
}

// TestReplaceBlockInvalidArgs verifies error on invalid JSON.
func TestReplaceBlockInvalidArgs(t *testing.T) {
	tool := NewReplaceBlockTool(nil)
	args := json.RawMessage(`{invalid}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "error") {
		t.Errorf("Execute() = %q, want error message", result)
	}
}

// TestReplaceBlockName verifies the tool name.
func TestReplaceBlockName(t *testing.T) {
	tool := NewReplaceBlockTool(nil)
	if tool.Name() != "replace_block" {
		t.Errorf("Name() = %q, want 'replace_block'", tool.Name())
	}
}

// TestReplaceBlockRelativePath verifies relative paths are resolved against the workdir.
func TestReplaceBlockRelativePath(t *testing.T) {
	abs := writeTestFile(t, "alpha")
	tool := NewReplaceBlockTool(func() string { return filepath.Dir(abs) })
	rel := filepath.Base(abs)
	args, err := json.Marshal(map[string]string{
		"file_path": rel,
		"old_block": "alpha",
		"new_block": "beta",
	})
	if err != nil {
		t.Fatal(err)
	}
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "block replaced") {
		t.Errorf("Execute() = %q, want 'block replaced'", result)
	}
	data, _ := os.ReadFile(abs)
	if !strings.Contains(string(data), "beta") {
		t.Errorf("file content = %q, want 'beta'", string(data))
	}
}

// TestReplaceBlockRelativePathNoWorkdir verifies error when workdir is nil and path is relative.
func TestReplaceBlockRelativePathNoWorkdir(t *testing.T) {
	tool := NewReplaceBlockTool(nil)
	args, err := json.Marshal(map[string]string{
		"file_path": "test.txt",
		"old_block": "alpha",
		"new_block": "beta",
	})
	if err != nil {
		t.Fatal(err)
	}
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "absolute file_path") {
		t.Errorf("Execute() = %q, want 'absolute file_path' error", result)
	}
}

// TestReplaceBlockParameters verifies parameters is valid JSON.
func TestReplaceBlockParameters(t *testing.T) {
	tool := NewReplaceBlockTool(nil)
	params := tool.Parameters()
	if !json.Valid(params) {
		t.Error("Parameters() is not valid JSON")
	}
	schemaIncludesRequiredPurpose(t, params)
}
