// files_tools_test.go — tests for fd and rg read-only wrappers.
// Verifies direct helper execution, argument validation, and bounded result output.
// Layer: tool execution tests. Dependencies: temporary filesystem and installed helpers.
package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestListFilesToolUsesFd verifies that list_files returns entries from its work directory.
func TestListFilesToolUsesFd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "target.txt"), []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	tool := NewListFilesTool(func() string { return dir })
	result := tool.Execute(context.Background(), json.RawMessage(`{"purpose":"Inspect the temporary project files. Search the work directory for text files. Use the listing to identify the target file."}`))
	if !strings.Contains(result, "target.txt") {
		t.Fatalf("list_files result = %q, want target.txt", result)
	}
}

// TestSearchFilesToolUsesRg verifies that search_files returns rg matches and status.
func TestSearchFilesToolUsesRg(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "target.txt"), []byte("needle\nother\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tool := NewSearchFilesTool(func() string { return dir })
	result := tool.Execute(context.Background(), json.RawMessage(`{"purpose":"Search the temporary project files for a marker. Search recursively with rg in the work directory. Use the matching line to verify the helper wrapper." ,"pattern":"needle"}`))
	if !strings.Contains(result, "exit_code: 0") || !strings.Contains(result, "target.txt:1:needle") {
		t.Fatalf("search_files result = %q, want successful matching output", result)
	}
}

// TestSearchFilesToolRequiresPattern verifies that an empty pattern fails explicitly.
func TestSearchFilesToolRequiresPattern(t *testing.T) {
	tool := NewSearchFilesTool(nil)
	result := tool.Execute(context.Background(), json.RawMessage(`{"purpose":"Search the project for a missing pattern. The scope is the current directory. The result should identify the validation problem."}`))
	if result != "error: pattern is required" {
		t.Fatalf("search_files validation = %q, want explicit pattern error", result)
	}
}

// TestHelperMissingReturnsInstallGuidance verifies that missing dependencies do not fall back.
func TestHelperMissingReturnsInstallGuidance(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	listResult := NewListFilesTool(nil).Execute(context.Background(), json.RawMessage(`{"purpose":"Inspect the project directory contents. Search the default scope for entries. The result should identify missing helper setup."}`))
	if !strings.Contains(listResult, "fd is not installed") || !strings.Contains(listResult, "Install") {
		t.Fatalf("list_files missing helper result = %q, want install guidance", listResult)
	}

	searchResult := NewSearchFilesTool(nil).Execute(context.Background(), json.RawMessage(`{"purpose":"Search the project for a marker. Search the default scope with the configured helper. The result should identify missing helper setup.","pattern":"needle"}`))
	if !strings.Contains(searchResult, "rg is not installed") || !strings.Contains(searchResult, "Install") {
		t.Fatalf("search_files missing helper result = %q, want install guidance", searchResult)
	}
}

// TestLimitOutputLines verifies that helper output is capped without changing earlier lines.
func TestLimitOutputLines(t *testing.T) {
	result := limitOutputLines("one\ntwo\nthree", 2)
	want := "one\ntwo\n[output truncated after 2 lines]"
	if result != want {
		t.Fatalf("limitOutputLines() = %q, want %q", result, want)
	}
}
