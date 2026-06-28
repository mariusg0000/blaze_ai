// review_test.go — tests for session discovery and compact transcript extraction.
package learning

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"blazeai/internal/session"
	"blazeai/internal/tools"
)

// TestDiscoverRecentSessions verifies terminal and Telegram sessions are scanned and sorted.
func TestDiscoverRecentSessions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	older := writeLearningSession(t, filepath.Join(home, "blazeai", "projects", "repo", "sessions", "20260628-090000-old"), time.Date(2026, 6, 28, 9, 0, 0, 0, time.UTC), false)
	newer := writeLearningSession(t, filepath.Join(home, "blazeai", "telegram", "bot1", "session"), time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC), true)
	infos, err := DiscoverRecentSessions(30, true, true)
	if err != nil {
		t.Fatalf("DiscoverRecentSessions() error: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("infos = %d, want 2", len(infos))
	}
	if infos[0].SessionPath != newer {
		t.Fatalf("infos[0].SessionPath = %q, want %q", infos[0].SessionPath, newer)
	}
	if infos[0].Transport != "telegram" {
		t.Fatalf("infos[0].Transport = %q, want telegram", infos[0].Transport)
	}
	if !infos[0].HasLearningMD {
		t.Fatal("infos[0].HasLearningMD = false, want true")
	}
	if infos[1].SessionPath != older {
		t.Fatalf("infos[1].SessionPath = %q, want %q", infos[1].SessionPath, older)
	}
	if infos[1].Transport != "terminal" {
		t.Fatalf("infos[1].Transport = %q, want terminal", infos[1].Transport)
	}
}

// TestExtractCompactTranscript verifies the compact learning transcript shape.
func TestExtractCompactTranscript(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sessionPath := writeLearningSession(t, filepath.Join(home, "blazeai", "projects", "repo", "sessions", "20260628-090000-test"), time.Date(2026, 6, 28, 9, 0, 0, 0, time.UTC), false)
	transcript, err := ExtractCompactTranscript(sessionPath)
	if err != nil {
		t.Fatalf("ExtractCompactTranscript() error: %v", err)
	}
	for _, want := range []string{"# Session Review Extract", "[USER]", "[ASSISTANT]", "[TOOL CALL]", "name: shell", "[TOOL RESULT]", "status: error", "summary: permission denied"} {
		if !strings.Contains(transcript, want) {
			t.Fatalf("transcript missing %q\n%s", want, transcript)
		}
	}
}

func writeLearningSession(t *testing.T, dir string, modTime time.Time, withLearning bool) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("cannot create session dir: %v", err)
	}
	s := &session.Session{
		Messages: []session.Message{
			{Role: "user", Content: "Investigate the repeated failure pattern."},
			{Role: "assistant", Content: "I will inspect the command output.", ToolCalls: []tools.OpenAIToolCall{{ID: "call_1", Type: "function", Function: tools.OpenAIFunction{Name: "shell", Arguments: `{"purpose":"inspect failing command","command":"make test"}`}}}},
			{Role: "tool", Name: "shell", ToolCallID: "call_1", Content: "exit_code: 1\nstderr:\npermission denied\n"},
		},
		Folder: dir,
	}
	if err := s.Save(); err != nil {
		t.Fatalf("cannot save test session: %v", err)
	}
	sessionPath := filepath.Join(dir, "session.json")
	if err := os.Chtimes(sessionPath, modTime, modTime); err != nil {
		t.Fatalf("cannot set session modtime: %v", err)
	}
	if withLearning {
		learningPath := filepath.Join(dir, "learning.md")
		if err := os.WriteFile(learningPath, []byte("# Learning Report\n"), 0644); err != nil {
			t.Fatalf("cannot write learning.md: %v", err)
		}
	}
	return sessionPath
}
