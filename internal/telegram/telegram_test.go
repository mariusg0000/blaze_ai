// telegram_test.go — tests for Telegram bridge startup update draining.
// Verifies that pending Telegram updates are discarded before normal polling starts.
package telegram

import (
	"context"
	"path/filepath"
	"testing"

	"blazeai/internal/session"
)

type mockUpdateClient struct {
	updatesCalls []struct {
		offset  int
		timeout int
	}
	updates []Update
	err     error
}

func (m *mockUpdateClient) GetUpdates(_ context.Context, offset int, timeoutSeconds int) ([]Update, error) {
	m.updatesCalls = append(m.updatesCalls, struct {
		offset  int
		timeout int
	}{offset: offset, timeout: timeoutSeconds})
	return m.updates, m.err
}

func (m *mockUpdateClient) SendMessage(_ context.Context, _ int64, _ string) (int, error) {
	return 0, nil
}

func (m *mockUpdateClient) EditMessage(_ context.Context, _ int64, _ int, _ string) error {
	return nil
}

func TestDrainPendingUpdatesUsesLatestUpdateID(t *testing.T) {
	client := &mockUpdateClient{updates: []Update{{UpdateID: 41}, {UpdateID: 44}, {UpdateID: 43}}}
	offset, err := drainPendingUpdates(context.Background(), client)
	if err != nil {
		t.Fatalf("drainPendingUpdates() error: %v", err)
	}
	if offset != 45 {
		t.Fatalf("offset = %d, want 45", offset)
	}
	if len(client.updatesCalls) != 1 {
		t.Fatalf("GetUpdates calls = %d, want 1", len(client.updatesCalls))
	}
	if client.updatesCalls[0].offset != 0 || client.updatesCalls[0].timeout != startupDrainTimeoutSeconds {
		t.Fatalf("GetUpdates args = (%d,%d), want (0,%d)", client.updatesCalls[0].offset, client.updatesCalls[0].timeout, startupDrainTimeoutSeconds)
	}
}

func TestDrainPendingUpdatesWithNoUpdatesStartsAtZero(t *testing.T) {
	client := &mockUpdateClient{}
	offset, err := drainPendingUpdates(context.Background(), client)
	if err != nil {
		t.Fatalf("drainPendingUpdates() error: %v", err)
	}
	if offset != 0 {
		t.Fatalf("offset = %d, want 0", offset)
	}
}

func TestNextOffsetFromUpdatesKeepsHigherInitialOffset(t *testing.T) {
	offset := nextOffsetFromUpdates([]Update{{UpdateID: 10}, {UpdateID: 11}}, 20)
	if offset != 20 {
		t.Fatalf("offset = %d, want 20", offset)
	}
}

func TestOpenTelegramSessionUsesInstanceSessionsDir(t *testing.T) {
	projectSessionsDir := filepath.Join(t.TempDir(), "project-sessions")
	projectSession, err := session.CreateInDir(projectSessionsDir)
	if err != nil {
		t.Fatalf("session.CreateInDir(project) error: %v", err)
	}
	if err := projectSession.Append(session.Message{Role: "user", Content: "console session"}); err != nil {
		t.Fatalf("project session Append() error: %v", err)
	}

	instanceSessionsDir := filepath.Join(t.TempDir(), "telegram", "home", "sessions")
	telegramSession, resumed, err := openTelegramSession(instanceSessionsDir)
	if err != nil {
		t.Fatalf("openTelegramSession() error: %v", err)
	}
	if resumed {
		t.Fatal("resumed = true, want false for a new Telegram instance session")
	}
	if filepath.Dir(telegramSession.Folder) != instanceSessionsDir {
		t.Fatalf("session folder parent = %q, want %q", filepath.Dir(telegramSession.Folder), instanceSessionsDir)
	}
	if telegramSession.Folder == projectSession.Folder {
		t.Fatal("telegram session reused the project session folder")
	}
	if len(telegramSession.Messages) != 0 {
		t.Fatalf("telegram session messages = %d, want 0", len(telegramSession.Messages))
	}

	loadedProjectSession, err := session.Load(projectSession.Folder)
	if err != nil {
		t.Fatalf("session.Load(project) error: %v", err)
	}
	if len(loadedProjectSession.Messages) != 1 {
		t.Fatalf("project session messages = %d, want 1", len(loadedProjectSession.Messages))
	}
	if loadedProjectSession.Messages[0].Content != "console session" {
		t.Fatalf("project session content = %q, want console session", loadedProjectSession.Messages[0].Content)
	}
	loadedTelegramSession, err := session.Load(telegramSession.Folder)
	if err != nil {
		t.Fatalf("session.Load(telegram) error: %v", err)
	}
	if len(loadedTelegramSession.Messages) != 0 {
		t.Fatalf("loaded telegram session messages = %d, want 0", len(loadedTelegramSession.Messages))
	}
	if _, err := session.LastInDir(projectSessionsDir); err != nil {
		t.Fatalf("session.LastInDir(project) error: %v", err)
	}
	lastTelegramSession, err := session.LastInDir(instanceSessionsDir)
	if err != nil {
		t.Fatalf("session.LastInDir(telegram) error: %v", err)
	}
	if lastTelegramSession.Folder != telegramSession.Folder {
		t.Fatalf("last telegram session folder = %q, want %q", lastTelegramSession.Folder, telegramSession.Folder)
	}
}
