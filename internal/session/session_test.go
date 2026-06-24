// session_test.go — tests for session creation, loading, appending, closing, and resume.
// Uses temp directories to avoid touching the real app home.
package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"blazeai/internal/tools"
)

// TestCreateInDir verifies that a new session is created with an empty message array.
func TestCreateInDir(t *testing.T) {
	dir := t.TempDir()
	s, err := CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() unexpected error: %v", err)
	}
	if s.Folder == "" {
		t.Fatal("Folder is empty")
	}
	if len(s.Messages) != 0 {
		t.Errorf("Messages = %d, want 0", len(s.Messages))
	}
	if s.ClosedCleanly {
		t.Error("ClosedCleanly = true, want false")
	}
	path := filepath.Join(s.Folder, "session.json")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("session.json not created: %v", err)
	}
}

// TestLoad verifies that a created session can be loaded back.
func TestLoad(t *testing.T) {
	dir := t.TempDir()
	s, err := CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() failed: %v", err)
	}
	loaded, err := Load(s.Folder)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if len(loaded.Messages) != 0 {
		t.Errorf("Loaded Messages = %d, want 0", len(loaded.Messages))
	}
	if loaded.ClosedCleanly {
		t.Error("Loaded ClosedCleanly = true, want false")
	}
	if loaded.Folder != s.Folder {
		t.Errorf("Loaded Folder = %q, want %q", loaded.Folder, s.Folder)
	}
}

// TestLoadMissing verifies that loading a non-existent folder returns an error.
func TestLoadMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(filepath.Join(dir, "nonexistent"))
	if err == nil {
		t.Fatal("Load() expected error for missing session, got nil")
	}
}

// TestAppend verifies that appending a message persists to disk.
func TestAppend(t *testing.T) {
	dir := t.TempDir()
	s, err := CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() failed: %v", err)
	}
	msg := Message{Role: "user", Content: "hello"}
	if err := s.Append(msg); err != nil {
		t.Fatalf("Append() unexpected error: %v", err)
	}
	if len(s.Messages) != 1 {
		t.Fatalf("Messages = %d, want 1", len(s.Messages))
	}
	loaded, err := Load(s.Folder)
	if err != nil {
		t.Fatalf("Load() after Append failed: %v", err)
	}
	if len(loaded.Messages) != 1 {
		t.Errorf("Loaded Messages = %d, want 1", len(loaded.Messages))
	}
}

// TestAppendAll verifies appending multiple messages at once.
func TestAppendAll(t *testing.T) {
	dir := t.TempDir()
	s, err := CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() failed: %v", err)
	}
	msgs := []Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "user", Content: "do something"},
	}
	if err := s.AppendAll(msgs); err != nil {
		t.Fatalf("AppendAll() unexpected error: %v", err)
	}
	if len(s.Messages) != 3 {
		t.Fatalf("Messages = %d, want 3", len(s.Messages))
	}
	loaded, err := Load(s.Folder)
	if err != nil {
		t.Fatalf("Load() after AppendAll failed: %v", err)
	}
	if len(loaded.Messages) != 3 {
		t.Errorf("Loaded Messages = %d, want 3", len(loaded.Messages))
	}
}

// TestClose verifies that Close sets closed_cleanly and persists.
func TestClose(t *testing.T) {
	dir := t.TempDir()
	s, err := CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() failed: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close() unexpected error: %v", err)
	}
	if !s.ClosedCleanly {
		t.Error("ClosedCleanly = false after Close, want true")
	}
	loaded, err := Load(s.Folder)
	if err != nil {
		t.Fatalf("Load() after Close failed: %v", err)
	}
	if !loaded.ClosedCleanly {
		t.Error("Loaded ClosedCleanly = false, want true")
	}
}

// TestReset verifies that Reset clears messages and reopens the session.
func TestReset(t *testing.T) {
	dir := t.TempDir()
	s, err := CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() failed: %v", err)
	}
	if err := s.Append(Message{Role: "user", Content: "hello"}); err != nil {
		t.Fatalf("Append() failed: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}
	if err := s.Reset(); err != nil {
		t.Fatalf("Reset() unexpected error: %v", err)
	}
	if len(s.Messages) != 0 {
		t.Errorf("Messages = %d, want 0", len(s.Messages))
	}
	if s.ClosedCleanly {
		t.Error("ClosedCleanly = true after Reset, want false")
	}
	loaded, err := Load(s.Folder)
	if err != nil {
		t.Fatalf("Load() after Reset failed: %v", err)
	}
	if len(loaded.Messages) != 0 {
		t.Errorf("Loaded Messages = %d, want 0", len(loaded.Messages))
	}
	if loaded.ClosedCleanly {
		t.Error("Loaded ClosedCleanly = true, want false")
	}
}

// TestLastCleanInDir verifies finding the last cleanly closed session.
func TestLastCleanInDir(t *testing.T) {
	dir := t.TempDir()

	_, err := CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() s1 failed: %v", err)
	}

	s2, err := CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() s2 failed: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := s2.Close(); err != nil {
		t.Fatalf("Close() s2 failed: %v", err)
	}

	_, err = CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() s3 failed: %v", err)
	}

	last, err := LastCleanInDir(dir)
	if err != nil {
		t.Fatalf("LastCleanInDir() unexpected error: %v", err)
	}
	if last.Folder != s2.Folder {
		t.Errorf("LastCleanInDir() = %q, want %q (s2)", last.Folder, s2.Folder)
	}
}

// TestLastCleanInDirNone verifies error when no clean session exists.
func TestLastCleanInDirNone(t *testing.T) {
	dir := t.TempDir()

	_, err := CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() failed: %v", err)
	}

	_, err = LastCleanInDir(dir)
	if err != ErrNoCleanSession {
		t.Errorf("LastCleanInDir() err = %v, want ErrNoCleanSession", err)
	}
}

// TestLastCleanInDirEmpty verifies error when sessions directory is empty.
func TestLastCleanInDirEmpty(t *testing.T) {
	dir := t.TempDir()
	_, err := LastCleanInDir(dir)
	if err != ErrNoCleanSession {
		t.Errorf("LastCleanInDir() err = %v, want ErrNoCleanSession", err)
	}
}

// TestLastCleanInDirMissing verifies error when sessions directory does not exist.
func TestLastCleanInDirMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")
	_, err := LastCleanInDir(dir)
	if err != ErrNoCleanSession {
		t.Errorf("LastCleanInDir() err = %v, want ErrNoCleanSession", err)
	}
}

// TestLastCleanPicksNewest verifies that the newest clean session is returned.
func TestLastCleanPicksNewest(t *testing.T) {
	dir := t.TempDir()

	s1, err := CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() s1 failed: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close() s1 failed: %v", err)
	}

	time.Sleep(15 * time.Millisecond)

	s2, err := CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() s2 failed: %v", err)
	}
	if err := s2.Close(); err != nil {
		t.Fatalf("Close() s2 failed: %v", err)
	}

	last, err := LastCleanInDir(dir)
	if err != nil {
		t.Fatalf("LastCleanInDir() unexpected error: %v", err)
	}
	if last.Folder != s2.Folder {
		t.Errorf("LastCleanInDir() = %q, want %q (s2 is newer)", last.Folder, s2.Folder)
	}
}

// TestLastInDirPicksNewest verifies that the most recent session is returned regardless of clean state.
func TestLastInDirPicksNewest(t *testing.T) {
	dir := t.TempDir()

	s1, err := CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() s1 failed: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close() s1 failed: %v", err)
	}

	time.Sleep(15 * time.Millisecond)

	s2, err := CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() s2 failed: %v", err)
	}
	// s2 is not closed cleanly — simulates an interrupted session.

	last, err := LastInDir(dir)
	if err != nil {
		t.Fatalf("LastInDir() unexpected error: %v", err)
	}
	if last.Folder != s2.Folder {
		t.Errorf("LastInDir() = %q, want %q (s2 is newer, unclean)", last.Folder, s2.Folder)
	}
	if last.ClosedCleanly {
		t.Error("Last session should not be marked cleanly closed")
	}
}

// TestLastInDirOnlyClean verifies LastInDir returns a clean session when it's the only one.
func TestLastInDirOnlyClean(t *testing.T) {
	dir := t.TempDir()

	s, err := CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() failed: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	last, err := LastInDir(dir)
	if err != nil {
		t.Fatalf("LastInDir() unexpected error: %v", err)
	}
	if last.Folder != s.Folder {
		t.Errorf("LastInDir() = %q, want %q", last.Folder, s.Folder)
	}
	if !last.ClosedCleanly {
		t.Error("LastInDir() should return cleanly closed session")
	}
}

// TestLastInDirEmpty verifies error when sessions directory is empty.
func TestLastInDirEmpty(t *testing.T) {
	dir := t.TempDir()
	_, err := LastInDir(dir)
	if err != ErrNoSessions {
		t.Errorf("LastInDir() err = %v, want ErrNoSessions", err)
	}
}

// TestLastInDirMissing verifies error when sessions directory does not exist.
func TestLastInDirMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")
	_, err := LastInDir(dir)
	if err != ErrNoSessions {
		t.Errorf("LastInDir() err = %v, want ErrNoSessions", err)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() failed: %v", err)
	}
	msgs := []Message{
		{Role: "user", Content: "what is 2+2?"},
		{Role: "assistant", Content: "4"},
		{Role: "user", Content: "thanks"},
		{Role: "assistant", Content: "you're welcome"},
	}
	if err := s.AppendAll(msgs); err != nil {
		t.Fatalf("AppendAll() failed: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	loaded, err := Load(s.Folder)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if len(loaded.Messages) != 4 {
		t.Fatalf("Loaded Messages = %d, want 4", len(loaded.Messages))
	}
	if !loaded.ClosedCleanly {
		t.Error("Loaded ClosedCleanly = false, want true")
	}
}

// TestRandomName verifies that generated names are non-empty and unique.
func TestRandomName(t *testing.T) {
	name1 := randomName()
	name2 := randomName()
	if name1 == "" {
		t.Error("randomName() returned empty string")
	}
	if name1 == name2 {
		t.Error("randomName() returned duplicate names")
	}
}

// TestSanitizeRemovesIncompleteToolCalls verifies trailing assistant with tool_calls is removed.
func TestSanitizeRemovesIncompleteToolCalls(t *testing.T) {
	dir := t.TempDir()
	s, err := CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() failed: %v", err)
	}
	if err := s.AppendAll([]Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
		{Role: "user", Content: "run something"},
		{Role: "assistant", Content: "", ToolCalls: []interface{}{map[string]interface{}{"id": "call_1"}}},
	}); err != nil {
		t.Fatalf("AppendAll() failed: %v", err)
	}

	if err := s.Sanitize(); err != nil {
		t.Fatalf("Sanitize() failed: %v", err)
	}

	if len(s.Messages) != 3 {
		t.Fatalf("Messages = %d, want 3", len(s.Messages))
	}
	if s.Messages[2].Role != "user" {
		t.Errorf("Last message role = %q, want 'user'", s.Messages[2].Role)
	}
}

// TestSanitizeKeepsCompleteToolCalls verifies a round with matching tool results is kept.
func TestSanitizeKeepsCompleteToolCalls(t *testing.T) {
	dir := t.TempDir()
	s, err := CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() failed: %v", err)
	}
	if err := s.AppendAll([]Message{
		{Role: "user", Content: "run something"},
		{Role: "assistant", Content: "", ToolCalls: []interface{}{map[string]interface{}{"id": "call_1"}}},
		{Role: "tool", Content: "done", ToolCallID: "call_1"},
	}); err != nil {
		t.Fatalf("AppendAll() failed: %v", err)
	}

	if err := s.Sanitize(); err != nil {
		t.Fatalf("Sanitize() failed: %v", err)
	}

	if len(s.Messages) != 3 {
		t.Fatalf("Messages = %d, want 3 (complete round kept)", len(s.Messages))
	}
}

// TestSanitizeRemovesIncompleteMultiToolCall verifies incomplete multi-call round is stripped.
func TestSanitizeRemovesIncompleteMultiToolCall(t *testing.T) {
	dir := t.TempDir()
	s, err := CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() failed: %v", err)
	}
	if err := s.AppendAll([]Message{
		{Role: "user", Content: "run stuff"},
		{Role: "assistant", Content: "", ToolCalls: []interface{}{
			map[string]interface{}{"id": "call_1"},
			map[string]interface{}{"id": "call_2"},
		}},
		{Role: "tool", Content: "done", ToolCallID: "call_1"},
	}); err != nil {
		t.Fatalf("AppendAll() failed: %v", err)
	}

	if err := s.Sanitize(); err != nil {
		t.Fatalf("Sanitize() failed: %v", err)
	}

	if len(s.Messages) != 1 {
		t.Fatalf("Messages = %d, want 1 (incomplete multi-call stripped)", len(s.Messages))
	}
}

// TestSanitizeRemovesIncompleteToolCallBeforeLaterUserMessages verifies later user messages
// do not hide an earlier incomplete assistant tool-call round.
func TestSanitizeRemovesIncompleteToolCallBeforeLaterUserMessages(t *testing.T) {
	dir := t.TempDir()
	s, err := CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() failed: %v", err)
	}
	if err := s.AppendAll([]Message{
		{Role: "user", Content: "start"},
		{Role: "assistant", Content: "", ToolCalls: []interface{}{map[string]interface{}{"id": "call_1"}}},
		{Role: "user", Content: "continua"},
		{Role: "user", Content: "continua"},
	}); err != nil {
		t.Fatalf("AppendAll() failed: %v", err)
	}

	if err := s.Sanitize(); err != nil {
		t.Fatalf("Sanitize() failed: %v", err)
	}

	if len(s.Messages) != 1 {
		t.Fatalf("Messages = %d, want 1 after truncating from incomplete assistant", len(s.Messages))
	}
	if s.Messages[0].Role != "user" {
		t.Fatalf("Remaining message role = %q, want 'user'", s.Messages[0].Role)
	}
}

// TestSanitizeKeepsCompleteMultiToolCall verifies complete multi-call round is kept.
func TestSanitizeKeepsCompleteMultiToolCall(t *testing.T) {
	dir := t.TempDir()
	s, err := CreateInDir(dir)
	if err != nil {
		t.Fatalf("CreateInDir() failed: %v", err)
	}
	if err := s.AppendAll([]Message{
		{Role: "user", Content: "run stuff"},
		{Role: "assistant", Content: "", ToolCalls: []interface{}{
			map[string]interface{}{"id": "call_1"},
			map[string]interface{}{"id": "call_2"},
		}},
		{Role: "tool", Content: "done", ToolCallID: "call_1"},
		{Role: "tool", Content: "done", ToolCallID: "call_2"},
	}); err != nil {
		t.Fatalf("AppendAll() failed: %v", err)
	}

	if err := s.Sanitize(); err != nil {
		t.Fatalf("Sanitize() failed: %v", err)
	}

	if len(s.Messages) != 4 {
		t.Fatalf("Messages = %d, want 4 (complete multi-call kept)", len(s.Messages))
	}
}

// TestSanitizeMessagesRemovesLeadingOrphanTool verifies standalone tool messages are dropped.
func TestSanitizeMessagesRemovesLeadingOrphanTool(t *testing.T) {
	sanitized, removed := SanitizeMessages([]Message{
		{Role: "tool", Content: "orphan", ToolCallID: "call_1", Name: "shell"},
		{Role: "assistant", Content: "done"},
	})

	if len(sanitized) != 1 || sanitized[0].Role != "assistant" {
		t.Fatalf("sanitized = %#v, want only assistant message", sanitized)
	}
	if len(removed) != 1 || removed[0].Role != "tool" {
		t.Fatalf("removed = %#v, want one orphan tool", removed)
	}
}

// TestSanitizeMessagesRemovesMismatchedToolID verifies unexpected tool_call_id values are removed.
func TestSanitizeMessagesRemovesMismatchedToolID(t *testing.T) {
	sanitized, removed := SanitizeMessages([]Message{
		{Role: "assistant", ToolCalls: []interface{}{map[string]interface{}{"id": "call_1"}}},
		{Role: "tool", Content: "wrong", ToolCallID: "call_2", Name: "shell"},
		{Role: "tool", Content: "right", ToolCallID: "call_1", Name: "shell"},
		{Role: "assistant", Content: "done"},
	})

	if len(sanitized) != 3 {
		t.Fatalf("sanitized len = %d, want 3", len(sanitized))
	}
	if sanitized[1].Role != "tool" || sanitized[1].ToolCallID != "call_1" {
		t.Fatalf("sanitized tool = %#v, want call_1", sanitized[1])
	}
	if len(removed) != 1 || removed[0].ToolCallID != "call_2" {
		t.Fatalf("removed = %#v, want mismatched call_2 tool", removed)
	}
}

// TestSanitizeMessagesReturnsTruncatedTail verifies incomplete rounds return the truncated messages.
func TestSanitizeMessagesReturnsTruncatedTail(t *testing.T) {
	sanitized, removed := SanitizeMessages([]Message{
		{Role: "user", Content: "start"},
		{Role: "assistant", ToolCalls: []interface{}{
			map[string]interface{}{"id": "call_1"},
			map[string]interface{}{"id": "call_2"},
		}},
		{Role: "tool", Content: "only one", ToolCallID: "call_1", Name: "shell"},
		{Role: "assistant", Content: "should be dropped too"},
	})

	if len(sanitized) != 1 || sanitized[0].Role != "user" {
		t.Fatalf("sanitized = %#v, want only initial user message", sanitized)
	}
	if len(removed) != 3 {
		t.Fatalf("removed len = %d, want 3 truncated messages", len(removed))
	}
	if removed[0].Role != "assistant" || removed[2].Role != "assistant" {
		t.Fatalf("removed = %#v, want truncated tail starting at assistant tool round", removed)
	}
}

// TestSanitizeMessagesKeepsRuntimeOpenAIToolCalls verifies sanitizer handles
// the exact in-memory tool_call type persisted by the runtime before JSON reload.
func TestSanitizeMessagesKeepsRuntimeOpenAIToolCalls(t *testing.T) {
	sanitized, removed := SanitizeMessages([]Message{
		{
			Role: "assistant",
			ToolCalls: []tools.OpenAIToolCall{
				{ID: "call_1", Type: "function", Function: tools.OpenAIFunction{Name: "shell", Arguments: `{"command":"pwd"}`}},
			},
		},
		{Role: "tool", Content: "exit_code: 0\nstdout:\n/mnt/work\n", ToolCallID: "call_1", Name: "shell"},
		{Role: "assistant", Content: "done"},
	})

	if len(removed) != 0 {
		t.Fatalf("removed = %#v, want none", removed)
	}
	if len(sanitized) != 3 {
		t.Fatalf("sanitized len = %d, want 3", len(sanitized))
	}
	if sanitized[1].Role != "tool" || sanitized[1].ToolCallID != "call_1" {
		t.Fatalf("sanitized tool = %#v, want matching tool result kept", sanitized[1])
	}
}
