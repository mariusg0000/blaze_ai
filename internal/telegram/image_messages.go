// image_messages.go — Telegram image intake and prompt shaping.
// Downloads Telegram photos or image documents into the instance storage and
// converts them into a user turn that asks the main agent to use analyze_image.
// Layer: transport input. Dependencies: internal/runtime via the shared agent turn path.
package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// telegramImageSource identifies one Telegram-hosted image file.
//
// WHAT:  File metadata needed to resolve and persist one incoming Telegram image.
// WHY:   Telegram updates may carry photos or image documents under different fields.
// PARAMS: FileID — Bot API file lookup identifier; FileUniqueID — stable Telegram file identity.
type telegramImageSource struct {
	FileID       string
	FileUniqueID string
}

// updateMessage returns the actual inbound message regardless of chat type.
func updateMessage(update Update) *Message {
	if update.Message != nil {
		return update.Message
	}
	return update.ChannelPost
}

// buildTelegramTurnInput converts one Telegram message into a normal agent input string.
func buildTelegramTurnInput(ctx context.Context, client telegramClient, attachmentsDir string, msg *Message) (string, error) {
	if msg == nil {
		return "", nil
	}
	text := strings.TrimSpace(msg.Text)
	if text != "" {
		return text, nil
	}
	path, err := downloadTelegramImageMessage(ctx, client, attachmentsDir, msg)
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil
	}
	return formatTelegramImageTurnInput(path, strings.TrimSpace(msg.Caption)), nil
}

// downloadTelegramImageMessage persists a Telegram image locally and returns its path.
func downloadTelegramImageMessage(ctx context.Context, client telegramClient, attachmentsDir string, msg *Message) (string, error) {
	source, err := extractTelegramImageSource(msg)
	if err != nil {
		return "", err
	}
	if source == nil {
		return "", nil
	}
	if client == nil {
		return "", fmt.Errorf("telegram client is nil")
	}
	meta, err := client.GetFile(ctx, source.FileID)
	if err != nil {
		return "", fmt.Errorf("cannot fetch telegram file metadata: %w", err)
	}
	if meta == nil {
		return "", fmt.Errorf("telegram file metadata is missing for %s", source.FileID)
	}
	filePath := strings.TrimSpace(meta.FilePath)
	if filePath == "" {
		return "", fmt.Errorf("telegram file metadata is missing file_path for %s", source.FileID)
	}
	if err := os.MkdirAll(attachmentsDir, 0755); err != nil {
		return "", fmt.Errorf("cannot create telegram attachments directory %s: %w", attachmentsDir, err)
	}
	fileID := strings.TrimSpace(meta.FileUniqueID)
	if fileID == "" {
		fileID = strings.TrimSpace(source.FileUniqueID)
	}
	if fileID == "" {
		fileID = strings.TrimSpace(meta.FileID)
	}
	if fileID == "" {
		fileID = strings.TrimSpace(source.FileID)
	}
	localPath := filepath.Join(attachmentsDir, buildTelegramAttachmentName(msg.MessageID, fileID, filePath))
	if err := client.DownloadFile(ctx, filePath, localPath); err != nil {
		return "", fmt.Errorf("cannot download telegram image %s: %w", source.FileID, err)
	}
	return localPath, nil
}

// extractTelegramImageSource selects the incoming image payload from a message.
func extractTelegramImageSource(msg *Message) (*telegramImageSource, error) {
	if msg == nil {
		return nil, nil
	}
	if len(msg.Photo) > 0 {
		best := largestPhotoSize(msg.Photo)
		if best == nil {
			return nil, fmt.Errorf("telegram photo message is missing file metadata")
		}
		if strings.TrimSpace(best.FileID) == "" {
			return nil, fmt.Errorf("telegram photo message is missing file_id")
		}
		return &telegramImageSource{FileID: best.FileID, FileUniqueID: best.FileUniqueID}, nil
	}
	if msg.Document == nil {
		return nil, nil
	}
	mimeType := strings.TrimSpace(msg.Document.MimeType)
	if !strings.HasPrefix(mimeType, "image/") {
		return nil, fmt.Errorf("telegram document is not an image: %s", mimeType)
	}
	if strings.TrimSpace(msg.Document.FileID) == "" {
		return nil, fmt.Errorf("telegram image document is missing file_id")
	}
	return &telegramImageSource{FileID: msg.Document.FileID, FileUniqueID: msg.Document.FileUniqueID}, nil
}

// largestPhotoSize returns the best photo candidate from Telegram's photo size list.
func largestPhotoSize(sizes []PhotoSize) *PhotoSize {
	if len(sizes) == 0 {
		return nil
	}
	best := &sizes[0]
	for i := 1; i < len(sizes); i++ {
		candidate := &sizes[i]
		if candidate.FileSize > best.FileSize {
			best = candidate
			continue
		}
		if candidate.FileSize == best.FileSize && candidate.Width*candidate.Height > best.Width*best.Height {
			best = candidate
		}
	}
	return best
}

// buildTelegramAttachmentName creates a stable local filename for a downloaded Telegram image.
func buildTelegramAttachmentName(messageID int, fileUniqueID string, remoteFilePath string) string {
	ext := filepath.Ext(strings.TrimSpace(remoteFilePath))
	return fmt.Sprintf("%d-%s%s", messageID, sanitizeTelegramAttachmentID(fileUniqueID), ext)
}

// sanitizeTelegramAttachmentID strips path-unsafe characters from Telegram file identifiers.
func sanitizeTelegramAttachmentID(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "attachment"
	}
	var out strings.Builder
	for _, r := range trimmed {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_':
			out.WriteRune(r)
		default:
			out.WriteByte('_')
		}
	}
	if out.Len() == 0 {
		return "attachment"
	}
	return out.String()
}

// formatTelegramImageTurnInput creates the synthetic user text fed to the main agent.
func formatTelegramImageTurnInput(localPath string, caption string) string {
	captionBlock := "No caption was provided."
	if strings.TrimSpace(caption) != "" {
		captionBlock = caption
	}
	return strings.TrimSpace(fmt.Sprintf(`A Telegram image message was received.
Local image path: %s

Use the analyze_image tool on this file.
Generate the exact analyze_image question yourself from the current conversation context and the visible image content.
If the user caption below is present, treat it as additional guidance, not as the only source of intent.
Then answer the user directly after the tool result.

Telegram caption:
%s`, localPath, captionBlock))
}
