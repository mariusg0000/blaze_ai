// desktop_test.go — desktop singleton fixed-session behavior tests.
// Verifies that the desktop transport owns one fixed session folder and resumes
// the same folder on subsequent opens instead of using project sessions.
// Layer: transport tests. Dependencies: internal/desktop, internal/session.
package desktop

import (
	"path/filepath"
	"testing"

	"blazeai/internal/session"
)

func TestOpenDesktopSessionUsesFixedSessionDir(t *testing.T) {
	projectSessionsDir := filepath.Join(t.TempDir(), "project-sessions")
	projectSession, err := session.CreateInDir(projectSessionsDir)
	if err != nil {
		t.Fatalf("session.CreateInDir(project) error: %v", err)
	}
	if err := projectSession.Append(session.Message{Role: "user", Content: "console session"}); err != nil {
		t.Fatalf("project session Append() error: %v", err)
	}

	desktopSessionDir := filepath.Join(t.TempDir(), "desktop", "session")
	desktopSession, resumed, err := openDesktopSession(desktopSessionDir)
	if err != nil {
		t.Fatalf("openDesktopSession() error: %v", err)
	}
	if resumed {
		t.Fatal("resumed = true, want false for a new desktop session")
	}
	if desktopSession.Folder != desktopSessionDir {
		t.Fatalf("session folder = %q, want %q", desktopSession.Folder, desktopSessionDir)
	}
	if desktopSession.Folder == projectSession.Folder {
		t.Fatal("desktop session reused the project session folder")
	}
	if len(desktopSession.Messages) != 0 {
		t.Fatalf("desktop session messages = %d, want 0", len(desktopSession.Messages))
	}

	loadedProjectSession, err := session.Load(projectSession.Folder)
	if err != nil {
		t.Fatalf("session.Load(project) error: %v", err)
	}
	if len(loadedProjectSession.Messages) != 1 {
		t.Fatalf("project session messages = %d, want 1", len(loadedProjectSession.Messages))
	}
	loadedDesktopSession, err := session.Load(desktopSession.Folder)
	if err != nil {
		t.Fatalf("session.Load(desktop) error: %v", err)
	}
	if len(loadedDesktopSession.Messages) != 0 {
		t.Fatalf("loaded desktop session messages = %d, want 0", len(loadedDesktopSession.Messages))
	}
}

func TestOpenDesktopSessionResumesSameFixedSession(t *testing.T) {
	sessionDir := filepath.Join(t.TempDir(), "desktop", "session")
	created, resumed, err := openDesktopSession(sessionDir)
	if err != nil {
		t.Fatalf("openDesktopSession() create error: %v", err)
	}
	if resumed {
		t.Fatal("resumed = true, want false on first open")
	}
	if err := created.Append(session.Message{Role: "user", Content: "hello desktop"}); err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	loaded, resumed, err := openDesktopSession(sessionDir)
	if err != nil {
		t.Fatalf("openDesktopSession() resume error: %v", err)
	}
	if !resumed {
		t.Fatal("resumed = false, want true on second open")
	}
	if loaded.Folder != sessionDir {
		t.Fatalf("loaded folder = %q, want %q", loaded.Folder, sessionDir)
	}
	if len(loaded.Messages) != 1 {
		t.Fatalf("loaded messages = %d, want 1", len(loaded.Messages))
	}
	if loaded.Messages[0].Content != "hello desktop" {
		t.Fatalf("loaded content = %v, want hello desktop", loaded.Messages[0].Content)
	}
}
