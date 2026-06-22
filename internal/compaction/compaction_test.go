// compaction_test.go — tests for the compaction package.
package compaction

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blazeai/internal/config"
	"blazeai/internal/provider"
	"blazeai/internal/session"
)

// setupManager creates a compaction Manager with mock config and optional mock provider.
func setupManager(t *testing.T, handler http.HandlerFunc) (*Manager, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	cfg := &config.Config{
		Providers: []config.Provider{{Name: "test", Endpoint: server.URL, APIKey: "sk-test"}},
		Roles:     config.Roles{Default: "test/test-model"},
		Compaction: config.Compaction{
			MaxContextTokens:       100,
			MinContextTokens:       50,
			SummaryMaxTokens:       2000,
			MaxSummaryFiles:        3,
			TokenCoefficient:       3.5,
			MaxBackoffOffsetTokens: 25,
		},
		StripReasoning: config.StripReasoning{Enable: true, PreserveLast: 5},
	}
	client := provider.NewClientRaw(server.URL, "sk-test")
	return NewManager(cfg, client), server
}

// makeSession creates a session in a temp dir with the given messages.
func makeSession(t *testing.T, msgs []session.Message) *session.Session {
	t.Helper()
	dir := t.TempDir()
	sess, err := session.CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() error: %v", err)
	}
	sess.Messages = msgs
	if err := sess.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	return sess
}

// TestShouldCompactTrue verifies trigger when tokens exceed threshold.
func TestShouldCompactTrue(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()
	usage := &provider.Usage{PromptTokens: 150}
	if !m.ShouldCompact(usage) {
		t.Error("ShouldCompact() = false, want true (150 >= 100)")
	}
}

// TestShouldCompactFalse verifies no trigger when below threshold.
func TestShouldCompactFalse(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()
	usage := &provider.Usage{PromptTokens: 50}
	if m.ShouldCompact(usage) {
		t.Error("ShouldCompact() = true, want false (50 < 100)")
	}
}

// TestShouldCompactNilUsage verifies no trigger when usage is nil.
func TestShouldCompactNilUsage(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()
	if m.ShouldCompact(nil) {
		t.Error("ShouldCompact(nil) = true, want false")
	}
}

// TestFindCutPoint verifies cut point selection based on token estimation.
func TestFindCutPoint(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	// Create messages with enough text to exceed minTokens.
	msgs := make([]session.Message, 10)
	for i := range msgs {
		msgs[i] = session.Message{Role: "user", Content: fmt.Sprintf("message number %d with some text", i)}
	}
	cut := m.findCutPoint(msgs, 50)
	if cut <= 0 {
		t.Errorf("cut = %d, want > 0", cut)
	}
	if cut >= len(msgs) {
		t.Errorf("cut = %d, want < %d", cut, len(msgs))
	}
}

// TestFindCutPointToolBoundary verifies tool call/result pairs are not split.
func TestFindCutPointToolBoundary(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	msgs := []session.Message{
		{Role: "user", Content: "do something"},
		{Role: "assistant", Content: "ok", ToolCalls: []map[string]interface{}{{"id": "call_1", "function": map[string]string{"name": "shell"}}}},
		{Role: "tool", Content: "result", ToolCallID: "call_1", Name: "shell"},
		{Role: "assistant", Content: "done"},
	}
	cut := m.findCutPoint(msgs, 10)
	// The cut should not be at index 2 (between tool call and tool result).
	if cut == 2 {
		t.Error("cut at index 2 splits tool call from result")
	}
}

// TestCompactSummarizeAndPrune verifies the full compaction flow with a mock summarizer.
func TestCompactSummarizeAndPrune(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Summary of conversation."}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "data: [DONE]")
		fmt.Fprintln(w)
	})
	defer server.Close()

	msgs := make([]session.Message, 5)
	for i := range msgs {
		msgs[i] = session.Message{Role: "user", Content: strings.Repeat("text", 20)}
	}
	sess := makeSession(t, msgs)
	originalCount := len(sess.Messages)

	usage := &provider.Usage{PromptTokens: 150}
	compacted, err := m.Compact(sess, usage)
	if err != nil {
		t.Fatalf("Compact() error: %v", err)
	}
	if !compacted {
		t.Fatal("Compact() = false, want true")
	}
	if len(sess.Messages) >= originalCount {
		t.Errorf("messages = %d, want < %d (pruned)", len(sess.Messages), originalCount)
	}

	// Verify summary file was created.
	summaryDir := filepath.Join(sess.Folder, "summaries")
	entries, _ := os.ReadDir(summaryDir)
	if len(entries) == 0 {
		t.Error("no summary files created")
	}
}

// TestCompactSkipBelowThreshold verifies no compaction when below threshold.
func TestCompactSkipBelowThreshold(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	msgs := []session.Message{{Role: "user", Content: "hi"}}
	sess := makeSession(t, msgs)
	originalCount := len(sess.Messages)

	usage := &provider.Usage{PromptTokens: 50}
	compacted, err := m.Compact(sess, usage)
	if err != nil {
		t.Fatalf("Compact() error: %v", err)
	}
	if compacted {
		t.Error("Compact() = true, want false (below threshold)")
	}
	if len(sess.Messages) != originalCount {
		t.Errorf("messages = %d, want %d (unchanged)", len(sess.Messages), originalCount)
	}
}

// TestCompactNilUsage verifies no compaction when usage is nil.
func TestCompactNilUsage(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	msgs := []session.Message{{Role: "user", Content: "hi"}}
	sess := makeSession(t, msgs)

	compacted, err := m.Compact(sess, nil)
	if err != nil {
		t.Fatalf("Compact() error: %v", err)
	}
	if compacted {
		t.Error("Compact() = true, want false (nil usage)")
	}
}

// TestCompactSummarizationFailure verifies skip when summarization fails below hard cap.
func TestCompactSummarizationFailure(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer server.Close()

	msgs := make([]session.Message, 5)
	for i := range msgs {
		msgs[i] = session.Message{Role: "user", Content: strings.Repeat("text", 20)}
	}
	sess := makeSession(t, msgs)
	originalCount := len(sess.Messages)

	// 110 is above threshold (100) but below hard cap (125).
	usage := &provider.Usage{PromptTokens: 110}
	compacted, err := m.Compact(sess, usage)
	if err != nil {
		t.Fatalf("Compact() error: %v", err)
	}
	if compacted {
		t.Error("Compact() = true, want false (summarization failed, below hard cap)")
	}
	if len(sess.Messages) != originalCount {
		t.Errorf("messages = %d, want %d (unchanged)", len(sess.Messages), originalCount)
	}
}

// TestCompactForcePruneAboveHardCap verifies forced prune without summary above hard cap.
func TestCompactForcePruneAboveHardCap(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer server.Close()

	msgs := make([]session.Message, 5)
	for i := range msgs {
		msgs[i] = session.Message{Role: "user", Content: strings.Repeat("text", 20)}
	}
	sess := makeSession(t, msgs)
	originalCount := len(sess.Messages)

	// 200 is above hard cap (125).
	usage := &provider.Usage{PromptTokens: 200}
	compacted, err := m.Compact(sess, usage)
	if err != nil {
		t.Fatalf("Compact() error: %v", err)
	}
	if !compacted {
		t.Error("Compact() = false, want true (forced prune above hard cap)")
	}
	if len(sess.Messages) >= originalCount {
		t.Errorf("messages = %d, want < %d (pruned)", len(sess.Messages), originalCount)
	}
}

// TestSaveAndLoadSummaries verifies summary storage and retrieval.
func TestSaveAndLoadSummaries(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	dir := t.TempDir()
	if err := m.saveSummary(dir, "first summary"); err != nil {
		t.Fatalf("saveSummary() error: %v", err)
	}
	if err := m.saveSummary(dir, "second summary"); err != nil {
		t.Fatalf("saveSummary() error: %v", err)
	}
	loaded := m.loadSummaries(dir)
	if !strings.Contains(loaded, "first summary") {
		t.Error("loaded summaries missing 'first summary'")
	}
	if !strings.Contains(loaded, "second summary") {
		t.Error("loaded summaries missing 'second summary'")
	}
}

// TestTrimSummaries verifies old summaries are deleted beyond maxSummaryFiles.
func TestTrimSummaries(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		m.saveSummary(dir, fmt.Sprintf("summary %d", i))
	}
	if err := m.trimSummaries(dir); err != nil {
		t.Fatalf("trimSummaries() error: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "summaries"))
	if len(entries) != 3 {
		t.Errorf("summaries = %d, want 3 (maxSummaryFiles)", len(entries))
	}
}

// TestLoadSummariesForResume verifies synthetic message creation on resume.
func TestLoadSummariesForResume(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	dir := t.TempDir()
	m.saveSummary(dir, "summary text")
	msg := m.LoadSummariesForResume(dir)
	if msg == nil {
		t.Fatal("LoadSummariesForResume() = nil, want message")
	}
	content, ok := msg.Content.(string)
	if !ok {
		t.Fatal("content is not a string")
	}
	if !strings.Contains(content, "summary text") {
		t.Errorf("content missing summary: %q", content)
	}
}

// TestLoadSummariesForResumeEmpty verifies nil when no summaries exist.
func TestLoadSummariesForResumeEmpty(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	dir := t.TempDir()
	msg := m.LoadSummariesForResume(dir)
	if msg != nil {
		t.Error("LoadSummariesForResume() = non-nil, want nil (no summaries)")
	}
}

// TestBuildTranscript verifies transcript construction from messages.
func TestBuildTranscript(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	msgs := []session.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "tool", Content: "result text", Name: "shell", ToolCallID: "call_1"},
	}
	transcript := m.buildTranscript(msgs)
	if !strings.Contains(transcript, "hello") {
		t.Error("transcript missing 'hello'")
	}
	if !strings.Contains(transcript, "hi there") {
		t.Error("transcript missing 'hi there'")
	}
	if !strings.Contains(transcript, "TOOL_RESULT") {
		t.Error("transcript missing tool result")
	}
}

// TestStripReasoningDisabled verifies no-op when stripping is disabled.
func TestStripReasoningDisabled(t *testing.T) {
	cfg := &config.Config{
		StripReasoning: config.StripReasoning{Enable: false, PreserveLast: 5},
	}
	m := NewManager(cfg, nil)
	msgs := []session.Message{{Role: "user", Content: "test"}}
	result := m.StripReasoningFromPayload(msgs)
	if len(result) != 1 || result[0].Content != "test" {
		t.Error("StripReasoningFromPayload modified messages when disabled")
	}
}
