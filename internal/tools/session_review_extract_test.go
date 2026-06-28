// session_review_extract_test.go — tests for the session_review_extract tool.
package tools

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// TestSessionReviewExtractToolList verifies recent-session listing.
func TestSessionReviewExtractToolList(t *testing.T) {
	home := t.TempDir()
	sessionPath := filepath.Join(home, "blazeai", "projects", "repo", "sessions", "20260628-100000-a", "session.json")
	tool := NewSessionReviewExtractTool(func(limit int, includeTerminal, includeTelegram bool) ([]SessionReviewSession, error) {
		return []SessionReviewSession{{SessionPath: sessionPath, Transport: "terminal"}}, nil
	}, func(sessionPath string) (string, error) {
		return "", nil
	})
	result := tool.Execute(context.Background(), []byte(`{"purpose":"list newest sessions","action":"list","limit":1}`))
	if !strings.Contains(result, `"sessions"`) {
		t.Fatalf("Execute() missing sessions JSON: %s", result)
	}
	if !strings.Contains(result, `"transport": "terminal"`) {
		t.Fatalf("Execute() missing terminal transport: %s", result)
	}
}

// TestSessionReviewExtractToolExtract verifies transcript extraction by session path.
func TestSessionReviewExtractToolExtract(t *testing.T) {
	home := t.TempDir()
	sessionPath := filepath.Join(home, "blazeai", "projects", "repo", "sessions", "20260628-100000-a", "session.json")
	tool := NewSessionReviewExtractTool(func(limit int, includeTerminal, includeTelegram bool) ([]SessionReviewSession, error) {
		return nil, nil
	}, func(path string) (string, error) {
		if path != sessionPath {
			t.Fatalf("extract path = %q, want %q", path, sessionPath)
		}
		return "# Session Review Extract\n\n[USER]\nReview this session.", nil
	})
	result := tool.Execute(context.Background(), []byte(`{"purpose":"extract session transcript","action":"extract","session_path":"`+sessionPath+`"}`))
	if !strings.Contains(result, "# Session Review Extract") {
		t.Fatalf("Execute() missing transcript header: %s", result)
	}
	if !strings.Contains(result, "[USER]") {
		t.Fatalf("Execute() missing user content: %s", result)
	}
}
