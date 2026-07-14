// agent_tools_test.go — tests for run_agent and agent_done protocol tools.
// Verifies strict arguments, non-empty completion, and callback delegation.
// Layer: tool execution. Dependencies: standard library.
package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAgentDoneRequiresNonEmptyAnswer(t *testing.T) {
	var got string
	tool := NewAgentDoneTool(func(answer string) { got = answer })
	if result := tool.Execute(context.Background(), json.RawMessage(`{"answer":"  "}`)); !strings.Contains(result, "non-empty") {
		t.Fatalf("empty result = %q", result)
	}
	if result := tool.Execute(context.Background(), json.RawMessage(`{"answer":"done"}`)); result != "completed" {
		t.Fatalf("valid result = %q", result)
	}
	if got != "done" {
		t.Fatalf("callback answer = %q", got)
	}
}

func TestRunAgentValidatesTasks(t *testing.T) {
	tool := NewRunAgentTool(func(context.Context, RunAgentArgs) string { return "ok" })
	if result := tool.Execute(context.Background(), json.RawMessage(`{}`)); !strings.Contains(result, "agent and task") {
		t.Fatalf("missing args result = %q", result)
	}
	if result := tool.Execute(context.Background(), json.RawMessage(`{"purpose":"Run the review agent. Inspect the requested files. Return findings.","agent":"review","task":"check"}`)); result != "ok" {
		t.Fatalf("valid result = %q", result)
	}
}

func TestRunAgentFormatArgsPurposeAndFallback(t *testing.T) {
	tool := NewRunAgentTool(nil)
	purpose := "Run explore. Inspect the target directory. Return a complete summary."
	if got := tool.FormatArgs(json.RawMessage(`{"purpose":"` + purpose + `","task":"ignored"}`)); got != purpose {
		t.Fatalf("purpose display = %q", got)
	}
	longTask := strings.Repeat("x", 200)
	got := tool.FormatArgs(json.RawMessage(`{"task":"` + longTask + `"}`))
	if len([]rune(got)) != 150 || !strings.HasSuffix(got, "...") {
		t.Fatalf("fallback display length/content = %d %q", len([]rune(got)), got)
	}
}
