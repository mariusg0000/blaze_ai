// compaction_test.go — tests for the compaction package.
package compaction

import (
	"fmt"
	"io"
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

// TestFindCutPointStrictTokens verifies cut point selection is token-based only.
func TestFindCutPointStrictTokens(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	msgs := []session.Message{
		{Role: "user", Content: "do something"},
		{Role: "assistant", Content: "ok", ToolCalls: []map[string]interface{}{{"id": "call_1", "function": map[string]string{"name": "shell"}}}},
		{Role: "tool", Content: strings.Repeat("result ", 20), ToolCallID: "call_1", Name: "shell"},
		{Role: "assistant", Content: "done"},
	}
	cut := m.findCutPoint(msgs, 10)
	if cut <= 0 || cut >= len(msgs) {
		t.Errorf("cut = %d, want within message range", cut)
	}
}

// TestCompactSanitizesRetainedTail verifies orphan tool messages caused by a raw token cut
// are removed from the retained tail and summarized with the pruned segment.
func TestCompactSanitizesRetainedTail(t *testing.T) {
	var requestBody string
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		requestBody = string(data)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Summary of conversation."}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "data: [DONE]")
		fmt.Fprintln(w)
	})
	defer server.Close()

	msgs := []session.Message{
		{Role: "user", Content: strings.Repeat("older ", 40)},
		{Role: "assistant", Content: "", ToolCalls: []interface{}{
			map[string]interface{}{"id": "call_1"},
			map[string]interface{}{"id": "call_2"},
		}},
		{Role: "tool", Content: strings.Repeat("tool-one ", 20), ToolCallID: "call_1", Name: "shell"},
		{Role: "tool", Content: strings.Repeat("tool-two ", 20), ToolCallID: "call_2", Name: "shell"},
		{Role: "assistant", Content: strings.Repeat("follow-up ", 20)},
	}
	sess := makeSession(t, msgs)

	compacted, err := m.Compact(sess, &provider.Usage{PromptTokens: 150})
	if err != nil {
		t.Fatalf("Compact() error: %v", err)
	}
	if !compacted {
		t.Fatal("Compact() = false, want true")
	}
	if !strings.Contains(requestBody, "tool-two") {
		t.Fatal("summarizer request did not include orphan tool result moved from retained tail")
	}
	for i, msg := range sess.Messages {
		if i == 0 {
			continue
		}
		if msg.Role == "tool" {
			t.Fatal("retained session still contains orphan tool message after compaction")
		}
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
	msgs := []session.Message{{Role: "user", Content: "test", Reasoning: "thinking"}}
	result := m.StripReasoningFromPayload(msgs)
	if len(result) != 1 || result[0].Reasoning != "thinking" {
		t.Error("StripReasoningFromPayload modified reasoning when disabled")
	}
}

// TestStripReasoningPreserveAll verifies all reasoning kept when count <= preserveLast.
func TestStripReasoningPreserveAll(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	msgs := []session.Message{
		{Role: "assistant", Content: "a1", Reasoning: "r1"},
		{Role: "assistant", Content: "a2", Reasoning: "r2"},
	}
	result := m.StripReasoningFromPayload(msgs)
	if result[0].Reasoning != "r1" || result[1].Reasoning != "r2" {
		t.Error("reasoning stripped when count <= preserveLast (5)")
	}
}

// TestStripReasoningStripsOlder verifies older reasoning is stripped beyond preserveLast.
func TestStripReasoningStripsOlder(t *testing.T) {
	cfg := &config.Config{
		Compaction:     config.DefaultCompaction(),
		StripReasoning: config.StripReasoning{Enable: true, PreserveLast: 2},
	}
	m := NewManager(cfg, nil)

	msgs := []session.Message{
		{Role: "assistant", Content: "a1", Reasoning: "r1"},
		{Role: "assistant", Content: "a2", Reasoning: "r2"},
		{Role: "assistant", Content: "a3", Reasoning: "r3"},
		{Role: "assistant", Content: "a4", Reasoning: "r4"},
	}
	result := m.StripReasoningFromPayload(msgs)
	// Newest 2 (r3, r4) kept, older 2 (r1, r2) stripped.
	if result[0].Reasoning != "" {
		t.Errorf("r1 not stripped, got %q", result[0].Reasoning)
	}
	if result[1].Reasoning != "" {
		t.Errorf("r2 not stripped, got %q", result[1].Reasoning)
	}
	if result[2].Reasoning != "r3" {
		t.Errorf("r3 stripped, got %q", result[2].Reasoning)
	}
	if result[3].Reasoning != "r4" {
		t.Errorf("r4 stripped, got %q", result[3].Reasoning)
	}
}

// TestStripReasoningNoReasoning verifies messages without reasoning are unchanged.
func TestStripReasoningNoReasoning(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	msgs := []session.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	result := m.StripReasoningFromPayload(msgs)
	if result[0].Content != "hello" || result[1].Content != "hi" {
		t.Error("non-reasoning messages modified")
	}
}

// TestBuildTranscriptWithReasoning verifies reasoning appears in transcript for newest N.
func TestBuildTranscriptWithReasoning(t *testing.T) {
	cfg := &config.Config{
		Compaction:     config.DefaultCompaction(),
		StripReasoning: config.StripReasoning{Enable: true, PreserveLast: 1},
	}
	m := NewManager(cfg, nil)

	msgs := []session.Message{
		{Role: "assistant", Content: "a1", Reasoning: "old reasoning"},
		{Role: "assistant", Content: "a2", Reasoning: "new reasoning"},
	}
	transcript := m.buildTranscript(msgs)
	if !strings.Contains(transcript, "new reasoning") {
		t.Error("transcript missing newest reasoning")
	}
	if strings.Contains(transcript, "old reasoning") {
		t.Error("transcript should not include older reasoning (preserveLast=1)")
	}
}

// TestEstimateTokensReasoningStripped verifies reasoning counts as 0 when stripped.
func TestEstimateTokensReasoningStripped(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	msg := session.Message{Content: "hello", Reasoning: "thinking about it"}
	tokensWith := m.estimateTokens(msg, false)
	tokensWithout := m.estimateTokens(msg, true)
	if tokensWith <= tokensWithout {
		t.Errorf("tokensWith(%d) should > tokensWithout(%d) when reasoning is non-empty", tokensWith, tokensWithout)
	}
}

// TestRebuildForResumeWithSummaries verifies synthetic message is rebuilt on resume.
func TestRebuildForResumeWithSummaries(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	sess := makeSession(t, []session.Message{
		{Role: "system", Content: syntheticPrefix + " old synthetic"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	})

	// Save a summary file.
	m.saveSummary(sess.Folder, "fresh summary content")

	if err := m.RebuildForResume(sess); err != nil {
		t.Fatalf("RebuildForResume() error: %v", err)
	}

	// First message should be synthetic with fresh content.
	if len(sess.Messages) == 0 {
		t.Fatal("no messages after rebuild")
	}
	content, ok := sess.Messages[0].Content.(string)
	if !ok {
		t.Fatal("first message content is not string")
	}
	if !strings.Contains(content, "fresh summary content") {
		t.Errorf("synthetic message missing fresh summary: %q", content)
	}
	if strings.Contains(content, "old synthetic") {
		t.Error("synthetic message still contains old synthetic text")
	}

	// Original messages should be preserved.
	if len(sess.Messages) != 3 {
		t.Errorf("messages = %d, want 3 (synthetic + user + assistant)", len(sess.Messages))
	}
}

// TestRebuildForResumeNoSummaries verifies old synthetic is removed when no summaries exist.
func TestRebuildForResumeNoSummaries(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	sess := makeSession(t, []session.Message{
		{Role: "system", Content: syntheticPrefix + " old synthetic"},
		{Role: "user", Content: "hello"},
	})

	if err := m.RebuildForResume(sess); err != nil {
		t.Fatalf("RebuildForResume() error: %v", err)
	}

	// Old synthetic should be removed, no new one added.
	if len(sess.Messages) != 1 {
		t.Errorf("messages = %d, want 1 (just user)", len(sess.Messages))
	}
	content, ok := sess.Messages[0].Content.(string)
	if !ok || content != "hello" {
		t.Errorf("first message = %v, want 'hello'", sess.Messages[0].Content)
	}
}

// TestRebuildForResumeNoSynthetic verifies no change when no synthetic and no summaries.
func TestRebuildForResumeNoSynthetic(t *testing.T) {
	m, server := setupManager(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	sess := makeSession(t, []session.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	})

	originalCount := len(sess.Messages)
	if err := m.RebuildForResume(sess); err != nil {
		t.Fatalf("RebuildForResume() error: %v", err)
	}
	if len(sess.Messages) != originalCount {
		t.Errorf("messages = %d, want %d (unchanged)", len(sess.Messages), originalCount)
	}
}
