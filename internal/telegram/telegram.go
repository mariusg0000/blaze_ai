// telegram.go — Telegram bridge startup, polling, and runtime wiring.
// Boots one Telegram instance from app_home storage, resumes or creates its session,
// and serializes message handling into one runtime agent.
// Layer: transport runtime. Dependencies: internal/config, internal/platform, internal/runtime, internal/session.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"blazeai/internal/config"
	"blazeai/internal/platform"
	"blazeai/internal/runtime"
	"blazeai/internal/session"
)

const startupDrainTimeoutSeconds = 0
const pollingTimeoutSeconds = 30
const pollingRetryDelay = 2 * time.Second

type telegramClient interface {
	GetUpdates(ctx context.Context, offset int, timeoutSeconds int) ([]Update, error)
	SendMessage(ctx context.Context, chatID int64, text string) (int, error)
	EditMessage(ctx context.Context, chatID int64, messageID int, text string) error
	SetCommands(ctx context.Context, commands []botCommand) error
	SendChatAction(ctx context.Context, chatID int64, action string) error
}

// Run starts one Telegram bridge instance and blocks in the polling loop.
//
// WHAT:  Boots a Telegram bot instance over the shared runtime agent core.
// WHY:   Telegram is a transport over the existing Handler contract.
// PARAMS: ctx — process lifetime context; cfg — global runtime config; osType — detected OS;
// promptsFS — embedded prompt filesystem; instance — Telegram instance folder name.
// RETURNS: error if startup, polling, or request handling fails.
func Run(ctx context.Context, cfg *config.Config, osType platform.OS, promptsFS fs.FS, instance string) error {
	bridgeCfg, _, err := LoadBridgeConfig(instance)
	if err != nil {
		return err
	}
	state, statePath, err := LoadState(instance, cfg)
	if err != nil {
		return err
	}
	instanceDir, err := InstanceDir(instance)
	if err != nil {
		return err
	}
	sessDir := filepath.Join(instanceDir, "session")
	sess, resumed, err := openTelegramSession(sessDir)
	if err != nil {
		return err
	}
	agent, err := runtime.NewAgent(cfg, sess, osType, promptsFS, bridgeCfg.WorkDir, nil)
	if err != nil {
		return fmt.Errorf("cannot create telegram agent: %w", err)
	}
	agent.Builder.TransportContext = strings.TrimSpace(fmt.Sprintf(`Telegram bridge transport is active.
Telegram instance: %s
Replies are sent into a Telegram chat, not an interactive terminal.
Exactly one configured chat can reach this instance.
Do not start, restart, or duplicate BlazeAI or Telegram bridge processes unless the user explicitly asks.
Do not treat generic greetings or /start as setup instructions.
Keep replies concise for chat and avoid unnecessary tool chatter.`, instance))
	if resumed && agent.Compactor != nil {
		if err := agent.Compactor.RebuildForResume(sess); err != nil {
			return fmt.Errorf("cannot rebuild summaries for telegram resume: %w", err)
		}
	}
	if err := agent.SetModelLocal(state.SelectedModel); err != nil {
		return fmt.Errorf("cannot apply telegram instance model: %w", err)
	}
	client := NewBotClient(bridgeCfg.BotToken)
	if err := publishTelegramCommands(ctx, client); err != nil {
		return err
	}
	handler := NewHandler(client, bridgeCfg.AllowedChatID)
	agent.Handler = handler
	return runPolling(ctx, client, bridgeCfg, state, statePath, agent, cfg, handler)
}

func publishTelegramCommands(ctx context.Context, client telegramClient) error {
	if err := client.SetCommands(ctx, telegramBotCommands()); err != nil {
		return fmt.Errorf("cannot publish telegram bot commands: %w", err)
	}
	return nil
}

func openTelegramSession(sessionDir string) (*session.Session, bool, error) {
	sess, err := session.Load(sessionDir)
	if err == nil {
		return sess, true, nil
	}
	if !errors.Is(err, session.ErrSessionNotFound) {
		return nil, false, fmt.Errorf("cannot load telegram session: %w", err)
	}
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, false, fmt.Errorf("cannot create telegram session folder: %w", err)
	}
	sess = &session.Session{
		Messages:      []session.Message{},
		ClosedCleanly: false,
		Folder:        sessionDir,
	}
	if err := sess.Save(); err != nil {
		return nil, false, fmt.Errorf("cannot create telegram session: %w", err)
	}
	return sess, false, nil
}

func runPolling(ctx context.Context, client telegramClient, bridgeCfg *BridgeConfig, state *State, statePath string, agent *runtime.Agent, cfg *config.Config, handler *Handler) error {
	offset, err := drainPendingUpdates(ctx, client)
	if err != nil {
		return fmt.Errorf("cannot drain pending telegram updates: %w", err)
	}
	for {
		updates, err := getUpdatesWithRetry(ctx, client, offset, pollingTimeoutSeconds, pollingRetryDelay)
		if err != nil {
			return fmt.Errorf("telegram polling failed: %w", err)
		}
		for _, update := range updates {
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}
			if update.Message == nil || strings.TrimSpace(update.Message.Text) == "" {
				continue
			}
			if update.Message.Chat.ID != bridgeCfg.AllowedChatID {
				continue
			}
			text := strings.TrimSpace(update.Message.Text)
			if strings.HasPrefix(text, "/") {
				handled, response, err := HandleCommand(ctx, text, agent, cfg, state, statePath)
				if err != nil {
					if _, sendErr := client.SendMessage(ctx, bridgeCfg.AllowedChatID, "error: "+err.Error()); sendErr != nil {
						return fmt.Errorf("telegram command failed: %v; cannot send error to chat: %w", err, sendErr)
					}
					continue
				}
				if handled {
					if response != "" {
						if _, err := client.SendMessage(ctx, bridgeCfg.AllowedChatID, response); err != nil {
							return fmt.Errorf("cannot send telegram command response: %w", err)
						}
					}
					continue
				}
			}

			handler.BeginTurn(ctx)
			err := agent.RunTurn(ctx, text)
			flushErr := handler.FinishTurn()
			if flushErr != nil {
				return flushErr
			}
			if err != nil {
				if _, sendErr := client.SendMessage(ctx, bridgeCfg.AllowedChatID, "error: "+err.Error()); sendErr != nil {
					return fmt.Errorf("telegram turn failed: %v; cannot send error to chat: %w", err, sendErr)
				}
			}
		}
	}
}

func drainPendingUpdates(ctx context.Context, client telegramClient) (int, error) {
	updates, err := getUpdatesWithRetry(ctx, client, 0, startupDrainTimeoutSeconds, pollingRetryDelay)
	if err != nil {
		return 0, err
	}
	return nextOffsetFromUpdates(updates, 0), nil
}

func getUpdatesWithRetry(ctx context.Context, client telegramClient, offset int, timeoutSeconds int, retryDelay time.Duration) ([]Update, error) {
	for {
		updates, err := client.GetUpdates(ctx, offset, timeoutSeconds)
		if err == nil {
			return updates, nil
		}
		if !isRetryablePollingError(err) {
			return nil, err
		}
		if err := waitForPollingRetry(ctx, retryDelay); err != nil {
			return nil, err
		}
	}
}

func isRetryablePollingError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "connection reset by peer") || strings.Contains(message, "broken pipe")
}

func waitForPollingRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func nextOffsetFromUpdates(updates []Update, initial int) int {
	offset := initial
	for _, update := range updates {
		if update.UpdateID >= offset {
			offset = update.UpdateID + 1
		}
	}
	return offset
}

// BotClient is a minimal Telegram Bot API client using long polling and plain text messages.
//
// WHAT:  Sends and receives Telegram bot API requests.
// WHY:   The bridge needs only a small subset of the Telegram API in v1.
// PARAMS: baseURL — bot API prefix; httpClient — HTTP transport.
type BotClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewBotClient creates a Telegram API client for one bot token.
func NewBotClient(token string) *BotClient {
	return &BotClient{
		baseURL:    "https://api.telegram.org/bot" + token,
		httpClient: &http.Client{Timeout: 35 * time.Second},
	}
}

// Update is the subset of Telegram update fields used by the bridge.
type Update struct {
	UpdateID int      `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

// Message is the subset of Telegram message fields used by the bridge.
type Message struct {
	MessageID int    `json:"message_id"`
	Text      string `json:"text,omitempty"`
	Chat      Chat   `json:"chat"`
}

// Chat identifies a Telegram chat.
type Chat struct {
	ID int64 `json:"id"`
}

type updatesResponse struct {
	OK          bool     `json:"ok"`
	Result      []Update `json:"result"`
	Description string   `json:"description,omitempty"`
}

type sendMessageResponse struct {
	OK          bool    `json:"ok"`
	Result      Message `json:"result"`
	Description string  `json:"description,omitempty"`
}

type apiResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
}

// GetUpdates fetches long-poll updates from Telegram.
func (c *BotClient) GetUpdates(ctx context.Context, offset int, timeoutSeconds int) ([]Update, error) {
	values := url.Values{}
	values.Set("offset", fmt.Sprintf("%d", offset))
	values.Set("timeout", fmt.Sprintf("%d", timeoutSeconds))
	body, err := c.doJSONRequest(ctx, "getUpdates", values)
	if err != nil {
		return nil, err
	}
	var resp updatesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("cannot parse getUpdates response: %w", err)
	}
	if !resp.OK {
		return nil, fmt.Errorf("telegram getUpdates failed: %s", resp.Description)
	}
	return resp.Result, nil
}

// SendMessage sends a plain-text Telegram message.
func (c *BotClient) SendMessage(ctx context.Context, chatID int64, text string) (int, error) {
	values := url.Values{}
	values.Set("chat_id", fmt.Sprintf("%d", chatID))
	values.Set("text", text)
	body, err := c.doJSONRequest(ctx, "sendMessage", values)
	if err != nil {
		return 0, err
	}
	var resp sendMessageResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("cannot parse sendMessage response: %w", err)
	}
	if !resp.OK {
		return 0, fmt.Errorf("telegram sendMessage failed: %s", resp.Description)
	}
	return resp.Result.MessageID, nil
}

// EditMessage edits an existing plain-text Telegram message.
func (c *BotClient) EditMessage(ctx context.Context, chatID int64, messageID int, text string) error {
	values := url.Values{}
	values.Set("chat_id", fmt.Sprintf("%d", chatID))
	values.Set("message_id", fmt.Sprintf("%d", messageID))
	values.Set("text", text)
	body, err := c.doJSONRequest(ctx, "editMessageText", values)
	if err != nil {
		return err
	}
	var resp apiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("cannot parse editMessageText response: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("telegram editMessageText failed: %s", resp.Description)
	}
	return nil
}

// SetCommands publishes the Telegram bot command menu.
func (c *BotClient) SetCommands(ctx context.Context, commands []botCommand) error {
	payload, err := json.Marshal(commands)
	if err != nil {
		return fmt.Errorf("cannot marshal setMyCommands request: %w", err)
	}
	values := url.Values{}
	values.Set("commands", string(payload))
	body, err := c.doJSONRequest(ctx, "setMyCommands", values)
	if err != nil {
		return err
	}
	var resp apiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("cannot parse setMyCommands response: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("telegram setMyCommands failed: %s", resp.Description)
	}
	return nil
}

// SendChatAction publishes a transient activity indicator like typing.
func (c *BotClient) SendChatAction(ctx context.Context, chatID int64, action string) error {
	values := url.Values{}
	values.Set("chat_id", fmt.Sprintf("%d", chatID))
	values.Set("action", action)
	body, err := c.doJSONRequest(ctx, "sendChatAction", values)
	if err != nil {
		return err
	}
	var resp apiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("cannot parse sendChatAction response: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("telegram sendChatAction failed: %s", resp.Description)
	}
	return nil
}

func (c *BotClient) doJSONRequest(ctx context.Context, method string, values url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/"+method, bytes.NewBufferString(values.Encode()))
	if err != nil {
		return nil, fmt.Errorf("cannot create telegram %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram %s request failed: %w", method, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read telegram %s response: %w", method, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram %s returned status %d: %s", method, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}
