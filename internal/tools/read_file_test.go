// read_file_test.go — tests for the read_file tool.
package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileSuccess(t *testing.T) {
	abs := writeTestFile(t, "hello world")
	tool := NewReadFileTool(func() string { return filepath.Dir(abs) })
	args, _ := json.Marshal(map[string]string{"file_path": abs})
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "<file_content "+abs+">") {
		t.Errorf("Execute() = %q, want <file_content> tag", result)
	}
	if !strings.Contains(result, "hello world") {
		t.Errorf("Execute() = %q, want file content", result)
	}
	if !strings.Contains(result, "</file_content>") {
		t.Errorf("Execute() = %q, want </file_content> tag", result)
	}
}

func TestReadFileRelativePath(t *testing.T) {
	abs := writeTestFile(t, "relative content")
	tool := NewReadFileTool(func() string { return filepath.Dir(abs) })
	rel := filepath.Base(abs)
	args, _ := json.Marshal(map[string]string{"file_path": rel})
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "relative content") {
		t.Errorf("Execute() = %q, want file content", result)
	}
	if !strings.Contains(result, "<file_content "+rel+">") {
		t.Errorf("Execute() = %q, want requested relative path in tag", result)
	}
}

func TestReadFileRelativePathNoWorkdir(t *testing.T) {
	tool := NewReadFileTool(nil)
	args, _ := json.Marshal(map[string]string{"file_path": "test.txt"})
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "absolute file_path") {
		t.Errorf("Execute() = %q, want 'absolute file_path' error", result)
	}
}

func TestReadFileNotFound(t *testing.T) {
	tool := NewReadFileTool(func() string { return "/tmp" })
	args, _ := json.Marshal(map[string]string{"file_path": "/nonexistent/path/file.txt"})
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "file not found") {
		t.Errorf("Execute() = %q, want 'file not found'", result)
	}
}

func TestReadFileEmpty(t *testing.T) {
	abs := writeTestFile(t, "")
	tool := NewReadFileTool(func() string { return filepath.Dir(abs) })
	args, _ := json.Marshal(map[string]string{"file_path": abs})
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "<file_content "+abs+">") {
		t.Errorf("Execute() = %q, want <file_content> tag", result)
	}
	if !strings.Contains(result, "</file_content>") {
		t.Errorf("Execute() = %q, want </file_content> tag", result)
	}
}

func TestReadFileName(t *testing.T) {
	tool := NewReadFileTool(nil)
	if tool.Name() != "read_file" {
		t.Errorf("Name() = %q, want 'read_file'", tool.Name())
	}
}

func TestReadFileParameters(t *testing.T) {
	tool := NewReadFileTool(nil)
	params := tool.Parameters()
	if !json.Valid(params) {
		t.Error("Parameters() is not valid JSON")
	}
}

func TestReadFileEmptyPath(t *testing.T) {
	tool := NewReadFileTool(func() string { return "/tmp" })
	args, _ := json.Marshal(map[string]string{"file_path": ""})
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "file_path is required") {
		t.Errorf("Execute() = %q, want 'file_path is required'", result)
	}
}

func TestReadFileAborted(t *testing.T) {
	tool := NewReadFileTool(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	args, _ := json.Marshal(map[string]string{"file_path": "/tmp/test.txt"})
	result := tool.Execute(ctx, args)
	if !strings.Contains(result, "aborted") {
		t.Errorf("Execute() = %q, want 'aborted'", result)
	}
}

func TestReadFileFormatArgs(t *testing.T) {
	tool := NewReadFileTool(func() string { return "/tmp" })
	args, _ := json.Marshal(map[string]string{"file_path": "/tmp/test.txt"})
	result := tool.FormatArgs(args)
	if !strings.Contains(result, "Reading") {
		t.Errorf("FormatArgs() = %q, want 'Reading'", result)
	}
}

func TestReadFileFormatArgsNoPath(t *testing.T) {
	tool := NewReadFileTool(nil)
	args := json.RawMessage(`{"file_path":""}`)
	result := tool.FormatArgs(args)
	if !strings.Contains(result, "Reading") {
		t.Errorf("FormatArgs() = %q, want 'Reading'", result)
	}
}

func TestReadFileFormatArgsInvalid(t *testing.T) {
	tool := NewReadFileTool(nil)
	args := json.RawMessage(`{invalid}`)
	result := tool.FormatArgs(args)
	if !strings.Contains(result, "Reading") {
		t.Errorf("FormatArgs() = %q, want 'Reading'", result)
	}
}

func TestReadFileDirectory(t *testing.T) {
	dir := t.TempDir()
	tool := NewReadFileTool(func() string { return dir })
	args, _ := json.Marshal(map[string]string{"file_path": dir})
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "error") {
		t.Errorf("Execute() = %q, want error for directory", result)
	}
}

func TestReadFileBinaryOk(t *testing.T) {
	abs := writeTestFile(t, "\x00binary\xff")
	tool := NewReadFileTool(func() string { return filepath.Dir(abs) })
	args, _ := json.Marshal(map[string]string{"file_path": abs})
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "<file_content "+abs+">") {
		t.Errorf("Execute() = %q, want <file_content> tag for binary file", result)
	}
}

func TestReadFileMultiLine(t *testing.T) {
	content := "line1\nline2\nline3\n"
	abs := writeTestFile(t, content)
	tool := NewReadFileTool(func() string { return filepath.Dir(abs) })
	args, _ := json.Marshal(map[string]string{"file_path": abs})
	result := tool.Execute(context.Background(), args)
	count := strings.Count(result, "line")
	if count != 3 {
		t.Errorf("Execute() returned %d 'line' occurrences, want 3", count)
	}
}

func TestReadFileLarge(t *testing.T) {
	// 100 KB — under the 300 KB limit, so read_file must succeed.
	large := strings.Repeat("x", 100000)
	abs := writeTestFile(t, large)
	tool := NewReadFileTool(func() string { return filepath.Dir(abs) })
	args, _ := json.Marshal(map[string]string{"file_path": abs})
	result := tool.Execute(context.Background(), args)
	if len(result) < 100000 {
		t.Errorf("Execute() returned %d bytes, want >= 100000", len(result))
	}
}

func TestReadFileTooLarge(t *testing.T) {
	// ~400 KB — over the 300 KB limit, must return an error.
	large := strings.Repeat("x", 400000)
	abs := writeTestFile(t, large)
	tool := NewReadFileTool(func() string { return filepath.Dir(abs) })
	args, _ := json.Marshal(map[string]string{"file_path": abs})
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "file too large") {
		t.Errorf("Execute() = %q, want 'file too large' error", result)
	}
	if !strings.Contains(result, "rg") || !strings.Contains(result, "head") {
		t.Errorf("Execute() = %q, want tool suggestions (rg, head/tail)", result)
	}
}

func TestReadFileFormatArgsWithPurpose(t *testing.T) {
	tool := NewReadFileTool(nil)
	args, _ := json.Marshal(map[string]string{
		"file_path": "/tmp/test.txt",
		"purpose":   "Read config.json to check current compaction thresholds for the task-switcher feature.",
	})
	result := tool.FormatArgs(args)
	if result != "Read config.json to check current compaction thresholds for the task-switcher feature." {
		t.Errorf("FormatArgs() = %q, want purpose text", result)
	}
}

func TestReadFileFormatArgsPurposeFallback(t *testing.T) {
	tool := NewReadFileTool(func() string { return "/tmp" })
	// Empty purpose → fallback to path
	args, _ := json.Marshal(map[string]string{"file_path": "/tmp/test.txt", "purpose": ""})
	result := tool.FormatArgs(args)
	if !strings.Contains(result, "Reading") {
		t.Errorf("FormatArgs() = %q, want 'Reading' fallback when purpose empty", result)
	}
}

func TestReadFileParametersHasPurpose(t *testing.T) {
	tool := NewReadFileTool(nil)
	params := string(tool.Parameters())
	if !strings.Contains(params, `"purpose"`) {
		t.Errorf("Parameters() missing 'purpose' field: %s", params)
	}
}

func TestReadFilePermissionDenied(t *testing.T) {
	abs := writeTestFile(t, "secret")
	if err := os.Chmod(abs, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(abs, 0644) })
	tool := NewReadFileTool(func() string { return filepath.Dir(abs) })
	args, _ := json.Marshal(map[string]string{"file_path": abs})
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "cannot read file") {
		t.Errorf("Execute() = %q, want 'cannot read file' error", result)
	}
}
