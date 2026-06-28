// telegram_test.go — tests for Telegram bridge startup update draining.
// Verifies that pending Telegram updates are discarded before normal polling starts.
package telegram

import (
	"context"
	"errors"
	"io"
	"net"
	"path/filepath"
	"testing"

	"blazeai/internal/session"
)

type mockUpdateClient struct {
	updatesCalls []struct {
		offset  int
		timeout int
	}
	updates    []Update
	errs       []error
	defaultErr error
	commands   [][]botCommand
}

func (m *mockUpdateClient) GetUpdates(_ context.Context, offset int, timeoutSeconds int) ([]Update, error) {
	m.updatesCalls = append(m.updatesCalls, struct {
		offset  int
		timeout int
	}{offset: offset, timeout: timeoutSeconds})
	if len(m.errs) > 0 {
		err := m.errs[0]
		m.errs = m.errs[1:]
		if err != nil {
			return nil, err
		}
	}
	if m.defaultErr != nil {
		return nil, m.defaultErr
	}
	return m.updates, nil
}

func (m *mockUpdateClient) SendMessage(_ context.Context, _ int64, _ string) (int, error) {
	return 0, nil
}

func (m *mockUpdateClient) EditMessage(_ context.Context, _ int64, _ int, _ string) error {
	return nil
}

func (m *mockUpdateClient) SetCommands(_ context.Context, commands []botCommand) error {
	copyCommands := append([]botCommand(nil), commands...)
	m.commands = append(m.commands, copyCommands)
	return nil
}

func (m *mockUpdateClient) SendChatAction(_ context.Context, _ int64, _ string) error {
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

func TestPublishTelegramCommandsUsesSupportedCommandList(t *testing.T) {
	client := &mockUpdateClient{}
	if err := publishTelegramCommands(context.Background(), client); err != nil {
		t.Fatalf("publishTelegramCommands() error: %v", err)
	}
	if len(client.commands) != 1 {
		t.Fatalf("SetCommands calls = %d, want 1", len(client.commands))
	}
	if len(client.commands[0]) != len(telegramBotCommands()) {
		t.Fatalf("published commands = %d, want %d", len(client.commands[0]), len(telegramBotCommands()))
	}
	if client.commands[0][0].Command != "help" {
		t.Fatalf("first published command = %q, want help", client.commands[0][0].Command)
	}
}

func TestOpenTelegramSessionUsesFixedInstanceSessionDir(t *testing.T) {
	projectSessionsDir := filepath.Join(t.TempDir(), "project-sessions")
	projectSession, err := session.CreateInDir(projectSessionsDir)
	if err != nil {
		t.Fatalf("session.CreateInDir(project) error: %v", err)
	}
	if err := projectSession.Append(session.Message{Role: "user", Content: "console session"}); err != nil {
		t.Fatalf("project session Append() error: %v", err)
	}

	instanceSessionDir := filepath.Join(t.TempDir(), "telegram", "home", "session")
	telegramSession, resumed, err := openTelegramSession(instanceSessionDir)
	if err != nil {
		t.Fatalf("openTelegramSession() error: %v", err)
	}
	if resumed {
		t.Fatal("resumed = true, want false for a new Telegram instance session")
	}
	if telegramSession.Folder != instanceSessionDir {
		t.Fatalf("session folder = %q, want %q", telegramSession.Folder, instanceSessionDir)
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
	lastTelegramSession, err := session.Load(instanceSessionDir)
	if err != nil {
		t.Fatalf("session.Load(telegram) error: %v", err)
	}
	if lastTelegramSession.Folder != telegramSession.Folder {
		t.Fatalf("last telegram session folder = %q, want %q", lastTelegramSession.Folder, telegramSession.Folder)
	}
}

func TestOpenTelegramSessionResumesSameFixedSession(t *testing.T) {
	sessionDir := filepath.Join(t.TempDir(), "telegram", "home", "session")
	created, resumed, err := openTelegramSession(sessionDir)
	if err != nil {
		t.Fatalf("openTelegramSession() create error: %v", err)
	}
	if resumed {
		t.Fatal("resumed = true, want false on first open")
	}
	if err := created.Append(session.Message{Role: "user", Content: "hello telegram"}); err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	loaded, resumed, err := openTelegramSession(sessionDir)
	if err != nil {
		t.Fatalf("openTelegramSession() resume error: %v", err)
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
	if loaded.Messages[0].Content != "hello telegram" {
		t.Fatalf("loaded content = %v, want hello telegram", loaded.Messages[0].Content)
	}
}

func TestGetUpdatesWithRetryRetriesTransientError(t *testing.T) {
	client := &mockUpdateClient{
		errs:    []error{&net.DNSError{IsTemporary: true, Err: "temporary telegram dns failure"}, nil},
		updates: []Update{{UpdateID: 7}},
	}

	updates, err := getUpdatesWithRetry(context.Background(), client, 15, pollingTimeoutSeconds, 0)
	if err != nil {
		t.Fatalf("getUpdatesWithRetry() error: %v", err)
	}
	if len(updates) != 1 || updates[0].UpdateID != 7 {
		t.Fatalf("updates = %+v, want one update with id 7", updates)
	}
	if len(client.updatesCalls) != 2 {
		t.Fatalf("GetUpdates calls = %d, want 2", len(client.updatesCalls))
	}
	if client.updatesCalls[0].offset != 15 || client.updatesCalls[0].timeout != pollingTimeoutSeconds {
		t.Fatalf("first GetUpdates args = (%d,%d), want (15,%d)", client.updatesCalls[0].offset, client.updatesCalls[0].timeout, pollingTimeoutSeconds)
	}
}

func TestGetUpdatesWithRetryStopsOnNonRetryableError(t *testing.T) {
	client := &mockUpdateClient{defaultErr: errors.New("telegram getUpdates failed: unauthorized")}

	_, err := getUpdatesWithRetry(context.Background(), client, 3, pollingTimeoutSeconds, 0)
	if err == nil {
		t.Fatal("getUpdatesWithRetry() error = nil, want non-retryable error")
	}
	if len(client.updatesCalls) != 1 {
		t.Fatalf("GetUpdates calls = %d, want 1", len(client.updatesCalls))
	}
}

func TestIsRetryablePollingError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "context canceled", err: context.Canceled, want: false},
		{name: "deadline", err: context.DeadlineExceeded, want: false},
		{name: "eof", err: io.EOF, want: true},
		{name: "net error", err: &net.DNSError{IsTemporary: true, Err: "timeout"}, want: true},
		{name: "connection reset text", err: errors.New("telegram getUpdates request failed: read tcp [::1]:443: read: connection reset by peer"), want: true},
		{name: "telegram api error", err: errors.New("telegram getUpdates failed: unauthorized"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryablePollingError(tt.err); got != tt.want {
				t.Fatalf("isRetryablePollingError() = %v, want %v", got, tt.want)
			}
		})
	}
}
