// write_file_test.go — tests for the write_file tool.
package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileSuccess(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "output.txt")
	tool := NewWriteFileTool(func() string { return dir })
	args, _ := json.Marshal(map[string]string{
		"file_path": target,
		"content":   "hello",
	})
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "ok wrote") {
		t.Errorf("Execute() = %q, want 'ok wrote'", result)
	}
	data, _ := os.ReadFile(target)
	if string(data) != "hello" {
		t.Errorf("file content = %q, want 'hello'", string(data))
	}
}

func TestWriteFileRelativePath(t *testing.T) {
	dir := t.TempDir()
	tool := NewWriteFileTool(func() string { return dir })
	args, _ := json.Marshal(map[string]string{
		"file_path": "output.txt",
		"content":   "relative",
	})
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "ok wrote") {
		t.Errorf("Execute() = %q, want 'ok wrote'", result)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "output.txt"))
	if string(data) != "relative" {
		t.Errorf("file content = %q, want 'relative'", string(data))
	}
}

func TestWriteFileRelativePathNoWorkdir(t *testing.T) {
	tool := NewWriteFileTool(nil)
	args, _ := json.Marshal(map[string]string{
		"file_path": "test.txt",
		"content":   "x",
	})
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "absolute file_path") {
		t.Errorf("Execute() = %q, want 'absolute file_path' error", result)
	}
}

func TestWriteFileCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a", "b", "c", "deep.txt")
	tool := NewWriteFileTool(func() string { return dir })
	args, _ := json.Marshal(map[string]string{
		"file_path": target,
		"content":   "deep",
	})
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "ok wrote") {
		t.Errorf("Execute() = %q, want 'ok wrote'", result)
	}
	data, _ := os.ReadFile(target)
	if string(data) != "deep" {
		t.Errorf("file content = %q, want 'deep'", string(data))
	}
}

func TestWriteFileRelativeCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	tool := NewWriteFileTool(func() string { return dir })
	args, _ := json.Marshal(map[string]string{
		"file_path": "x/y/z.txt",
		"content":   "nested",
	})
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "ok wrote") {
		t.Errorf("Execute() = %q, want 'ok wrote'", result)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "x", "y", "z.txt"))
	if string(data) != "nested" {
		t.Errorf("file content = %q, want 'nested'", string(data))
	}
}

func TestWriteFileOverwrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "existing.txt")
	os.WriteFile(target, []byte("old"), 0644)
	tool := NewWriteFileTool(func() string { return dir })
	args, _ := json.Marshal(map[string]string{
		"file_path": target,
		"content":   "new",
	})
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "ok wrote") {
		t.Errorf("Execute() = %q, want 'ok wrote'", result)
	}
	data, _ := os.ReadFile(target)
	if string(data) != "new" {
		t.Errorf("file content = %q, want 'new'", string(data))
	}
}

func TestWriteFileEmptyContent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "empty.txt")
	os.WriteFile(target, []byte("not empty"), 0644)
	tool := NewWriteFileTool(func() string { return dir })
	args, _ := json.Marshal(map[string]string{
		"file_path": target,
		"content":   "",
	})
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "ok wrote") {
		t.Errorf("Execute() = %q, want 'ok wrote'", result)
	}
	data, _ := os.ReadFile(target)
	if len(data) != 0 {
		t.Errorf("file content length = %d, want 0", len(data))
	}
}

func TestWriteFileName(t *testing.T) {
	tool := NewWriteFileTool(nil)
	if tool.Name() != "write_file" {
		t.Errorf("Name() = %q, want 'write_file'", tool.Name())
	}
}

func TestWriteFileParameters(t *testing.T) {
	tool := NewWriteFileTool(nil)
	params := tool.Parameters()
	if !json.Valid(params) {
		t.Error("Parameters() is not valid JSON")
	}
}

func TestWriteFileEmptyPath(t *testing.T) {
	tool := NewWriteFileTool(func() string { return "/tmp" })
	args, _ := json.Marshal(map[string]string{
		"file_path": "",
		"content":   "x",
	})
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "file_path is required") {
		t.Errorf("Execute() = %q, want 'file_path is required'", result)
	}
}

func TestWriteFileAborted(t *testing.T) {
	tool := NewWriteFileTool(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	args, _ := json.Marshal(map[string]string{
		"file_path": "/tmp/test.txt",
		"content":   "x",
	})
	result := tool.Execute(ctx, args)
	if !strings.Contains(result, "aborted") {
		t.Errorf("Execute() = %q, want 'aborted'", result)
	}
}

func TestWriteFileFormatArgs(t *testing.T) {
	tool := NewWriteFileTool(func() string { return "/tmp" })
	args, _ := json.Marshal(map[string]string{"file_path": "/tmp/test.txt", "content": "x"})
	result := tool.FormatArgs(args)
	if !strings.Contains(result, "Writing") {
		t.Errorf("FormatArgs() = %q, want 'Writing'", result)
	}
}

func TestWriteFileFormatArgsNoPath(t *testing.T) {
	tool := NewWriteFileTool(nil)
	args := json.RawMessage(`{"file_path":"","content":"x"}`)
	result := tool.FormatArgs(args)
	if !strings.Contains(result, "Writing") {
		t.Errorf("FormatArgs() = %q, want 'Writing'", result)
	}
}

func TestWriteFileFormatArgsInvalid(t *testing.T) {
	tool := NewWriteFileTool(nil)
	args := json.RawMessage(`{invalid}`)
	result := tool.FormatArgs(args)
	if !strings.Contains(result, "Writing") {
		t.Errorf("FormatArgs() = %q, want 'Writing'", result)
	}
}

func TestWriteFileInvalidArgs(t *testing.T) {
	tool := NewWriteFileTool(nil)
	args := json.RawMessage(`{invalid}`)
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "error") {
		t.Errorf("Execute() = %q, want error", result)
	}
}

func TestWriteFilePermissionDeniedDir(t *testing.T) {
	dir := t.TempDir()
	locked := filepath.Join(dir, "locked")
	os.Mkdir(locked, 0444)
	t.Cleanup(func() { os.Chmod(locked, 0755) })
	target := filepath.Join(locked, "sub", "file.txt")
	tool := NewWriteFileTool(func() string { return dir })
	args, _ := json.Marshal(map[string]string{
		"file_path": target,
		"content":   "x",
	})
	result := tool.Execute(context.Background(), args)
	if !strings.Contains(result, "error") {
		t.Errorf("Execute() = %q, want error for permission denied", result)
	}
}
