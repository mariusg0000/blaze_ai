// taskswitch_test.go — tests for task-switch detection transcript building and response parsing.
package compaction

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blazeai/internal/config"
	"blazeai/internal/session"
)

func TestDetectionSystemPromptContract(t *testing.T) {
	prompt := detectionSystemPrompt()
	for _, fragment := range []string{
		"classification and memory-boundary task",
		"[user N]",
		"null",
		`{"index":"user N","summary":"<concise technical summary of all messages before this user message>"}`,
		"Do not consider it a switch when:",
		"Do not summarize the new task",
		"Respond ONLY with null or the JSON object.",
	} {
		if !strings.Contains(prompt, fragment) {
			t.Errorf("task-switch prompt missing %q", fragment)
		}
	}
}

// TestBuildDetectionTranscriptBasic verifies the compact transcript format.
func TestBuildDetectionTranscriptBasic(t *testing.T) {
	sess := &session.Session{
		Messages: []session.Message{
			{Role: "user", Content: "first message"},
			{Role: "assistant", Content: "first reply"},
			{Role: "user", Content: "second message"},
			{Role: "assistant", Content: "second reply", ToolCalls: []interface{}{
				map[string]interface{}{"id": "call_1", "function": map[string]interface{}{"name": "shell", "arguments": `{"command":"echo hi"}`}},
			}},
			{Role: "tool", Content: "exit_code: 0\nstdout: hi\n", Name: "shell", ToolCallID: "call_1"},
		},
	}

	transcript := buildDetectionTranscript(sess, "")

	// Must contain user messages with 0-based indices.
	if !strings.Contains(transcript, "[user 0] first message") {
		t.Error("missing [user 0]")
	}
	if !strings.Contains(transcript, "[user 1] second message") {
		t.Error("missing [user 1]")
	}
	// Must contain assistant content.
	if !strings.Contains(transcript, "[assistant] first reply") {
		t.Error("missing first assistant reply")
	}
	// Must contain tool call marker.
	if !strings.Contains(transcript, "[tool call]") {
		t.Error("missing [tool call]")
	}
	// Must contain tool result marker.
	if !strings.Contains(transcript, "[tool result: shell]") {
		t.Error("missing [tool result]")
	}
	// Must NOT contain reasoning.
	if strings.Contains(transcript, "[REASONING]") {
		t.Error("transcript contains reasoning (should be stripped)")
	}
	// Must NOT contain [system].
	if strings.Contains(transcript, "[system]") {
		t.Error("transcript contains system message")
	}
}

// TestBuildDetectionTranscriptTruncation verifies tool calls and results are truncated.
func TestBuildDetectionTranscriptTruncation(t *testing.T) {
	longArgs := strings.Repeat("x", 300)
	sess := &session.Session{
		Messages: []session.Message{
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "ok", ToolCalls: []interface{}{
				map[string]interface{}{"id": "c1", "function": map[string]interface{}{"name": "shell", "arguments": longArgs}},
			}},
			{Role: "tool", Content: strings.Repeat("y", 300), Name: "shell", ToolCallID: "c1"},
		},
	}

	transcript := buildDetectionTranscript(sess, "")

	// Tool call should be truncated (serialized JSON of tool calls + truncation).
	if strings.Count(transcript, "xxx") > truncatedLen/3 {
		t.Error("tool call arguments not truncated")
	}
	// Tool result should be truncated.
	if strings.Count(transcript, "yyy") > truncatedLen/3 {
		t.Error("tool result not truncated")
	}
}

// TestBuildDetectionTranscriptWithSummaries verifies existing summaries are included.
func TestBuildDetectionTranscriptWithSummaries(t *testing.T) {
	sess := &session.Session{
		Messages: []session.Message{
			{Role: "user", Content: "hello"},
		},
	}

	transcript := buildDetectionTranscript(sess, "prior summary text")
	if !strings.Contains(transcript, "[summary]\nprior summary text") {
		t.Error("missing existing summaries in transcript")
	}
}

// TestParseDetectionResponseNull verifies "null" response is parsed as no change.
func TestParseDetectionResponseNull(t *testing.T) {
	result := parseDetectionResponse("null")
	if result.Changed {
		t.Error("null should parse as Changed=false")
	}
}

// TestParseDetectionResponseEmpty verifies empty response is no change.
func TestParseDetectionResponseEmpty(t *testing.T) {
	result := parseDetectionResponse("")
	if result.Changed {
		t.Error("empty string should parse as Changed=false")
	}
}

// TestParseDetectionResponseWhitespaceNull verifies "  null  " is no change.
func TestParseDetectionResponseWhitespaceNull(t *testing.T) {
	result := parseDetectionResponse("  null  ")
	if result.Changed {
		t.Error("whitespace-surrounded null should parse as Changed=false")
	}
}

// TestParseDetectionResponseValid verifies valid JSON with integer index.
func TestParseDetectionResponseValid(t *testing.T) {
	resp := `{"index": 3, "summary": "User debugged authentication flow."}`
	result := parseDetectionResponse(resp)
	if !result.Changed {
		t.Fatal("valid JSON should parse as Changed=true")
	}
	if result.Index != 3 {
		t.Errorf("Index = %d, want 3", result.Index)
	}
	if result.Summary != "User debugged authentication flow." {
		t.Errorf("Summary = %q, want 'User debugged authentication flow.'", result.Summary)
	}
}

// TestParseDetectionResponseStringIndex verifies string "5" is parsed as 5.
func TestParseDetectionResponseStringIndex(t *testing.T) {
	resp := `{"index": "5", "summary": "text"}`
	result := parseDetectionResponse(resp)
	if !result.Changed {
		t.Fatal("string index should parse as Changed=true")
	}
	if result.Index != 5 {
		t.Errorf("Index = %d, want 5", result.Index)
	}
}

// TestParseDetectionResponseLabelIndex verifies "user 5" label is parsed correctly.
func TestParseDetectionResponseLabelIndex(t *testing.T) {
	resp := `{"index": "user 4", "summary": "text"}`
	result := parseDetectionResponse(resp)
	if !result.Changed {
		t.Fatal("label index should parse as Changed=true")
	}
	if result.Index != 4 {
		t.Errorf("Index = %d, want 4", result.Index)
	}
}

// TestParseDetectionResponseLabelUnderscore verifies "user_04" extracts 4.
func TestParseDetectionResponseLabelUnderscore(t *testing.T) {
	resp := `{"index": "user_04", "summary": "text"}`
	result := parseDetectionResponse(resp)
	if !result.Changed {
		t.Fatal("label with underscore should parse as Changed=true")
	}
	if result.Index != 4 {
		t.Errorf("Index = %d, want 4", result.Index)
	}
}

// TestParseDetectionResponseNoDigits verifies unparseable string is no change.
func TestParseDetectionResponseNoDigits(t *testing.T) {
	resp := `{"index": "no digits here", "summary": "text"}`
	result := parseDetectionResponse(resp)
	if result.Changed {
		t.Error("unparseable string should parse as Changed=false")
	}
}

// TestParseDetectionResponseEmptyIndex verifies empty string index is no change.
func TestParseDetectionResponseEmptyIndex(t *testing.T) {
	resp := `{"index": "", "summary": "text"}`
	result := parseDetectionResponse(resp)
	if result.Changed {
		t.Error("empty string index should parse as Changed=false")
	}
}

// TestParseDetectionResponseMarkdownFence verifies markdown code fences are stripped.
func TestParseDetectionResponseMarkdownFence(t *testing.T) {
	resp := "```json\n{\"index\": 1, \"summary\": \"summary text.\"}\n```"
	result := parseDetectionResponse(resp)
	if !result.Changed {
		t.Fatal("markdown-fenced JSON should parse as Changed=true")
	}
	if result.Index != 1 {
		t.Errorf("Index = %d, want 1", result.Index)
	}
}

// TestParseDetectionResponseInvalidJSON verifies invalid JSON is no change.
func TestParseDetectionResponseInvalidJSON(t *testing.T) {
	result := parseDetectionResponse("not valid json at all")
	if result.Changed {
		t.Error("invalid JSON should parse as Changed=false")
	}
}

// TestParseDetectionResponseMissingIndex verifies missing index is no change.
func TestParseDetectionResponseMissingIndex(t *testing.T) {
	result := parseDetectionResponse(`{"summary": "some text"}`)
	if result.Changed {
		t.Error("missing index should parse as Changed=false")
	}
}

// TestParseDetectionResponseZeroIndex verifies index 0 is ignored (can't switch at first message).
func TestParseDetectionResponseZeroIndex(t *testing.T) {
	result := parseDetectionResponse(`{"index": 0, "summary": "some text"}`)
	if result.Changed {
		t.Error("index 0 should parse as Changed=false (no messages to prune)")
	}
}

// TestParseDetectionResponseEmptySummary verifies empty summary is no change.
func TestParseDetectionResponseEmptySummary(t *testing.T) {
	result := parseDetectionResponse(`{"index": 2, "summary": ""}`)
	if result.Changed {
		t.Error("empty summary should parse as Changed=false")
	}
}

// TestConsumeTaskSwitchResultPending verifies an empty marker file is treated as in-progress.
func TestConsumeTaskSwitchResultPending(t *testing.T) {
	m := NewManager(DefaultCompactionConfig(), nil, nil)
	dir := t.TempDir()
	sess := &session.Session{Folder: dir}
	if err := os.WriteFile(filepath.Join(dir, taskSwitchFile), nil, 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	result, err := m.ConsumeTaskSwitchResult(sess)
	if err != nil {
		t.Fatalf("ConsumeTaskSwitchResult() error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result for pending marker")
	}
}

// TestCancelTaskSwitchRemovesPendingProtocol verifies cancellation clears pending protocol files.
func TestCancelTaskSwitchRemovesPendingProtocol(t *testing.T) {
	m := NewManager(DefaultCompactionConfig(), nil, nil)
	dir := t.TempDir()
	for _, name := range []string{taskSwitchFile, taskSwitchTempFile} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("pending"), 0644); err != nil {
			t.Fatalf("WriteFile(%s) error: %v", name, err)
		}
	}

	if err := m.CancelTaskSwitch(dir); err != nil {
		t.Fatalf("CancelTaskSwitch() error: %v", err)
	}
	for _, name := range []string{taskSwitchFile, taskSwitchTempFile} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("pending file %s still exists, err=%v", name, err)
		}
	}
}

// TestConsumeTaskSwitchResultInvalidRemovesFile verifies invalid JSON is deleted with an explicit error.
func TestConsumeTaskSwitchResultInvalidRemovesFile(t *testing.T) {
	m := NewManager(DefaultCompactionConfig(), nil, nil)
	dir := t.TempDir()
	sess := &session.Session{Folder: dir}
	path := filepath.Join(dir, taskSwitchFile)
	if err := os.WriteFile(path, []byte("{bad json"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	_, err := m.ConsumeTaskSwitchResult(sess)
	if err == nil || !strings.Contains(err.Error(), "invalid taskswitch.json removed") {
		t.Fatalf("ConsumeTaskSwitchResult() error = %v, want invalid-file error", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("taskswitch.json still exists after invalid removal: %v", statErr)
	}
}

// TestCompactByTaskSwitch verifies messages are pruned and summary is saved.
func TestCompactByTaskSwitch(t *testing.T) {
	cfg := DefaultCompactionConfig()
	m := NewManager(cfg, nil, nil)

	dir := t.TempDir()
	sess := &session.Session{
		Messages: []session.Message{
			{Role: "system", Content: "sysprompt"},
			{Role: "user", Content: "old task message 1"}, // user 0
			{Role: "assistant", Content: "old task reply 1"},
			{Role: "user", Content: "old task message 2"}, // user 1
			{Role: "assistant", Content: "old task reply 2"},
			{Role: "user", Content: "new task message"}, // user 2 ← switch here
			{Role: "assistant", Content: "new task reply"},
		},
		Folder: dir,
	}

	// userIndex=2 means the 3rd user message (0-based), which is the session index used for slicing.
	err := m.CompactByTaskSwitch(sess, 2, "summary of old task")
	if err != nil {
		t.Fatalf("CompactByTaskSwitch() error: %v", err)
	}

	// Verify old messages are removed.
	if len(sess.Messages) < 3 {
		t.Fatalf("session has %d messages, want at least 3 (synthetic + kept)", len(sess.Messages))
	}

	// First message should be synthetic summary.
	firstContent, ok := sess.Messages[0].Content.(string)
	if !ok || !strings.Contains(firstContent, syntheticPrefix) {
		t.Errorf("first message is not synthetic summary: %v", sess.Messages[0].Content)
	}

	// Verify kept messages exist.
	foundNewTask := false
	for _, msg := range sess.Messages {
		if content, ok := msg.Content.(string); ok && content == "new task message" {
			foundNewTask = true
			break
		}
	}
	if !foundNewTask {
		t.Error("new task messages not preserved after switch")
	}

	// Summary file should exist.
	summaryDir := summariesDir(dir)
	entries, err := os.ReadDir(summaryDir)
	if err != nil || len(entries) == 0 {
		t.Error("no summary files created")
	}
}

// TestCompactByTaskSwitchNoop verifies empty summary or index 0 is no-op.
func TestCompactByTaskSwitchNoop(t *testing.T) {
	cfg := DefaultCompactionConfig()
	m := NewManager(cfg, nil, nil)

	dir := t.TempDir()
	sess := &session.Session{
		Messages: []session.Message{
			{Role: "user", Content: "hello"},
		},
		Folder: dir,
	}

	originalLen := len(sess.Messages)

	// Empty summary.
	err := m.CompactByTaskSwitch(sess, 1, "")
	if err != nil {
		t.Fatalf("CompactByTaskSwitch() error: %v", err)
	}
	if len(sess.Messages) != originalLen {
		t.Error("messages modified on empty summary")
	}

	// Index 0.
	err = m.CompactByTaskSwitch(sess, 0, "some summary")
	if err != nil {
		t.Fatalf("CompactByTaskSwitch() error: %v", err)
	}
	if len(sess.Messages) != originalLen {
		t.Error("messages modified on index 0")
	}
}

// TestUserIndexToSessionIndex verifies the conversion from user index to session index.
func TestUserIndexToSessionIndex(t *testing.T) {
	msgs := []session.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "u0"}, // session idx 1
		{Role: "assistant", Content: "a"},
		{Role: "user", Content: "u1"}, // session idx 3
		{Role: "assistant", Content: "b"},
		{Role: "user", Content: "u2"}, // session idx 5
	}

	if idx := userIndexToSessionIndex(msgs, 0); idx != 1 {
		t.Errorf("user 0 → session %d, want 1", idx)
	}
	if idx := userIndexToSessionIndex(msgs, 1); idx != 3 {
		t.Errorf("user 1 → session %d, want 3", idx)
	}
	if idx := userIndexToSessionIndex(msgs, 2); idx != 5 {
		t.Errorf("user 2 → session %d, want 5", idx)
	}
	if idx := userIndexToSessionIndex(msgs, 3); idx != -1 {
		t.Errorf("user 3 (out of range) → session %d, want -1", idx)
	}
	if idx := userIndexToSessionIndex(msgs, -1); idx != -1 {
		t.Errorf("negative user index → session %d, want -1", idx)
	}
}

// TestCompactByTaskSwitchBadSummary verifies summary file content is correct.
func TestCompactByTaskSwitchSummaryContent(t *testing.T) {
	cfg := DefaultCompactionConfig()
	m := NewManager(cfg, nil, nil)

	dir := t.TempDir()
	sess := &session.Session{
		Messages: []session.Message{
			{Role: "user", Content: "task A"},
			{Role: "assistant", Content: "reply A"},
			{Role: "user", Content: "task B"},
			{Role: "assistant", Content: "reply B"},
		},
		Folder: dir,
	}

	summaryText := "User worked on task A and completed it."
	err := m.CompactByTaskSwitch(sess, 1, summaryText)
	if err != nil {
		t.Fatalf("CompactByTaskSwitch() error: %v", err)
	}

	// Read summary file.
	summaryDir := summariesDir(dir)
	entries, err := os.ReadDir(summaryDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 summary file, got %v", entries)
	}
	data, err := os.ReadFile(filepath.Join(summaryDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("cannot read summary file: %v", err)
	}
	if string(data) != summaryText {
		t.Errorf("summary file = %q, want %q", string(data), summaryText)
	}

	// Synthetic message must contain the summary.
	synthContent, _ := sess.Messages[0].Content.(string)
	if !strings.Contains(synthContent, summaryText) {
		t.Error("synthetic message does not contain summary text")
	}
}

// DefaultCompactionConfig returns a minimal config for compaction tests.
func DefaultCompactionConfig() *config.Config {
	return &config.Config{
		Compaction:     config.DefaultCompaction(),
		StripReasoning: config.DefaultStripReasoning(),
	}
}
