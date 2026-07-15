// provider.go — OpenAI-compatible HTTP client with streaming support.
// Sends chat completion requests with streaming, parses SSE responses, extracts tool calls,
// and reports token usage for compaction triggers.
// Layer: external API client. Dependencies: internal/config, internal/session.
package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"blazeai/internal/config"
	"blazeai/internal/reasoning"
	"blazeai/internal/session"
	"blazeai/internal/tools"
	usagepkg "blazeai/internal/usage"
)

// ErrAborted reports that the active provider stream was canceled by the user.
var ErrAborted = errors.New("provider stream aborted")

const (
	providerConnectTimeout        = 10 * time.Second
	providerTLSHandshakeTimeout   = 10 * time.Second
	providerResponseHeaderTimeout = 60 * time.Second
)

var providerStreamIdleTimeout = 180 * time.Second

// Usage aliases the shared normalized usage record for existing provider callers.
type Usage = usagepkg.Usage

// Response holds the complete response from a streaming chat completion.
//
// WHAT:  The accumulated result of a streamed LLM response.
// WHY:   The runtime needs the full assistant message, tool calls, and usage after streaming ends.
// PARAMS: Content — accumulated text; Reasoning — accumulated reasoning text; ToolCalls — parsed tool calls; Usage — token counts.
type Response struct {
	Content            string
	Reasoning          string
	ReasoningEncrypted string
	ToolCalls          []tools.ToolCall
	Usage              *usagepkg.Usage
}

// StreamPhase identifies the current provider streaming phase for UI feedback.
//
// WHAT:  Labels distinct provider request/stream milestones.
// WHY:   Transports can show more truthful waiting states than a generic spinner.
type StreamPhase string

const (
	PhaseConnecting        StreamPhase = "connecting"
	PhaseWaitingFirstEvent StreamPhase = "waiting_first_event"
	PhaseHiddenReasoning   StreamPhase = "hidden_reasoning"
	PhaseStreaming         StreamPhase = "streaming"
)

// StreamPhaseTimeout returns the existing timeout budget for a provider phase.
//
// WHAT: Exposes phase timeout values to transport status displays.
// WHY: The console must show the real provider deadline instead of duplicating constants.
// RETURNS: timeout duration, or zero when the phase has no countdown.
func StreamPhaseTimeout(phase StreamPhase) time.Duration {
	switch phase {
	case PhaseConnecting:
		return providerConnectTimeout
	case PhaseWaitingFirstEvent:
		return providerResponseHeaderTimeout
	case PhaseHiddenReasoning, PhaseStreaming:
		return providerStreamIdleTimeout
	default:
		return 0
	}
}

// Client communicates with an OpenAI-compatible endpoint.
//
// WHAT:  HTTP client for a single provider endpoint.
// WHY:   The runtime uses one client per provider to send chat completion requests.
// PARAMS: Endpoint — base API URL; APIKey — secret key; Model — bare model name;
//
//	HTTP — HTTP client; ReasoningLevel — active reasoning level for reasoning-capable models.
type Client struct {
	Endpoint         string
	APIKey           string
	Model            string
	AuthType         string
	OAuth            *config.OAuthCredential
	OAuthStore       func(config.OAuthCredential) error
	HTTP             *http.Client
	PromptCacheKey   string
	RawCaptureFolder string
	SessionID        string
	ThreadID         string
	WindowID         string
	InstallationID   string
	ReasoningLevel   string
	oauthMu          sync.Mutex
}

// NewClient creates a Client from a config provider and a model identifier.
//
// WHAT:  Builds a provider client from config.
// WHY:   The runtime resolves the provider and model from config to make API calls.
// HOW:   Parses the full modelID with reasoning.ParseModelSpec to extract the bare
//
//	model ID and optional reasoning level suffix. Sets client.Model to the bare
//	model name and client.ReasoningLevel to the parsed level.
//
// PARAMS: cfg — the loaded config; modelID — full provider/model_name identifier,
//
//	optionally suffixed with |reasoning_level.
//
// RETURNS: *Client — configured client; error if provider not found or model invalid.
func NewClient(cfg *config.Config, modelID string) (*Client, error) {
	spec, err := reasoning.ParseModelSpec(modelID)
	if err != nil {
		return nil, err
	}
	providerName, modelName := config.SplitModelID(spec.ModelID)
	client, err := NewClientForProvider(cfg, providerName)
	if err != nil {
		return nil, err
	}
	client.Model = modelName
	client.ReasoningLevel = spec.ReasoningLevel
	return client, nil
}

// SetPromptCacheKey assigns the stable cache identity used by Responses requests.
//
// WHAT: Sets a session-scoped prompt cache key on the provider client.
// WHY:  The Responses API groups reusable prompt prefixes by this key.
// PARAMS: key — stable, non-sensitive session identifier.
func (c *Client) SetPromptCacheKey(key string) {
	c.PromptCacheKey = strings.TrimSpace(key)
}

// SetResponsesIdentity assigns stable Codex-compatible request identities.
//
// WHAT: Sets session, thread, and window identifiers for Responses requests.
// WHY:  ChatGPT's Codex route uses these values for request correlation and sticky routing.
// PARAMS: identity — stable non-sensitive identifier for the BlazeAI session.
func (c *Client) SetResponsesIdentity(identity string) {
	identity = strings.TrimSpace(identity)
	c.SessionID = identity
	c.ThreadID = identity
	c.WindowID = identity
	c.InstallationID = "blazeai"
}

// NewClientForProvider creates a client for provider-level operations such as model listing.
//
// WHAT:  Builds a client with the provider's API key or OAuth credential.
// WHY:   Console provider integration must list OAuth models without a model ID first.
// PARAMS: cfg — runtime config; providerName — configured provider identifier.
// RETURNS: *Client — provider client; error if the provider is missing.
func NewClientForProvider(cfg *config.Config, providerName string) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	p := cfg.ProviderByName(providerName)
	if p == nil {
		return nil, fmt.Errorf("provider not found: %s", providerName)
	}
	client := &Client{
		Endpoint: p.Endpoint,
		APIKey:   p.APIKey,
		AuthType: p.AuthType,
		HTTP:     newHTTPClient(),
	}
	if p.OAuth != nil {
		credential := *p.OAuth
		client.OAuth = &credential
		client.OAuthStore = func(updated config.OAuthCredential) error {
			current := cfg.ProviderByName(providerName)
			if current == nil {
				return fmt.Errorf("provider not found while saving OAuth credential: %s", providerName)
			}
			current.OAuth = &updated
			return cfg.Save()
		}
	}
	return client, nil
}

// NewClientRaw creates a Client directly from endpoint and API key without config.
// Used during first-run setup when config does not exist yet.
//
// WHAT:  Builds a provider client from raw endpoint and key.
// WHY:   First-run setup needs to call the provider API before config is written.
// PARAMS: endpoint — base API URL; apiKey — secret key.
// RETURNS: *Client — configured client.
func NewClientRaw(endpoint, apiKey string) *Client {
	return &Client{
		Endpoint: endpoint,
		APIKey:   apiKey,
		HTTP:     newHTTPClient(),
	}
}

// NewOAuthClientRaw creates a ChatGPT OAuth client before a config exists.
// Used during first-run setup to discover the account's live Codex models.
func NewOAuthClientRaw(endpoint string, credential config.OAuthCredential) *Client {
	return &Client{
		Endpoint: endpoint,
		AuthType: config.OAuthAuthType,
		OAuth:    &credential,
		HTTP:     newHTTPClient(),
	}
}

// newHTTPClient builds the provider HTTP client with startup timeouts that are
// safe for long-running SSE streams.
//
// WHAT:  Creates the HTTP client used for provider API calls.
// WHY:   Connect, TLS, and response-header phases must fail explicitly instead
//
//	of hanging forever, while SSE body streaming must remain uncapped in total duration.
//
// HOW:   Uses a custom Transport with bounded connect, handshake, and header waits.
// RETURNS: *http.Client — configured client.
func newHTTPClient() *http.Client {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: providerConnectTimeout,
		}).DialContext,
		TLSHandshakeTimeout:   providerTLSHandshakeTimeout,
		ResponseHeaderTimeout: providerResponseHeaderTimeout,
	}
	return &http.Client{Transport: transport}
}

// modelEntry represents one model in the provider's model list response.
type modelEntry struct {
	ID string `json:"id"`
}

// modelsResponse is the JSON response from GET /models.
type modelsResponse struct {
	Data []modelEntry `json:"data"`
}

// ListModels retrieves the list of available model IDs from the provider endpoint.
//
// WHAT:  Fetches the model list from the provider's /models endpoint.
// WHY:   First-run setup presents available models to the user for selection.
// HOW:   GET {endpoint}/models with Authorization header, parse JSON response.
// RETURNS: []string — sorted list of model IDs; error if the request or parse fails.
func (c *Client) ListModels() ([]string, error) {
	if c.AuthType == config.OAuthAuthType {
		return c.listChatGPTModels()
	}
	url := strings.TrimRight(c.Endpoint, "/") + "/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("cannot parse models response: %w", err)
	}

	ids := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		id := m.ID
		id = strings.TrimPrefix(id, "models/")
		ids = append(ids, id)
	}
	return ids, nil
}

type chatGPTModelEntry struct {
	Slug string `json:"slug"`
}

type chatGPTModelsResponse struct {
	Models []chatGPTModelEntry `json:"models"`
}

// listChatGPTModels retrieves the account-scoped Codex model catalog.
//
// WHAT: Fetches model slugs from the ChatGPT Codex /models endpoint.
// WHY: The account's entitlement, not a local catalog, determines which model IDs work.
// HOW: Sends the OAuth bearer token and account ID, then parses models[].slug.
func (c *Client) listChatGPTModels() ([]string, error) {
	token, err := c.oauthAccessToken(context.Background())
	if err != nil {
		return nil, err
	}

	endpoint := chatGPTCodexBaseURL(c.Endpoint) + "/models?client_version=" + chatGPTCodexClientVersion
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create ChatGPT models request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Originator", "codex_cli_rs")
	req.Header.Set("User-Agent", chatGPTCodexUserAgent())
	if c.OAuth != nil && c.OAuth.AccountID != "" {
		req.Header.Set("ChatGPT-Account-ID", c.OAuth.AccountID)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ChatGPT models request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ChatGPT models returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result chatGPTModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("cannot parse ChatGPT models response: %w", err)
	}
	models := make([]string, 0, len(result.Models))
	for _, model := range result.Models {
		if model.Slug != "" {
			models = append(models, model.Slug)
		}
	}
	return models, nil
}

// chatRequest is the request body sent to the chat completions endpoint.
//
// WHAT:  OpenAI-compatible chat completion request with streaming and tools.
// PARAMS: ReasoningEffort — optional reasoning level ("", "low", "medium", etc.).
type chatRequest struct {
	Model           string             `json:"model"`
	Messages        []session.Message  `json:"messages"`
	Tools           []tools.OpenAITool `json:"tools,omitempty"`
	Stream          bool               `json:"stream"`
	StreamOptions   *streamOptions     `json:"stream_options,omitempty"`
	ReasoningEffort string             `json:"reasoning_effort,omitempty"`
}

// streamOptions controls what additional data is included in the streaming response.
//
// WHAT:  Configures the streaming response to include token usage.
// WHY:   Compaction triggers on provider-reported prompt_tokens; usage is only sent
//
//	in the stream when include_usage is set to true.
type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// streamDelta represents the delta object in a streaming SSE chunk.
//
// WHAT:  The incremental content in one SSE chunk.
// PARAMS: Content — text delta; ReasoningContent — reasoning text delta; ToolCalls — tool call deltas.
type streamDelta struct {
	Content          string           `json:"content,omitempty"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
	ToolCalls        []streamToolCall `json:"tool_calls,omitempty"`
}

// streamToolCall represents an incremental tool call in a streaming chunk.
//
// WHAT:  Tool call delta with index for assembling multi-chunk tool calls.
// PARAMS: Index — position in the tool calls array; ID — call ID (first chunk only);
//
//	Function — function name and arguments deltas;
//	ExtraContent — provider-specific extensions (e.g. Google Gemini thought_signature).
type streamToolCall struct {
	Index        int            `json:"index"`
	ID           string         `json:"id,omitempty"`
	Function     streamFunction `json:"function"`
	ExtraContent *extraContent  `json:"extra_content,omitempty"`
}

// extraContent holds provider-specific extensions in tool calls.
type extraContent struct {
	Google *googleExtra `json:"google,omitempty"`
}

// googleExtra holds Google-specific fields from the OpenAI-compatible response.
type googleExtra struct {
	ThoughtSignature string `json:"thought_signature,omitempty"`
}

// streamFunction holds the function name and arguments deltas.
//
// PARAMS: Name — function name (first chunk); Arguments — argument fragments.
type streamFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// streamChoice represents one choice in a streaming SSE chunk.
//
// PARAMS: Delta — the incremental content; FinishReason — why the stream ended.
type streamChoice struct {
	Delta        streamDelta `json:"delta"`
	FinishReason string      `json:"finish_reason,omitempty"`
}

// streamChunk represents one SSE data chunk from the streaming response.
type streamChunk struct {
	Choices []streamChoice  `json:"choices"`
	Usage   json.RawMessage `json:"usage,omitempty"`
}

// Stream sends a chat completion request with streaming and calls onContent for each text delta.
// Returns the accumulated response with content, tool calls, and usage.
//
// WHAT:  Sends a streaming chat completion request and accumulates the response.
// WHY:   The runtime needs streaming for real-time output and the full response for persistence.
// HOW:   POSTs to /chat/completions with stream=true, reads SSE line by line, parses JSON chunks.
// PARAMS: messages — the full prompt message array; toolDefs — OpenAI tool definitions or nil;
//
//	onContent — callback called for each text delta (may be nil);
//	onReasoning — callback called for each reasoning delta (may be nil).
//
// RETURNS: *Response — accumulated content, tool calls, and usage; error on HTTP or parse failure.
func (c *Client) Stream(ctx context.Context, messages []session.Message, toolDefs []tools.OpenAITool, onContent func(string), onReasoning func(string)) (*Response, error) {
	return c.StreamWithPhase(ctx, messages, toolDefs, onContent, onReasoning, nil)
}

// StreamWithPhase sends a chat completion request with streaming and reports high-level
// provider phases for transports that want richer waiting indicators.
func (c *Client) StreamWithPhase(ctx context.Context, messages []session.Message, toolDefs []tools.OpenAITool, onContent func(string), onReasoning func(string), onPhase func(StreamPhase)) (*Response, error) {
	if c.AuthType == config.OAuthAuthType {
		return c.streamChatGPT(ctx, messages, toolDefs, onContent, onReasoning, onPhase)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if c.RawCaptureFolder != "" {
		_ = session.ResetRawJSON(c.RawCaptureFolder, "llm-raw.json")
	}
	if onPhase != nil {
		onPhase(PhaseConnecting)
	}
	reqBody := chatRequest{
		Model:         c.Model,
		Messages:      messages,
		Tools:         toolDefs,
		Stream:        true,
		StreamOptions: &streamOptions{IncludeUsage: true},
	}
	// Apply reasoning level if configured for this model.
	if c.ReasoningLevel != "" {
		fragment, err := reasoning.Normalize("openai_chat", c.Model, c.ReasoningLevel)
		if err != nil {
			return nil, fmt.Errorf("reasoning level error: %w", err)
		}
		if effort, ok := fragment["reasoning_effort"].(string); ok {
			reqBody.ReasoningEffort = effort
		}
	}
	bodyData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal request: %w", err)
	}

	url := strings.TrimRight(c.Endpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyData))
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return &Response{}, ErrAborted
		}
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	if onPhase != nil {
		onPhase(PhaseWaitingFirstEvent)
	}

	return parseSSEStream(ctx, resp.Body, onContent, onReasoning, onPhase, c.RawCaptureFolder)
}

// oauthAccessToken returns a valid ChatGPT access token, refreshing and persisting it when needed.
//
// WHAT:  Resolves the bearer token used by the private ChatGPT Codex endpoint.
// WHY:   Access tokens expire while refresh tokens are intended to persist.
// HOW:   Serializes refreshes so concurrent requests do not rotate the credential repeatedly.
// PARAMS: ctx — request cancellation context.
// RETURNS: string — valid access token; error if refresh fails.
func (c *Client) oauthAccessToken(ctx context.Context) (string, error) {
	c.oauthMu.Lock()
	defer c.oauthMu.Unlock()
	if c.OAuth == nil || c.OAuth.RefreshToken == "" {
		return "", fmt.Errorf("ChatGPT OAuth is not connected")
	}
	if c.OAuth.AccessToken != "" && c.OAuth.ExpiresAt > time.Now().Add(30*time.Second).UnixMilli() {
		return c.OAuth.AccessToken, nil
	}
	updated, err := refreshChatGPTCredential(ctx, *c.OAuth)
	if err != nil {
		return "", err
	}
	*c.OAuth = updated
	if c.OAuthStore != nil {
		if err := c.OAuthStore(updated); err != nil {
			return "", fmt.Errorf("cannot persist refreshed ChatGPT credential: %w", err)
		}
	}
	return c.OAuth.AccessToken, nil
}

// parseSSEStream reads an SSE stream, parses JSON chunks, and accumulates the response.
//
// WHAT:  Parses the Server-Sent Events stream from the provider.
// WHY:   OpenAI-compatible streaming uses SSE with "data: " prefixed JSON lines.
// HOW:   Reads line by line, skips non-data lines, parses JSON, accumulates content and tool calls.
// PARAMS: reader — the response body; onContent — callback for text deltas (may be nil);
//
//	onReasoning — callback for reasoning deltas (may be nil).
//
// RETURNS: *Response — accumulated response; error on parse failure.
func parseSSEStream(ctx context.Context, reader io.ReadCloser, onContent func(string), onReasoning func(string), onPhase func(StreamPhase), captureFolder string) (*Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)
	type scanResult struct {
		line string
		err  error
	}
	scanCh := make(chan scanResult, 1)
	go func() {
		for scanner.Scan() {
			scanCh <- scanResult{line: scanner.Text()}
		}
		scanCh <- scanResult{err: scanner.Err()}
	}()

	result := &Response{}
	toolCallMap := make(map[int]*tools.ToolCall)
	var toolCallOrder []int
	seenFirstEvent := false
	hiddenReasoningStarted := false
	idleTimer := time.NewTimer(providerStreamIdleTimeout)
	defer idleTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = reader.Close()
			finalizeToolCalls(result, toolCallMap, toolCallOrder)
			return result, ErrAborted
		case <-idleTimer.C:
			_ = reader.Close()
			return nil, fmt.Errorf("provider stream idle timeout after %s with no events", providerStreamIdleTimeout)
		case scan := <-scanCh:
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(providerStreamIdleTimeout)
			if scan.err != nil {
				if ctx.Err() != nil {
					finalizeToolCalls(result, toolCallMap, toolCallOrder)
					return result, ErrAborted
				}
				return nil, fmt.Errorf("error reading SSE stream: %w", scan.err)
			}
			if scan.line == "" {
				continue
			}
			line := scan.line
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				finalizeToolCalls(result, toolCallMap, toolCallOrder)
				return result, nil
			}
			if captureFolder != "" {
				_ = session.AppendRawJSON(captureFolder, "llm-raw.json", []byte(data))
			}

			var chunk streamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if !seenFirstEvent {
				seenFirstEvent = true
				if onPhase != nil {
					onPhase(PhaseStreaming)
				}
			}

			if normalized, ok := usagepkg.Extract([]byte(data)); ok {
				result.Usage = normalized
			}

			for _, choice := range chunk.Choices {
				delta := choice.Delta

				if delta.Content != "" {
					result.Content += delta.Content
					if onContent != nil {
						onContent(delta.Content)
					}
				}

				if delta.ReasoningContent != "" {
					if onReasoning == nil && !hiddenReasoningStarted {
						hiddenReasoningStarted = true
						if onPhase != nil {
							onPhase(PhaseHiddenReasoning)
						}
					}
					result.Reasoning += delta.ReasoningContent
					if onReasoning != nil {
						onReasoning(delta.ReasoningContent)
					}
				}

				for _, tc := range delta.ToolCalls {
					existing, ok := toolCallMap[tc.Index]
					if !ok {
						existing = &tools.ToolCall{
							ID:   tc.ID,
							Name: tc.Function.Name,
						}
						toolCallMap[tc.Index] = existing
						toolCallOrder = append(toolCallOrder, tc.Index)
					}
					if tc.ID != "" && existing.ID == "" {
						existing.ID = tc.ID
					}
					if tc.Function.Name != "" && existing.Name == "" {
						existing.Name = tc.Function.Name
					}
					if tc.ExtraContent != nil && tc.ExtraContent.Google != nil &&
						tc.ExtraContent.Google.ThoughtSignature != "" && existing.ThoughtSignature == "" {
						existing.ThoughtSignature = tc.ExtraContent.Google.ThoughtSignature
					}
					existing.Arguments = appendRawJSON(existing.Arguments, tc.Function.Arguments)
				}
			}
		}
	}
}

// finalizeToolCalls appends only complete tool calls assembled from the SSE stream.
func finalizeToolCalls(result *Response, toolCallMap map[int]*tools.ToolCall, toolCallOrder []int) {
	result.ToolCalls = result.ToolCalls[:0]
	for _, idx := range toolCallOrder {
		tc := toolCallMap[idx]
		if tc == nil || tc.ID == "" || tc.Name == "" || len(tc.Arguments) == 0 || !json.Valid(tc.Arguments) {
			continue
		}
		result.ToolCalls = append(result.ToolCalls, *tc)
	}
}

// appendRawJSON appends a string fragment to a raw JSON byte slice.
// Used to accumulate tool call argument fragments across streaming chunks.
//
// WHAT:  Concatenates argument fragments from streaming chunks.
// WHY:   Tool call arguments arrive in pieces across multiple SSE chunks.
// PARAMS: existing — accumulated bytes so far; fragment — new fragment from this chunk.
// RETURNS: json.RawMessage — the combined argument bytes.
func appendRawJSON(existing json.RawMessage, fragment string) json.RawMessage {
	if fragment == "" {
		return existing
	}
	if len(existing) == 0 {
		return json.RawMessage(fragment)
	}
	return append(existing, []byte(fragment)...)
}
