// agent_orchestration_test.go — tests for persistent child-agent result handling.
// Purpose: Verify fallback completion, resume metadata formatting, and childHandler CTX propagation.
// Layer: runtime orchestration tests. Direct dependency: internal/session.
package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blazeai/internal/session"
	"blazeai/internal/tools"
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

// TestFormatChildErrorIncludesResumeMetadata verifies failed children expose the exact ID.
// WHAT: Ensures timeout-style errors contain valid resume arguments.
// HOW: Formats a representative failure and checks the agent name is not used as the ID.
func TestFormatChildErrorIncludesResumeMetadata(t *testing.T) {
	got := formatChildError("coder", "e83f12", fmt.Errorf("child timed out due to inactivity")).Error()
	for _, want := range []string{
		"agent: coder",
		"child session id: e83f12",
		"child timed out due to inactivity",
		`Resume with agent="coder" and id="e83f12"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("formatChildError() missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, `id="coder"`) {
		t.Fatalf("formatChildError() used agent name as ID: %q", got)
	}
}

// TestChildHandlerOnUsageStoresPromptTokens verifies that childHandler tracks
// the latest prompt token count from OnUsage for CTX propagation.
// WHAT: OnUsage must update internal state that OnToolResult reads.
// HOW: Calls OnUsage then OnToolResult and checks LastPromptTokens in emitted activity.
func TestChildHandlerOnUsageStoresPromptTokens(t *testing.T) {
	var emitted []AgentActivity
	h := &childHandler{
		agentID: "[test_abc]",
		emit: func(a AgentActivity) {
			emitted = append(emitted, a)
		},
	}
	h.OnUsage(15000, 3000, 12000)
	h.OnToolResult("shell", "ok")

	if len(emitted) != 1 {
		t.Fatalf("expected 1 emitted activity, got %d", len(emitted))
	}
	if emitted[0].LastPromptTokens != 15000 {
		t.Errorf("LastPromptTokens = %d, want 15000", emitted[0].LastPromptTokens)
	}
}

// TestChildHandlerToolResultWithoutUsageShowsZeroTokens verifies that when no
// OnUsage was called, the tool_result activity carries zero tokens.
// WHAT: OnToolResult must carry LastPromptTokens=0 by default.
// HOW: Calls only OnToolResult without preceding OnUsage.
func TestChildHandlerToolResultWithoutUsageShowsZeroTokens(t *testing.T) {
	var emitted []AgentActivity
	h := &childHandler{
		agentID: "[test_abc]",
		emit: func(a AgentActivity) {
			emitted = append(emitted, a)
		},
	}
	h.OnToolResult("shell", "ok")

	if len(emitted) != 1 {
		t.Fatalf("expected 1 emitted activity, got %d", len(emitted))
	}
	if emitted[0].LastPromptTokens != 0 {
		t.Errorf("LastPromptTokens = %d, want 0", emitted[0].LastPromptTokens)
	}
}

// TestChildHandlerOnUsageUpdatesAcrossCalls verifies that OnUsage tracks the
// most recent token count, not the first.
// WHAT: Each OnUsage call must overwrite the previous token count.
// HOW: Calls OnUsage twice with different values, then checks the final activity.
func TestChildHandlerOnUsageUpdatesAcrossCalls(t *testing.T) {
	var emitted []AgentActivity
	h := &childHandler{
		agentID: "[test_abc]",
		emit: func(a AgentActivity) {
			emitted = append(emitted, a)
		},
	}
	h.OnUsage(10000, 0, 10000)
	h.OnUsage(20000, 5000, 15000)
	h.OnToolResult("shell", "ok")

	if len(emitted) != 1 {
		t.Fatalf("expected 1 emitted activity, got %d", len(emitted))
	}
	if emitted[0].LastPromptTokens != 20000 {
		t.Errorf("LastPromptTokens = %d, want 20000", emitted[0].LastPromptTokens)
	}
}

// TestChildHandlerToolCallDoesNotCarryTokens verifies that tool_call activities
// never include prompt token data (CTX belongs only on result lines).
// WHAT: tool_call Kind must have zero LastPromptTokens.
// HOW: Emits a tool_call and verifies the field is zero.
func TestChildHandlerToolCallDoesNotCarryTokens(t *testing.T) {
	var emitted []AgentActivity
	h := &childHandler{
		agentID: "[test_abc]",
		emit: func(a AgentActivity) {
			emitted = append(emitted, a)
		},
	}
	h.OnUsage(15000, 0, 15000)
	h.OnToolCall("shell", "ls -la")

	if len(emitted) != 1 {
		t.Fatalf("expected 1 emitted activity, got %d", len(emitted))
	}
	if emitted[0].LastPromptTokens != 0 {
		t.Errorf("tool_call LastPromptTokens = %d, want 0", emitted[0].LastPromptTokens)
	}
}

// TestOpenChildSessionMarksExistingSessionAsResumed verifies that an existing
// child session is distinguished from a newly created child session.
// WHAT: Detects resume state without changing the persisted session.
// HOW: Creates a child once, then opens the same ID again.
func TestOpenChildSessionMarksExistingSessionAsResumed(t *testing.T) {
	mainFolder := t.TempDir()
	folder, _, resumed, err := openChildSession(mainFolder, "child1")
	if err != nil {
		t.Fatalf("first openChildSession() error: %v", err)
	}
	if resumed {
		t.Fatal("new child session was marked resumed")
	}
	if err := os.WriteFile(filepath.Join(folder, "agent_task.md"), []byte("original task\n"), 0644); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	resumedFolder, _, resumed, err := openChildSession(mainFolder, "child1")
	if err != nil {
		t.Fatalf("resume openChildSession() error: %v", err)
	}
	if !resumed {
		t.Fatal("existing child session was not marked resumed")
	}
	if resumedFolder != folder {
		t.Fatalf("resumed folder = %q, want %q", resumedFolder, folder)
	}
	content, err := os.ReadFile(filepath.Join(folder, "agent_task.md"))
	if err != nil {
		t.Fatalf("read task file: %v", err)
	}
	if string(content) != "original task\n" {
		t.Fatalf("original task file changed: %q", content)
	}
}

// TestBuildChildInputUsesResumeMessage verifies that resumed work is sent as
// a distinct user-facing input instead of being written into the task file.
// WHAT: Includes the new task and optional context in the resume message.
// HOW: Builds resumed input and checks its explicit protocol markers.
func TestBuildChildInputUsesResumeMessage(t *testing.T) {
	got := buildChildInput(tools.RunAgentTask{Task: "new task", Context: "extra context"}, true)
	for _, want := range []string{"[RESUME TASK]", "new task", "[CONTEXT]", "extra context"} {
		if !strings.Contains(got, want) {
			t.Errorf("buildChildInput() missing %q in %q", want, got)
		}
	}
}
