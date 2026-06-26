// telegram_test.go — tests for Telegram bridge startup update draining.
// Verifies that pending Telegram updates are discarded before normal polling starts.
package telegram

import (
	"context"
	"testing"
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
