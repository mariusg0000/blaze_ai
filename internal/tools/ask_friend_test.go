// ask_friend_test.go — tests for the ask_a_friend tool.
package tools

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
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

// TestAskFriendExecuteAllowsLongOptionalFields verifies that local caps were removed.
func TestAskFriendExecuteAllowsLongOptionalFields(t *testing.T) {
	tool := NewAskFriendTool(func(ctx context.Context, args AskFriendArgs) (string, error) {
		if len(args.Purpose) != 2000 {
			t.Fatalf("purpose length = %d, want 2000", len(args.Purpose))
		}
		if len(args.Question) != 10000 {
			t.Fatalf("question length = %d, want 10000", len(args.Question))
		}
		if len(args.OutputFormat) != 5000 {
			t.Fatalf("output_format length = %d, want 5000", len(args.OutputFormat))
		}
		return strings.Repeat("R", 20000), nil
	})
	result := tool.Execute(context.Background(), []byte(`{"role":"advisor","purpose":"`+strings.Repeat("p", 2000)+`","question":"`+strings.Repeat("q", 10000)+`","context":"Current runtime wiring.","output_format":"`+strings.Repeat("o", 5000)+`"}`))
	if result != strings.Repeat("R", 20000) {
		t.Fatalf("Execute() returned unexpected result length %d", len(result))
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
	if result != "error: input_file exceeds "+strconv.Itoa(maxAskFriendInputFileBytes)+" bytes: "+path {
		t.Fatalf("Execute() = %q", result)
	}
}

// TestAskFriendExecuteInputFileAtLimit verifies the raised file size limit is accepted.
func TestAskFriendExecuteInputFileAtLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")
	data := make([]byte, maxAskFriendInputFileBytes)
	for i := range data {
		data[i] = 'b'
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	tool := NewAskFriendTool(func(ctx context.Context, args AskFriendArgs) (string, error) {
		if !strings.Contains(args.Context, "size_bytes: "+strconv.Itoa(len(data))) {
			t.Fatalf("context missing exact size marker")
		}
		return "Accepted", nil
	})
	result := tool.Execute(context.Background(), []byte(`{"role":"summarization","purpose":"summarize file","question":"What matters?","context":"Base context.","input_file":"`+path+`","output_format":"bullet list"}`))
	if result != "Accepted" {
		t.Fatalf("Execute() = %q, want %q", result, "Accepted")
	}
}

// TestAskFriendExecuteContextTooLarge verifies the raised context limit is enforced.
func TestAskFriendExecuteContextTooLarge(t *testing.T) {
	tool := NewAskFriendTool(nil)
	contextText := strings.Repeat("c", maxAskFriendContextChars+1)
	result := tool.Execute(context.Background(), []byte(`{"role":"advisor","purpose":"review architecture","question":"What is the main risk?","context":"`+contextText+`","output_format":"markdown findings"}`))
	if result != "error: context exceeds "+strconv.Itoa(maxAskFriendContextChars)+" characters" {
		t.Fatalf("Execute() = %q", result)
	}
}

// TestAskFriendExecuteRejectsImageInput verifies images are routed to analyze_image instead.
func TestAskFriendExecuteRejectsImageInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "screen.png")
	img := image.NewRGBA(image.Rect(0, 0, 20, 10))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		t.Fatalf("png.Encode() error: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	tool := NewAskFriendTool(func(ctx context.Context, args AskFriendArgs) (string, error) {
		return "", nil
	})
	result := tool.Execute(context.Background(), []byte(`{"role":"summarization","purpose":"summarize file","question":"What matters?","context":"Base context.","input_file":"`+path+`","output_format":"bullet list"}`))
	if result != "error: input_file is an image; use analyze_image: "+path {
		t.Fatalf("Execute() = %q", result)
	}
}
