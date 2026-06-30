// image_messages_test.go — tests for Telegram image intake and download helpers.
package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type mockImageClient struct {
	file        *TelegramFile
	fileErr     error
	data        []byte
	downloadErr error
	downloads   []struct {
		filePath        string
		destinationPath string
	}
}

func (m *mockImageClient) GetUpdates(_ context.Context, _ int, _ int) ([]Update, error) {
	return nil, nil
}

func (m *mockImageClient) SendMessage(_ context.Context, _ int64, _ string) (int, error) {
	return 0, nil
}

func (m *mockImageClient) EditMessage(_ context.Context, _ int64, _ int, _ string) error {
	return nil
}

func (m *mockImageClient) SetCommands(_ context.Context, _ []botCommand) error {
	return nil
}

func (m *mockImageClient) SendChatAction(_ context.Context, _ int64, _ string) error {
	return nil
}

func (m *mockImageClient) GetFile(_ context.Context, _ string) (*TelegramFile, error) {
	if m.fileErr != nil {
		return nil, m.fileErr
	}
	return m.file, nil
}

func (m *mockImageClient) DownloadFile(_ context.Context, filePath string, destinationPath string) error {
	m.downloads = append(m.downloads, struct {
		filePath        string
		destinationPath string
	}{filePath: filePath, destinationPath: destinationPath})
	if m.downloadErr != nil {
		return m.downloadErr
	}
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(destinationPath, m.data, 0644)
}

// TestBuildTelegramTurnInputKeepsText verifies plain text messages remain unchanged.
func TestBuildTelegramTurnInputKeepsText(t *testing.T) {
	result, err := buildTelegramTurnInput(context.Background(), &mockImageClient{}, t.TempDir(), &Message{Text: "  hello  "})
	if err != nil {
		t.Fatalf("buildTelegramTurnInput() error: %v", err)
	}
	if result != "hello" {
		t.Fatalf("buildTelegramTurnInput() = %q, want hello", result)
	}
}

// TestBuildTelegramTurnInputPhotoWithoutCaption verifies image download and synthetic prompt generation.
func TestBuildTelegramTurnInputPhotoWithoutCaption(t *testing.T) {
	client := &mockImageClient{
		file: &TelegramFile{FileID: "photo-file", FileUniqueID: "uniq-1", FilePath: "photos/example.jpg"},
		data: []byte("jpeg bytes"),
	}
	attachmentsDir := t.TempDir()
	result, err := buildTelegramTurnInput(context.Background(), client, attachmentsDir, &Message{
		MessageID: 44,
		Photo:     []PhotoSize{{FileID: "small", FileUniqueID: "small-1", Width: 100, Height: 100, FileSize: 100}, {FileID: "large", FileUniqueID: "large-1", Width: 800, Height: 600, FileSize: 5000}},
	})
	if err != nil {
		t.Fatalf("buildTelegramTurnInput() error: %v", err)
	}
	if len(client.downloads) != 1 {
		t.Fatalf("downloads = %d, want 1", len(client.downloads))
	}
	if client.downloads[0].filePath != "photos/example.jpg" {
		t.Fatalf("download file path = %q, want photos/example.jpg", client.downloads[0].filePath)
	}
	if !strings.Contains(result, "Use the analyze_image tool on this file.") {
		t.Fatalf("prompt missing analyze_image instruction: %q", result)
	}
	if !strings.Contains(result, "No caption was provided.") {
		t.Fatalf("prompt missing empty caption marker: %q", result)
	}
	if !strings.Contains(result, client.downloads[0].destinationPath) {
		t.Fatalf("prompt missing local path: %q", result)
	}
	if _, err := os.Stat(client.downloads[0].destinationPath); err != nil {
		t.Fatalf("downloaded file missing: %v", err)
	}
}

// TestBuildTelegramTurnInputImageDocumentWithCaption verifies image documents use caption guidance.
func TestBuildTelegramTurnInputImageDocumentWithCaption(t *testing.T) {
	client := &mockImageClient{
		file: &TelegramFile{FileID: "doc-file", FileUniqueID: "uniq-2", FilePath: "documents/scan.png"},
		data: []byte("png bytes"),
	}
	result, err := buildTelegramTurnInput(context.Background(), client, t.TempDir(), &Message{
		MessageID: 18,
		Caption:   "what error is visible here?",
		Document:  &Document{FileID: "doc-file", FileUniqueID: "uniq-2", MimeType: "image/png"},
	})
	if err != nil {
		t.Fatalf("buildTelegramTurnInput() error: %v", err)
	}
	if !strings.Contains(result, "what error is visible here?") {
		t.Fatalf("prompt missing caption guidance: %q", result)
	}
	if !strings.Contains(result, "Generate the exact analyze_image question yourself") {
		t.Fatalf("prompt missing question synthesis guidance: %q", result)
	}
}

// TestBuildTelegramTurnInputRejectsNonImageDocument verifies clear rejection for non-image documents.
func TestBuildTelegramTurnInputRejectsNonImageDocument(t *testing.T) {
	_, err := buildTelegramTurnInput(context.Background(), &mockImageClient{}, t.TempDir(), &Message{
		Document: &Document{FileID: "doc", MimeType: "application/pdf"},
	})
	if err == nil {
		t.Fatal("buildTelegramTurnInput() error = nil, want non-image document error")
	}
	if err.Error() != "telegram document is not an image: application/pdf" {
		t.Fatalf("error = %q", err.Error())
	}
}

// TestUpdateMessageUsesChannelPost verifies channels reuse the same message processing path.
func TestUpdateMessageUsesChannelPost(t *testing.T) {
	msg := &Message{MessageID: 7, Caption: "channel image"}
	result := updateMessage(Update{ChannelPost: msg})
	if result != msg {
		t.Fatal("updateMessage() did not return channel_post message")
	}
}

// TestBotClientGetFile verifies Bot API file metadata parsing.
func TestBotClientGetFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getFile" {
			t.Fatalf("path = %q, want /getFile", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error: %v", err)
		}
		if r.Form.Get("file_id") != "abc" {
			t.Fatalf("file_id = %q, want abc", r.Form.Get("file_id"))
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"file_id":"abc","file_unique_id":"uniq","file_path":"photos/pic.jpg"}}`))
	}))
	defer server.Close()
	client := &BotClient{baseURL: server.URL, fileBaseURL: server.URL, httpClient: server.Client()}
	file, err := client.GetFile(context.Background(), "abc")
	if err != nil {
		t.Fatalf("GetFile() error: %v", err)
	}
	if file.FilePath != "photos/pic.jpg" {
		t.Fatalf("FilePath = %q, want photos/pic.jpg", file.FilePath)
	}
}

// TestBotClientDownloadFile verifies remote Telegram files are written locally.
func TestBotClientDownloadFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/photos/pic.jpg" {
			t.Fatalf("path = %q, want /photos/pic.jpg", r.URL.Path)
		}
		_, _ = w.Write([]byte("image bytes"))
	}))
	defer server.Close()
	client := &BotClient{baseURL: server.URL, fileBaseURL: server.URL, httpClient: server.Client()}
	destinationPath := filepath.Join(t.TempDir(), "downloads", "pic.jpg")
	if err := client.DownloadFile(context.Background(), "photos/pic.jpg", destinationPath); err != nil {
		t.Fatalf("DownloadFile() error: %v", err)
	}
	data, err := os.ReadFile(destinationPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != "image bytes" {
		t.Fatalf("downloaded data = %q, want image bytes", string(data))
	}
}
