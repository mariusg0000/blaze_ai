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
	"net/http"
	"net/url"
	"strings"
	"time"

	"blazeai/internal/config"
	"blazeai/internal/platform"
	"blazeai/internal/runtime"
	"blazeai/internal/session"
)

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
	sess, resumed, err := openTelegramSession(bridgeCfg.WorkDir)
	if err != nil {
		return err
	}
	agent, err := runtime.NewAgent(cfg, sess, osType, promptsFS, bridgeCfg.WorkDir, nil)
	if err != nil {
		return fmt.Errorf("cannot create telegram agent: %w", err)
	}
	if resumed && agent.Compactor != nil {
		if err := agent.Compactor.RebuildForResume(sess); err != nil {
			return fmt.Errorf("cannot rebuild summaries for telegram resume: %w", err)
		}
	}
	if err := agent.SetModelLocal(state.SelectedModel); err != nil {
		return fmt.Errorf("cannot apply telegram instance model: %w", err)
	}
	client := NewBotClient(bridgeCfg.BotToken)
	handler := NewHandler(client, bridgeCfg.AllowedChatID)
	agent.Handler = handler
	return runPolling(ctx, client, bridgeCfg, state, statePath, agent, cfg, handler)
}

func openTelegramSession(workDir string) (*session.Session, bool, error) {
	sess, err := session.Last(workDir)
	if err == nil {
		return sess, true, nil
	}
	if !errors.Is(err, session.ErrNoSessions) {
		return nil, false, fmt.Errorf("cannot load telegram session: %w", err)
	}
	sess, err = session.Create(workDir)
	if err != nil {
		return nil, false, fmt.Errorf("cannot create telegram session: %w", err)
	}
	return sess, false, nil
}

func runPolling(ctx context.Context, client *BotClient, bridgeCfg *BridgeConfig, state *State, statePath string, agent *runtime.Agent, cfg *config.Config, handler *Handler) error {
	offset := 0
	for {
		updates, err := client.GetUpdates(ctx, offset, 30)
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
