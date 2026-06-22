// provider.go — OpenAI-compatible HTTP client with streaming support.
// Sends chat completion requests with streaming, parses SSE responses, extracts tool calls,
// and reports token usage for compaction triggers.
// Layer: external API client. Dependencies: internal/config, internal/session.
package provider

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"blazeai/internal/config"
	"blazeai/internal/session"
	"blazeai/internal/tools"
)

// Usage holds token usage from the provider response.
//
// WHAT:  Captures the usage block from the last assistant response.
// WHY:   Compaction triggers on usage.prompt_tokens reaching maxContextTokens.
// PARAMS: PromptTokens — tokens in the prompt; CompletionTokens — tokens generated; TotalTokens — sum.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Response holds the complete response from a streaming chat completion.
//
// WHAT:  The accumulated result of a streamed LLM response.
// WHY:   The runtime needs the full assistant message, tool calls, and usage after streaming ends.
// PARAMS: Content — accumulated text; ToolCalls — parsed tool calls; Usage — token counts.
type Response struct {
	Content   string
	ToolCalls []tools.ToolCall
	Usage     *Usage
}

// Client communicates with an OpenAI-compatible endpoint.
//
// WHAT:  HTTP client for a single provider endpoint.
// WHY:   The runtime uses one client per provider to send chat completion requests.
// PARAMS: Endpoint — base API URL; APIKey — secret key; Model — bare model name; HTTP — HTTP client.
type Client struct {
	Endpoint string
	APIKey   string
	Model    string
	HTTP     *http.Client
}

// NewClient creates a Client from a config provider and a model identifier.
//
// WHAT:  Builds a provider client from config.
// WHY:   The runtime resolves the provider and model from config to make API calls.
// PARAMS: cfg — the loaded config; modelID — full provider/model_name identifier.
// RETURNS: *Client — configured client; error if provider not found or model invalid.
func NewClient(cfg *config.Config, modelID string) (*Client, error) {
	providerName, modelName := config.SplitModelID(modelID)
	p := cfg.ProviderByName(providerName)
	if p == nil {
		return nil, fmt.Errorf("provider not found: %s", providerName)
	}
	return &Client{
		Endpoint: p.Endpoint,
		APIKey:   p.APIKey,
		Model:    modelName,
		HTTP:     &http.Client{},
	}, nil
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
		HTTP:     &http.Client{},
	}
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
		ids = append(ids, m.ID)
	}
	return ids, nil
}

// chatRequest is the request body sent to the chat completions endpoint.
//
// WHAT:  OpenAI-compatible chat completion request with streaming and tools.
type chatRequest struct {
	Model    string             `json:"model"`
	Messages []session.Message  `json:"messages"`
	Tools    []tools.OpenAITool `json:"tools,omitempty"`
	Stream   bool               `json:"stream"`
}

// streamDelta represents the delta object in a streaming SSE chunk.
//
// WHAT:  The incremental content in one SSE chunk.
// PARAMS: Content — text delta; ToolCalls — tool call deltas.
type streamDelta struct {
	Content   string           `json:"content,omitempty"`
	ToolCalls []streamToolCall `json:"tool_calls,omitempty"`
}

// streamToolCall represents an incremental tool call in a streaming chunk.
//
// WHAT:  Tool call delta with index for assembling multi-chunk tool calls.
// PARAMS: Index — position in the tool calls array; ID — call ID (first chunk only);
//         Function — function name and arguments deltas.
type streamToolCall struct {
	Index    int            `json:"index"`
	ID       string         `json:"id,omitempty"`
	Function streamFunction `json:"function"`
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
//
// PARAMS: Choices — response choices; Usage — token usage (final chunk only).
type streamChunk struct {
	Choices []streamChoice `json:"choices"`
	Usage   *Usage         `json:"usage,omitempty"`
}

// Stream sends a chat completion request with streaming and calls onContent for each text delta.
// Returns the accumulated response with content, tool calls, and usage.
//
// WHAT:  Sends a streaming chat completion request and accumulates the response.
// WHY:   The runtime needs streaming for real-time output and the full response for persistence.
// HOW:   POSTs to /chat/completions with stream=true, reads SSE line by line, parses JSON chunks.
// PARAMS: messages — the full prompt message array; toolDefs — OpenAI tool definitions or nil;
//         onContent — callback called for each text delta (may be nil).
// RETURNS: *Response — accumulated content, tool calls, and usage; error on HTTP or parse failure.
func (c *Client) Stream(messages []session.Message, toolDefs []tools.OpenAITool, onContent func(string)) (*Response, error) {
	reqBody := chatRequest{
		Model:    c.Model,
		Messages: messages,
		Tools:    toolDefs,
		Stream:   true,
	}
	bodyData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal request: %w", err)
	}

	url := strings.TrimRight(c.Endpoint, "/") + "/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyData))
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return parseSSEStream(resp.Body, onContent)
}

// parseSSEStream reads an SSE stream, parses JSON chunks, and accumulates the response.
//
// WHAT:  Parses the Server-Sent Events stream from the provider.
// WHY:   OpenAI-compatible streaming uses SSE with "data: " prefixed JSON lines.
// HOW:   Reads line by line, skips non-data lines, parses JSON, accumulates content and tool calls.
// PARAMS: reader — the response body; onContent — callback for text deltas (may be nil).
// RETURNS: *Response — accumulated response; error on parse failure.
func parseSSEStream(reader io.Reader, onContent func(string)) (*Response, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)

	result := &Response{}
	toolCallMap := make(map[int]*tools.ToolCall)
	var toolCallOrder []int

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.Usage != nil {
			result.Usage = chunk.Usage
		}

		for _, choice := range chunk.Choices {
			delta := choice.Delta

			if delta.Content != "" {
				result.Content += delta.Content
				if onContent != nil {
					onContent(delta.Content)
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
				existing.Arguments = appendRawJSON(existing.Arguments, tc.Function.Arguments)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading SSE stream: %w", err)
	}

	for _, idx := range toolCallOrder {
		result.ToolCalls = append(result.ToolCalls, *toolCallMap[idx])
	}

	return result, nil
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
