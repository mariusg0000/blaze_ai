// openai_responses_test.go — tests for the ChatGPT OAuth Responses adapter.
// Verifies request conversion, OAuth headers, streamed text, tool calls, and usage parsing.
// Layer: provider tests. Dependencies: internal/config, internal/session, internal/tools.
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"blazeai/internal/config"
	"blazeai/internal/session"
	"blazeai/internal/tools"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestBuildChatGPTRequestConvertsHistoryAndTools(t *testing.T) {
	call := tools.OpenAIToolCall{
		ID:   "call_1",
		Type: "function",
		Function: tools.OpenAIFunction{
			Name:      "shell",
			Arguments: `{"command":"pwd"}`,
		},
	}
	request, err := buildChatGPTRequest(Request{
		Model: ModelReference{Name: "gpt-5.4"},
		Messages: []session.Message{
			{Role: "system", Content: "system prompt"},
			{Role: "user", Content: "run pwd"},
			{Role: "assistant", ToolCalls: []tools.OpenAIToolCall{call}},
			{Role: "tool", ToolCallID: "call_1", Content: "output"},
		},
		Tools: []tools.OpenAITool{{
			Type:     "function",
			Function: tools.FunctionDef{Name: "shell", Description: "Run a command", Parameters: json.RawMessage(`{"type":"object"}`)},
		}},
	})
	if err != nil {
		t.Fatalf("buildChatGPTRequest() error: %v", err)
	}
	if request.Instructions != "system prompt" {
		t.Errorf("Instructions = %q, want system prompt", request.Instructions)
	}
	if len(request.Input) != 3 {
		t.Fatalf("Input length = %d, want 3", len(request.Input))
	}
	encoded := string(request.Input[1]) + string(request.Input[2])
	if !strings.Contains(encoded, `"type":"function_call"`) || !strings.Contains(encoded, `"call_id":"call_1"`) {
		t.Errorf("tool call input = %s", encoded)
	}
	if len(request.Tools) != 1 || request.Tools[0].Name != "shell" {
		t.Fatalf("Tools = %#v, want one shell tool", request.Tools)
	}
}

func TestStreamChatGPTParsesTextToolCallAndUsage(t *testing.T) {
	var requested *http.Request
	client := &Client{
		Model: "gpt-5.4",
		modelRef: ModelReference{ID: "chatgpt/gpt-5.4", Name: "gpt-5.4", Definition: config.ModelDefinition{
			Protocol:     config.ProtocolOpenAIResponses,
			Capabilities: config.ModelCapabilities{Tools: true, Reasoning: false},
			Responses:    &config.ResponsesVariant{Lite: false},
		}},
		protocol:       openAIResponsesProtocol{},
		PromptCacheKey: "blazeai-session-key",
		AuthType:       config.OAuthAuthType,
		OAuth: &config.OAuthCredential{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			ExpiresAt:    time.Now().Add(time.Hour).UnixMilli(),
			AccountID:    "acct_1",
		},
		HTTP: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requested = request
			argumentsDone := fmt.Sprintf(`data: {"type":"response.function_call_arguments.done","item_id":"fc_1","arguments":%q}`, `{"command":"pwd"}`)
			body := strings.Join([]string{
				"event: response.output_item.added",
				`data: {"type":"response.output_item.added","item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"shell","arguments":""}}`,
				"",
				`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"{\\"command\\":\\"pwd\\"}"}`,
				"",
				argumentsDone,
				"",
				`data: {"type":"response.output_text.delta","delta":"done"}`,
				"",
				`data: {"type":"response.completed","response":{"usage":{"input_tokens":12,"output_tokens":4,"total_tokens":16,"input_tokens_details":{"cached_tokens":9}}}}`,
				"",
			}, "\n")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		})},
	}
	client.SetResponsesIdentity("session-1")

	resp, err := client.Stream(context.Background(), []session.Message{{Role: "user", Content: "run pwd"}}, nil, nil, nil)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	if requested == nil {
		t.Fatal("request was not sent")
	}
	if requested.URL.String() != chatGPTCodexEndpoint {
		t.Errorf("URL = %q, want %q", requested.URL, chatGPTCodexEndpoint)
	}
	var requestBody chatGPTResponsesRequest
	if err := json.NewDecoder(requested.Body).Decode(&requestBody); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	if requestBody.PromptCacheKey != "blazeai-session-key" {
		t.Errorf("PromptCacheKey = %q", requestBody.PromptCacheKey)
	}
	if requestBody.Reasoning.Effort != "" {
		t.Errorf("Reasoning.Effort = %q, want empty for no-suffix model", requestBody.Reasoning.Effort)
	}
	if requestBody.Reasoning.Summary != "" {
		t.Errorf("Reasoning.Summary = %q, want empty for no-suffix model", requestBody.Reasoning.Summary)
	}
	if requestBody.ClientMetadata["session_id"] != "session-1" || requestBody.ClientMetadata["thread_id"] != "session-1" {
		t.Errorf("ClientMetadata = %#v", requestBody.ClientMetadata)
	}
	if requested.Header.Get("session-id") != "session-1" || requested.Header.Get("thread-id") != "session-1" || requested.Header.Get("x-client-request-id") != "session-1" {
		t.Errorf("identity headers are missing: %v", requested.Header)
	}
	if requested.Header.Get("conversation_id") != "blazeai-session-key" || requested.Header.Get("session_id") != "blazeai-session-key" {
		t.Errorf("cache identity headers are missing: %v", requested.Header)
	}
	if requested.Header.Get("OpenAI-Beta") != "responses=experimental" {
		t.Errorf("OpenAI-Beta = %q", requested.Header.Get("OpenAI-Beta"))
	}
	if requested.Header.Get("Authorization") != "Bearer access-token" {
		t.Errorf("Authorization = %q", requested.Header.Get("Authorization"))
	}
	if requested.Header.Get("User-Agent") != chatGPTCodexUserAgent() {
		t.Errorf("User-Agent = %q, want %q", requested.Header.Get("User-Agent"), chatGPTCodexUserAgent())
	}
	if requested.Header.Get("ChatGPT-Account-Id") != "acct_1" {
		t.Errorf("ChatGPT-Account-Id = %q", requested.Header.Get("ChatGPT-Account-Id"))
	}
	if requested.Header.Get("version") != "" {
		t.Errorf("version = %q, want empty for non-lite model", requested.Header.Get("version"))
	}
	if requested.Header.Get(chatGPTCodexLiteHeader) != "" {
		t.Errorf("%s = %q, want empty for non-lite model", chatGPTCodexLiteHeader, requested.Header.Get(chatGPTCodexLiteHeader))
	}
	if resp.Content != "done" {
		t.Errorf("Content = %q, want done", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("ToolCalls = %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ID != "call_1" || resp.ToolCalls[0].Name != "shell" {
		t.Errorf("ToolCall = %#v", resp.ToolCalls[0])
	}
	if resp.Usage == nil || resp.Usage.PromptTokens != 12 || resp.Usage.CompletionTokens != 4 || resp.Usage.CachedTokens != 9 || resp.Usage.CacheStatus != "hit" {
		t.Errorf("Usage = %#v", resp.Usage)
	}
}

func TestStreamChatGPTAddsResponsesLiteHeaderForGPT56(t *testing.T) {
	var requested *http.Request
	client := &Client{
		Model: "gpt-5.6-luna",
		modelRef: ModelReference{ID: "chatgpt/gpt-5.6-luna", Name: "gpt-5.6-luna", Definition: config.ModelDefinition{
			Protocol:     config.ProtocolOpenAIResponses,
			Capabilities: config.ModelCapabilities{Tools: true, Reasoning: false},
			Responses:    &config.ResponsesVariant{Lite: true},
		}},
		protocol: openAIResponsesProtocol{},
		AuthType: config.OAuthAuthType,
		OAuth: &config.OAuthCredential{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			ExpiresAt:    time.Now().Add(time.Hour).UnixMilli(),
			AccountID:    "acct_1",
		},
		HTTP: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requested = request
			body := strings.Join([]string{
				`data: {"type":"response.output_text.delta","delta":"done"}`,
				"",
				`data: {"type":"response.completed","response":{"usage":{"input_tokens":12,"output_tokens":4,"total_tokens":16}}}`,
				"",
			}, "\n")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	_, err := client.Stream(context.Background(), []session.Message{{Role: "user", Content: "hi"}}, nil, nil, nil)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	if requested == nil {
		t.Fatal("request was not sent")
	}
	// Decode request body to verify reasoning is absent for no-suffix model.
	var requestBody chatGPTResponsesRequest
	if err := json.NewDecoder(requested.Body).Decode(&requestBody); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	if requestBody.Reasoning.Effort != "" {
		t.Errorf("Reasoning.Effort = %q, want empty for no-suffix lite model", requestBody.Reasoning.Effort)
	}
	if requestBody.Reasoning.Context != "all_turns" {
		t.Errorf("Reasoning.Context = %q, want all_turns for no-suffix lite model", requestBody.Reasoning.Context)
	}
	if requested.Header.Get(chatGPTCodexLiteHeader) != "true" {
		t.Errorf("%s = %q, want true", chatGPTCodexLiteHeader, requested.Header.Get(chatGPTCodexLiteHeader))
	}
	if requested.Header.Get("version") != chatGPTCodexClientVersion {
		t.Errorf("version = %q, want %q", requested.Header.Get("version"), chatGPTCodexClientVersion)
	}
}

func TestBuildChatGPTLiteRequestAddsAllTurnsReasoningAndStripsImageDetail(t *testing.T) {
	request, err := buildChatGPTLiteRequest(Request{
		Model: ModelReference{Name: "gpt-5.6-luna"},
		Messages: []session.Message{{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "describe"},
				map[string]interface{}{"type": "image_url", "image_url": map[string]interface{}{"url": "data:image/png;base64,abc", "detail": "high"}},
			},
		}},
		Tools: []tools.OpenAITool{{
			Type:     "function",
			Function: tools.FunctionDef{Name: "shell", Description: "Run a command", Parameters: json.RawMessage(`{"type":"object"}`)},
		}, {
			Type:     "function",
			Function: tools.FunctionDef{Name: "load_skill", Description: "Load a skill", Parameters: json.RawMessage(`{"type":"object"}`)},
		}},
	})
	if err != nil {
		t.Fatalf("buildChatGPTRequest() error: %v", err)
	}
	if request.Reasoning.Context != "all_turns" {
		t.Fatalf("Reasoning.Context = %q, want all_turns", request.Reasoning.Context)
	}
	if len(request.Input) < 2 {
		t.Fatalf("Input length = %d, want at least 2", len(request.Input))
	}
	encoded := string(request.Input[0]) + string(request.Input[1])
	if !strings.Contains(encoded, `"type":"additional_tools"`) {
		t.Fatalf("lite input missing additional_tools: %s", encoded)
	}
	if !strings.Contains(encoded, `"type":"input_image"`) {
		t.Fatalf("lite input missing input_image: %s", encoded)
	}
	if strings.Contains(encoded, `"detail"`) {
		t.Fatalf("lite input should strip image detail: %s", encoded)
	}
	if !strings.Contains(encoded, `"text":"describe"`) {
		t.Fatalf("lite input missing text part: %s", encoded)
	}
}

func TestBuildChatGPTRequestNoSuffixOmitsReasoning(t *testing.T) {
	request, err := buildChatGPTRequest(Request{
		Model:    ModelReference{Name: "gpt-5.4"},
		Messages: []session.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("buildChatGPTRequest() error: %v", err)
	}
	if request.Reasoning.Effort != "" {
		t.Errorf("Reasoning.Effort = %q, want empty for no-suffix model", request.Reasoning.Effort)
	}
	if request.Reasoning.Summary != "" {
		t.Errorf("Reasoning.Summary = %q, want empty for no-suffix model", request.Reasoning.Summary)
	}
}

func TestBuildChatGPTLiteRequestNoSuffixUsesAllTurnsContext(t *testing.T) {
	request, err := buildChatGPTLiteRequest(Request{
		Model:    ModelReference{Name: "gpt-5.6-luna"},
		Messages: []session.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("buildChatGPTRequest() error: %v", err)
	}
	if request.Reasoning.Effort != "" {
		t.Errorf("Reasoning.Effort = %q, want empty for no-suffix model", request.Reasoning.Effort)
	}
	if request.Reasoning.Summary != "" {
		t.Errorf("Reasoning.Summary = %q, want empty for no-suffix model", request.Reasoning.Summary)
	}
	if request.Reasoning.Context != "all_turns" {
		t.Errorf("Reasoning.Context = %q, want all_turns for no-suffix lite model", request.Reasoning.Context)
	}
}
