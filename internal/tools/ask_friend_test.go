// ask_friend_test.go — tests for the ask_a_friend tool.
package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAskFriendExecuteSuccess verifies a successful delegated answer.
func TestAskFriendExecuteSuccess(t *testing.T) {
	tool := NewAskFriendTool(func(ctx context.Context, args AskFriendArgs) (string, error) {
		if args.Role != "advisor" {
			t.Fatalf("role = %q, want advisor", args.Role)
		}
		if args.Context != "Current runtime wiring." {
			t.Fatalf("context = %q", args.Context)
		}
		return "Findings and recommendation", nil
	})
	result := tool.Execute(context.Background(), []byte(`{"role":"advisor","purpose":"review architecture","question":"What is the main risk?","context":"Current runtime wiring.","output_format":"markdown findings"}`))
	if result != "Findings and recommendation" {
		t.Fatalf("Execute() = %q, want %q", result, "Findings and recommendation")
	}
}

// TestAskFriendExecuteWithInputFile verifies that input_file content is attached to context.
func TestAskFriendExecuteWithInputFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("Important details\nLine two"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	tool := NewAskFriendTool(func(ctx context.Context, args AskFriendArgs) (string, error) {
		if !strings.Contains(args.Context, "[INPUT FILE]") {
			t.Fatalf("context missing input file header: %q", args.Context)
		}
		if !strings.Contains(args.Context, "path: "+path) {
			t.Fatalf("context missing file path: %q", args.Context)
		}
		if !strings.Contains(args.Context, "Important details") {
			t.Fatalf("context missing file content: %q", args.Context)
		}
		return "Summarized", nil
	})
	result := tool.Execute(context.Background(), []byte(`{"role":"summarization","purpose":"summarize file","question":"What matters?","context":"Base context.","input_file":"`+path+`","output_format":"bullet list"}`))
	if result != "Summarized" {
		t.Fatalf("Execute() = %q, want %q", result, "Summarized")
	}
}

// TestAskFriendExecuteInvalidRole verifies strict role validation.
func TestAskFriendExecuteInvalidRole(t *testing.T) {
	tool := NewAskFriendTool(nil)
	result := tool.Execute(context.Background(), []byte(`{"role":"friend","purpose":"review architecture","question":"What is the main risk?","context":"Current runtime wiring.","output_format":"markdown findings"}`))
	if result != "error: role must be one of advisor, summarization, or vision" {
		t.Fatalf("Execute() = %q", result)
	}
}

// TestAskFriendExecuteMissingContext verifies required evidence is enforced.
func TestAskFriendExecuteMissingContext(t *testing.T) {
	tool := NewAskFriendTool(nil)
	result := tool.Execute(context.Background(), []byte(`{"role":"advisor","purpose":"review architecture","question":"What is the main risk?","context":"","output_format":"markdown findings"}`))
	if result != "error: context is required" {
		t.Fatalf("Execute() = %q", result)
	}
}

// TestAskFriendExecuteInputFileMissing verifies strict missing-file failure.
func TestAskFriendExecuteInputFileMissing(t *testing.T) {
	tool := NewAskFriendTool(func(ctx context.Context, args AskFriendArgs) (string, error) {
		return "", nil
	})
	result := tool.Execute(context.Background(), []byte(`{"role":"summarization","purpose":"summarize file","question":"What matters?","context":"Base context.","input_file":"/tmp/does-not-exist-opencode","output_format":"bullet list"}`))
	if !strings.Contains(result, "error: cannot stat input_file /tmp/does-not-exist-opencode") {
		t.Fatalf("Execute() = %q", result)
	}
}

// TestAskFriendExecuteInputFileTooLarge verifies strict size enforcement.
func TestAskFriendExecuteInputFileTooLarge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")
	data := make([]byte, maxAskFriendInputFileBytes+1)
	for i := range data {
		data[i] = 'a'
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	tool := NewAskFriendTool(func(ctx context.Context, args AskFriendArgs) (string, error) {
		return "", nil
	})
	result := tool.Execute(context.Background(), []byte(`{"role":"summarization","purpose":"summarize file","question":"What matters?","context":"Base context.","input_file":"`+path+`","output_format":"bullet list"}`))
	if result != "error: input_file exceeds 150000 bytes: "+path {
		t.Fatalf("Execute() = %q", result)
	}
}
