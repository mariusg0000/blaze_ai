// openai_responses.go — ChatGPT Codex Responses API adapter.
// Converts BlazeAI's OpenAI-compatible messages and tools to the Responses
// request shape and parses streamed text, reasoning, tool calls, and usage.
// Layer: external API client. Dependencies: internal/session and internal/tools.
package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"blazeai/internal/session"
	"blazeai/internal/tools"
)

type chatGPTResponsesRequest struct {
	Model             string                 `json:"model"`
	Input             []json.RawMessage      `json:"input"`
	Instructions      string                 `json:"instructions,omitempty"`
	Tools             []chatGPTResponsesTool `json:"tools,omitempty"`
	ToolChoice        string                 `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool                  `json:"parallel_tool_calls,omitempty"`
	Store             bool                   `json:"store"`
	Stream            bool                   `json:"stream"`
	Include           []string               `json:"include,omitempty"`
	Reasoning         chatGPTReasoning       `json:"reasoning,omitempty"`
	Text              chatGPTText            `json:"text,omitempty"`
	ServiceTier       string                 `json:"service_tier,omitempty"`
	PromptCacheKey    string                 `json:"prompt_cache_key,omitempty"`
	ClientMetadata    map[string]string      `json:"client_metadata,omitempty"`
}

func isResponsesLiteModel(model string) bool {
	return strings.HasPrefix(model, "gpt-5.6-")
}

type chatGPTResponsesTool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
	Strict      bool            `json:"strict"`
}

type chatGPTReasoning struct {
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type chatGPTText struct {
	Verbosity string `json:"verbosity,omitempty"`
}

type chatGPTStreamEvent struct {
	Type      string                   `json:"type"`
	Delta     string                   `json:"delta,omitempty"`
	ItemID    string                   `json:"item_id,omitempty"`
	Arguments string                   `json:"arguments,omitempty"`
	Item      chatGPTOutputItem        `json:"item"`
	Response  chatGPTCompletedResponse `json:"response"`
	Error     *chatGPTResponseError    `json:"error,omitempty"`
}

type chatGPTOutputItem struct {
	ID               string `json:"id"`
	Type             string `json:"type"`
	CallID           string `json:"call_id"`
	Name             string `json:"name"`
	Arguments        string `json:"arguments"`
	EncryptedContent string `json:"encrypted_content"`
}

type chatGPTCompletedResponse struct {
	Usage *chatGPTUsage `json:"usage"`
}

type chatGPTUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type chatGPTResponseError struct {
	Message string `json:"message"`
}

// streamChatGPT sends one request through the ChatGPT/Codex Responses endpoint.
//
// WHAT:  Executes a streamed Responses API turn using the OAuth credential.
// WHY:   ChatGPT OAuth is not compatible with the public chat-completions route.
// HOW:   Converts the current session to Responses input, sends bearer/account headers,
// and maps Responses SSE events back to BlazeAI's internal response structure.
// PARAMS: ctx — cancellation; messages — prompt/history; toolDefs — native tools;
// onContent/onReasoning/onPhase — streaming callbacks.
// RETURNS: *Response — accumulated output; error on transport, auth, or parsing failure.
func (c *Client) streamChatGPT(ctx context.Context, messages []session.Message, toolDefs []tools.OpenAITool, onContent func(string), onReasoning func(string), onPhase func(StreamPhase)) (*Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if onPhase != nil {
		onPhase(PhaseConnecting)
	}
	token, err := c.oauthAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	requestBody, err := buildChatGPTRequest(c.Model, messages, toolDefs)
	if err != nil {
		return nil, fmt.Errorf("cannot build ChatGPT request: %w", err)
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal ChatGPT request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, chatGPTCodexBaseURL(c.Endpoint)+"/responses", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("cannot create ChatGPT request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Originator", "codex_cli_rs")
	request.Header.Set("User-Agent", "BlazeAI")
	if c.OAuth != nil && c.OAuth.AccountID != "" {
		request.Header.Set("ChatGPT-Account-Id", c.OAuth.AccountID)
	}
	response, err := c.HTTP.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return &Response{}, ErrAborted
		}
		return nil, fmt.Errorf("ChatGPT request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("ChatGPT returned status %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	if onPhase != nil {
		onPhase(PhaseWaitingFirstEvent)
	}
	return parseChatGPTSSE(ctx, response.Body, onContent, onReasoning, onPhase)
}

func buildChatGPTRequest(model string, messages []session.Message, toolDefs []tools.OpenAITool) (chatGPTResponsesRequest, error) {
	if isResponsesLiteModel(model) {
		return buildChatGPTLiteRequest(model, messages, toolDefs)
	}
	responseTools := buildResponseTools(toolDefs)
	input, instructions, err := buildChatGPTInput(messages)
	if err != nil {
		return chatGPTResponsesRequest{}, err
	}
	return chatGPTResponsesRequest{
		Model:             model,
		Input:             input,
		Instructions:      instructions,
		Tools:             responseTools,
		ToolChoice:        "auto",
		ParallelToolCalls: ptr(true),
		Store:             false,
		Stream:            true,
		Include:           []string{"reasoning.encrypted_content"},
		Reasoning:         chatGPTReasoning{Effort: "medium", Summary: "auto"},
		Text:              chatGPTText{Verbosity: "low"},
	}, nil
}

func buildChatGPTLiteRequest(model string, messages []session.Message, toolDefs []tools.OpenAITool) (chatGPTResponsesRequest, error) {
	input, err := buildChatGPTLiteInput(messages, toolDefs)
	if err != nil {
		return chatGPTResponsesRequest{}, err
	}
	return chatGPTResponsesRequest{
		Model:             model,
		Input:             input,
		ToolChoice:        "auto",
		ParallelToolCalls: ptr(false),
		Store:             false,
		Stream:            true,
		Include:           []string{"reasoning.encrypted_content"},
		Reasoning:         chatGPTReasoning{Effort: "medium", Summary: "auto"},
		Text:              chatGPTText{Verbosity: "low"},
	}, nil
}

func buildChatGPTLiteInput(messages []session.Message, toolDefs []tools.OpenAITool) ([]json.RawMessage, error) {
	input := make([]json.RawMessage, 0, len(messages)+2)
	instructions := make([]string, 0, 1)
	for _, message := range messages {
		switch message.Role {
		case "system":
			if text := messageContentText(message.Content); text != "" {
				instructions = append(instructions, text)
			}
		case "user":
			input = append(input, mustJSON(map[string]interface{}{
				"role":    "user",
				"content": []interface{}{map[string]string{"type": "input_text", "text": messageContentText(message.Content)}},
			}))
		case "assistant":
			if message.ReasoningEncrypted != "" {
				input = append(input, mustJSON(map[string]interface{}{
					"type":              "reasoning",
					"summary":           []interface{}{},
					"encrypted_content": message.ReasoningEncrypted,
				}))
			}
			calls, err := decodeOpenAIToolCalls(message.ToolCalls)
			if err != nil {
				return nil, err
			}
			if len(calls) > 0 {
				for _, call := range calls {
					input = append(input, mustJSON(map[string]interface{}{
						"type":      "function_call",
						"call_id":   call.ID,
						"name":      call.Function.Name,
						"arguments": call.Function.Arguments,
					}))
				}
				continue
			}
			if text := messageContentText(message.Content); text != "" {
				input = append(input, mustJSON(map[string]interface{}{
					"role":    "assistant",
					"content": []interface{}{map[string]string{"type": "output_text", "text": text}},
				}))
			}
		case "tool":
			input = append(input, mustJSON(map[string]interface{}{
				"type":    "function_call_output",
				"call_id": message.ToolCallID,
				"output":  messageContentText(message.Content),
			}))
		}
	}
	prefix := make([]json.RawMessage, 0, 2)
	if len(toolDefs) > 0 {
		responseTools := buildResponseTools(toolDefs)
		prefix = append(prefix, mustJSON(map[string]interface{}{
			"role":  "developer",
			"type":  "additional_tools",
			"tools": responseTools,
		}))
	}
	if len(instructions) > 0 {
		prefix = append(prefix, mustJSON(map[string]interface{}{
			"role":    "developer",
			"content": []interface{}{map[string]string{"type": "input_text", "text": strings.Join(instructions, "\n\n")}},
		}))
	}
	return append(prefix, input...), nil
}

func buildResponseTools(toolDefs []tools.OpenAITool) []chatGPTResponsesTool {
	responseTools := make([]chatGPTResponsesTool, 0, len(toolDefs))
	for _, tool := range toolDefs {
		parameters := tool.Function.Parameters
		if len(parameters) == 0 {
			parameters = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		responseTools = append(responseTools, chatGPTResponsesTool{
			Type:        "function",
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			Parameters:  parameters,
			Strict:      false,
		})
	}
	return responseTools
}

func buildChatGPTInput(messages []session.Message) ([]json.RawMessage, string, error) {
	input := make([]json.RawMessage, 0, len(messages))
	instructions := make([]string, 0, 1)
	for _, message := range messages {
		switch message.Role {
		case "system":
			if text := messageContentText(message.Content); text != "" {
				instructions = append(instructions, text)
			}
		case "user":
			input = append(input, mustJSON(map[string]interface{}{
				"role":    "user",
				"content": []interface{}{map[string]string{"type": "input_text", "text": messageContentText(message.Content)}},
			}))
		case "assistant":
			if message.ReasoningEncrypted != "" {
				input = append(input, mustJSON(map[string]interface{}{
					"type":              "reasoning",
					"summary":           []interface{}{},
					"encrypted_content": message.ReasoningEncrypted,
				}))
			}
			calls, err := decodeOpenAIToolCalls(message.ToolCalls)
			if err != nil {
				return nil, "", err
			}
			if len(calls) > 0 {
				for _, call := range calls {
					input = append(input, mustJSON(map[string]interface{}{
						"type":      "function_call",
						"call_id":   call.ID,
						"name":      call.Function.Name,
						"arguments": call.Function.Arguments,
					}))
				}
				continue
			}
			if text := messageContentText(message.Content); text != "" {
				input = append(input, mustJSON(map[string]interface{}{
					"role":    "assistant",
					"content": []interface{}{map[string]string{"type": "output_text", "text": text}},
				}))
			}
		case "tool":
			input = append(input, mustJSON(map[string]interface{}{
				"type":    "function_call_output",
				"call_id": message.ToolCallID,
				"output":  messageContentText(message.Content),
			}))
		}
	}
	return input, strings.Join(instructions, "\n\n"), nil
}

func ptr(b bool) *bool {
	return &b
}

func decodeOpenAIToolCalls(value interface{}) ([]tools.OpenAIToolCall, error) {
	if value == nil {
		return nil, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("cannot encode tool calls: %w", err)
	}
	var calls []tools.OpenAIToolCall
	if err := json.Unmarshal(raw, &calls); err != nil {
		return nil, fmt.Errorf("cannot decode tool calls: %w", err)
	}
	return calls, nil
}

func messageContentText(content interface{}) string {
	switch value := content.(type) {
	case nil:
		return ""
	case string:
		return value
	case []interface{}:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			if object, ok := item.(map[string]interface{}); ok {
				if text, ok := object["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return fmt.Sprint(value)
	}
}

func mustJSON(value interface{}) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}

func parseChatGPTSSE(ctx context.Context, reader io.ReadCloser, onContent func(string), onReasoning func(string), onPhase func(StreamPhase)) (*Response, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)
	result := &Response{}
	toolCalls := make(map[string]*tools.ToolCall)
	toolOrder := make([]string, 0)
	seenFirstEvent := false
	hiddenReasoningStarted := false
	for {
		if ctx.Err() != nil {
			return result, ErrAborted
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return nil, fmt.Errorf("error reading ChatGPT SSE stream: %w", err)
			}
			finalizeChatGPTToolCalls(result, toolCalls, toolOrder)
			return result, nil
		}
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			finalizeChatGPTToolCalls(result, toolCalls, toolOrder)
			return result, nil
		}
		var event chatGPTStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		if !seenFirstEvent {
			seenFirstEvent = true
			if onPhase != nil {
				onPhase(PhaseStreaming)
			}
		}
		switch event.Type {
		case "response.output_text.delta":
			result.Content += event.Delta
			if onContent != nil {
				onContent(event.Delta)
			}
		case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
			if onReasoning == nil && !hiddenReasoningStarted {
				hiddenReasoningStarted = true
				if onPhase != nil {
					onPhase(PhaseHiddenReasoning)
				}
			}
			result.Reasoning += event.Delta
			if onReasoning != nil {
				onReasoning(event.Delta)
			}
		case "response.output_item.added", "response.output_item.done":
			if event.Item.Type == "reasoning" && event.Item.EncryptedContent != "" {
				result.ReasoningEncrypted = event.Item.EncryptedContent
			}
			if event.Item.Type == "function_call" {
				key := event.Item.ID
				if key == "" {
					key = event.Item.CallID
				}
				call := toolCalls[key]
				if call == nil {
					call = &tools.ToolCall{ID: event.Item.CallID, Name: event.Item.Name}
					toolCalls[key] = call
					toolOrder = append(toolOrder, key)
				}
				if event.Item.CallID != "" {
					call.ID = event.Item.CallID
				}
				if event.Item.Name != "" {
					call.Name = event.Item.Name
				}
				if event.Item.Arguments != "" {
					call.Arguments = json.RawMessage(event.Item.Arguments)
				}
			}
		case "response.function_call_arguments.delta":
			call := toolCalls[event.ItemID]
			if call == nil {
				call = &tools.ToolCall{}
				toolCalls[event.ItemID] = call
				toolOrder = append(toolOrder, event.ItemID)
			}
			call.Arguments = append(call.Arguments, []byte(event.Delta)...)
		case "response.function_call_arguments.done":
			call := toolCalls[event.ItemID]
			if call != nil && event.Arguments != "" {
				call.Arguments = json.RawMessage(event.Arguments)
			}
		case "response.completed":
			if event.Response.Usage != nil {
				usage := event.Response.Usage
				result.Usage = &Usage{PromptTokens: usage.InputTokens, CompletionTokens: usage.OutputTokens, TotalTokens: usage.TotalTokens}
			}
			finalizeChatGPTToolCalls(result, toolCalls, toolOrder)
			return result, nil
		case "response.failed", "error":
			message := "ChatGPT response failed"
			if event.Error != nil && event.Error.Message != "" {
				message = event.Error.Message
			}
			return nil, fmt.Errorf("%s", message)
		}
	}
}

func finalizeChatGPTToolCalls(result *Response, calls map[string]*tools.ToolCall, order []string) {
	result.ToolCalls = result.ToolCalls[:0]
	for _, key := range order {
		call := calls[key]
		if call == nil || call.ID == "" || call.Name == "" || len(call.Arguments) == 0 || !json.Valid(call.Arguments) {
			continue
		}
		result.ToolCalls = append(result.ToolCalls, *call)
	}
}
