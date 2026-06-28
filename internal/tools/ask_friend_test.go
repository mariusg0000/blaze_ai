// ask_friend_test.go — tests for the ask_a_friend tool.
package tools

import (
	"context"
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
