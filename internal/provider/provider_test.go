// provider_test.go — tests for the provider client using a mock SSE server.
package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"blazeai/internal/config"
	"blazeai/internal/session"
)

// mockProvider returns a config with a provider matching the test server.
func mockProvider(t *testing.T, server *httptest.Server) *config.Config {
	return &config.Config{
		Providers: []config.Provider{
			{
				Name:     "test",
				Endpoint: server.URL,
				APIKey:   "sk-test",
			},
		},
		Roles: config.Roles{Default: "test/test-model"},
	}
}

// TestNewClient verifies client creation from config.
func TestNewClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	cfg := mockProvider(t, server)
	client, err := NewClient(cfg, "test/test-model")
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	if client.Model != "test-model" {
		t.Errorf("Model = %q, want 'test-model'", client.Model)
	}
	if client.APIKey != "sk-test" {
		t.Errorf("APIKey = %q, want 'sk-test'", client.APIKey)
	}
}

// TestNewClientProviderNotFound verifies error for missing provider.
func TestNewClientProviderNotFound(t *testing.T) {
	cfg := &config.Config{
		Providers: []config.Provider{
			{Name: "other", Endpoint: "https://example.com", APIKey: "sk-x"},
		},
		Roles: config.Roles{Default: "test/test-model"},
	}
	_, err := NewClient(cfg, "test/test-model")
	if err == nil {
		t.Fatal("NewClient() expected error for missing provider, got nil")
	}
}

// TestStreamContent verifies streaming text content.
func TestStreamContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":2,\"total_tokens\":12}}")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "data: [DONE]")
		fmt.Fprintln(w)
	}))
	defer server.Close()

	cfg := mockProvider(t, server)
	client, err := NewClient(cfg, "test/test-model")
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	var contentDeltas []string
	resp, err := client.Stream([]session.Message{{Role: "user", Content: "hi"}}, nil, func(delta string) {
		contentDeltas = append(contentDeltas, delta)
	})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	if resp.Content != "Hello world" {
		t.Errorf("Content = %q, want 'Hello world'", resp.Content)
	}
	if len(contentDeltas) != 2 {
		t.Errorf("contentDeltas = %d, want 2", len(contentDeltas))
	}
	if resp.Usage == nil {
		t.Fatal("Usage is nil")
	}
	if resp.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", resp.Usage.PromptTokens)
	}
}

// TestStreamToolCall verifies tool call parsing from streaming.
func TestStreamToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_123","function":{"name":"shell","arguments":""}}]}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"comm"}}]}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"and\":\"ls\"}"}}]}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "data: [DONE]")
		fmt.Fprintln(w)
	}))
	defer server.Close()

	cfg := mockProvider(t, server)
	client, err := NewClient(cfg, "test/test-model")
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	resp, err := client.Stream([]session.Message{{Role: "user", Content: "list files"}}, nil, nil)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("ToolCalls = %d, want 1", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "call_123" {
		t.Errorf("ToolCall ID = %q, want 'call_123'", tc.ID)
	}
	if tc.Name != "shell" {
		t.Errorf("ToolCall Name = %q, want 'shell'", tc.Name)
	}
	var args map[string]string
	if err := json.Unmarshal(tc.Arguments, &args); err != nil {
		t.Fatalf("cannot parse arguments: %v", err)
	}
	if args["command"] != "ls" {
		t.Errorf("arguments command = %q, want 'ls'", args["command"])
	}
}

// TestStreamErrorStatus verifies error on non-200 HTTP status.
func TestStreamErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"invalid api key"}`)
	}))
	defer server.Close()

	cfg := mockProvider(t, server)
	client, err := NewClient(cfg, "test/test-model")
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	_, err = client.Stream([]session.Message{{Role: "user", Content: "hi"}}, nil, nil)
	if err == nil {
		t.Fatal("Stream() expected error for 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %q, want it to contain '401'", err.Error())
	}
}

// TestStreamMultipleToolCalls verifies multiple tool calls in one response.
func TestStreamMultipleToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"shell","arguments":"{\"c"}}]}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_2","function":{"name":"load_skill","arguments":"{\"n"}}]}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ommand\":\"ls\"}"}}]}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":1,"function":{"arguments":"ame\":\"memory\"}"}}]}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "data: [DONE]")
		fmt.Fprintln(w)
	}))
	defer server.Close()

	cfg := mockProvider(t, server)
	client, err := NewClient(cfg, "test/test-model")
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	resp, err := client.Stream([]session.Message{{Role: "user", Content: "do two things"}}, nil, nil)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	if len(resp.ToolCalls) != 2 {
		t.Fatalf("ToolCalls = %d, want 2", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "shell" {
		t.Errorf("ToolCalls[0].Name = %q, want 'shell'", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[1].Name != "load_skill" {
		t.Errorf("ToolCalls[1].Name = %q, want 'load_skill'", resp.ToolCalls[1].Name)
	}
}

// TestStreamNoUsage verifies that missing usage results in nil.
func TestStreamNoUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"hi"},"finish_reason":"stop"}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "data: [DONE]")
		fmt.Fprintln(w)
	}))
	defer server.Close()

	cfg := mockProvider(t, server)
	client, err := NewClient(cfg, "test/test-model")
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	resp, err := client.Stream([]session.Message{{Role: "user", Content: "hi"}}, nil, nil)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	if resp.Usage != nil {
		t.Errorf("Usage = %v, want nil", resp.Usage)
	}
}

// TestStreamEmpty verifies that a stream with only [DONE] returns an empty response.
func TestStreamEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, "data: [DONE]")
		fmt.Fprintln(w)
	}))
	defer server.Close()

	cfg := mockProvider(t, server)
	client, err := NewClient(cfg, "test/test-model")
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	resp, err := client.Stream([]session.Message{{Role: "user", Content: "hi"}}, nil, nil)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	if resp.Content != "" {
		t.Errorf("Content = %q, want empty", resp.Content)
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("ToolCalls = %d, want 0", len(resp.ToolCalls))
	}
}

// TestAppendRawJSON verifies argument fragment accumulation.
func TestAppendRawJSON(t *testing.T) {
	var args json.RawMessage
	args = appendRawJSON(args, "")
	if len(args) != 0 {
		t.Errorf("appendRawJSON empty = %s, want empty", args)
	}
	args = appendRawJSON(args, `{"comm`)
	args = appendRawJSON(args, `and":"ls"}`)
	if string(args) != `{"command":"ls"}` {
		t.Errorf("appendRawJSON = %s, want {\"command\":\"ls\"}", args)
	}
}
