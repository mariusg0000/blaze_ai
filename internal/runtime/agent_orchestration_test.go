// agent_orchestration_test.go — tests for persistent child-agent result handling.
// Purpose: Verify fallback completion and resume metadata formatting.
// Layer: runtime orchestration tests. Direct dependency: internal/session.
package runtime

import (
	"strings"
	"testing"

	"blazeai/internal/session"
)

// TestLastAssistantAnswerUsesFinalPlainText verifies a final assistant message
// can complete a child when agent_done was not called.
// WHAT: Extracts the last plain assistant answer.
// HOW: Ignores assistant messages that contain tool calls.
func TestLastAssistantAnswerUsesFinalPlainText(t *testing.T) {
	child := &session.Session{Messages: []session.Message{
		{Role: "assistant", Content: "intermediate", ToolCalls: []interface{}{map[string]any{"id": "call-1"}}},
		{Role: "tool", Content: "tool result"},
		{Role: "assistant", Content: "Implementation finished and syntax checks passed."},
	}}

	got := lastAssistantAnswer(child)
	want := "Implementation finished and syntax checks passed."
	if got != want {
		t.Fatalf("lastAssistantAnswer() = %q, want %q", got, want)
	}
}

// TestLastAssistantAnswerRejectsToolOnlyTail verifies tool-call-only sessions
// remain incomplete instead of being reported as successful.
// WHAT: Rejects a final assistant message that has no text.
// HOW: Returns an empty answer for tool-call messages.
func TestLastAssistantAnswerRejectsToolOnlyTail(t *testing.T) {
	child := &session.Session{Messages: []session.Message{
		{Role: "assistant", Content: "", ToolCalls: []interface{}{map[string]any{"id": "call-1"}}},
	}}
	if got := lastAssistantAnswer(child); got != "" {
		t.Fatalf("lastAssistantAnswer() = %q, want empty", got)
	}
}

// TestFormatChildResultIncludesResumeMetadata verifies every child result
// exposes the agent identity and optional resume information.
// WHAT: Formats child identity before the answer.
// HOW: Includes the exact session ID and neutral resume wording.
func TestFormatChildResultIncludesResumeMetadata(t *testing.T) {
	got := formatChildResult("coder", "d2a1c3", "", "finished")
	for _, want := range []string{
		"agent: coder",
		"child session id: d2a1c3",
		"can be resumed later",
		"same agent name, this id, and a new task",
		"finished",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("formatChildResult() missing %q in %q", want, got)
		}
	}
}
