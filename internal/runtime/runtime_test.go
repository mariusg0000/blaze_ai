// runtime_test.go — tests for the agent orchestration loop.
// Uses a mock SSE server to test RunTurn with text and tool call responses.
package runtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"blazeai/internal/config"
	"blazeai/internal/platform"
	"blazeai/internal/session"
)

// mockHandler captures handler calls for verification.
type mockHandler struct {
	content     []string
	toolCalls   []string
	toolResults []string
	usages      []int
}

func (h *mockHandler) OnContent(delta string) { h.content = append(h.content, delta) }
func (h *mockHandler) OnToolCall(name string, args string) {
	h.toolCalls = append(h.toolCalls, name)
}
func (h *mockHandler) OnToolResult(name string, result string) {
	h.toolResults = append(h.toolResults, name+": "+result)
}
func (h *mockHandler) OnUsage(promptTokens int) { h.usages = append(h.usages, promptTokens) }

// setupAgent creates a fully wired Agent with a mock SSE server.
func setupAgent(t *testing.T, handler http.HandlerFunc) (*Agent, *mockHandler, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)

	// Override HOME so config.Save() writes to a temp directory.
	t.Setenv("HOME", t.TempDir())

	cfg := &config.Config{
		Providers: []config.Provider{
			{Name: "test", Endpoint: server.URL, APIKey: "sk-test"},
		},
		Roles:          config.Roles{Default: "test/test-model"},
		Compaction:     config.DefaultCompaction(),
		StripReasoning: config.DefaultStripReasoning(),
	}

	dir := t.TempDir()
	sess, _ := session.CreateInDir(dir)

	// Create minimal prompt files required by the builder.
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	os.WriteFile(filepath.Join(promptsDir, "sysprompt.md"), []byte("system"), 0644)
	os.WriteFile(filepath.Join(promptsDir, "sysprompt.linux.md"), []byte("linux"), 0644)

	h := &mockHandler{}
	agent, err := NewAgent(cfg, sess, platform.Linux, filepath.Join(dir, "skills"), promptsDir, dir, h)
	if err != nil {
		t.Fatalf("NewAgent() error: %v", err)
	}
	return agent, h, server
}

// TestRunTurnTextResponse verifies a turn with a text-only LLM response.
func TestRunTurnTextResponse(t *testing.T) {
	agent, h, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Hello!"}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "data: [DONE]")
		fmt.Fprintln(w)
	})
	defer server.Close()

	err := agent.RunTurn("hi")
	if err != nil {
		t.Fatalf("RunTurn() error: %v", err)
	}
	if len(h.content) == 0 {
		t.Error("OnContent was not called")
	}
	if len(agent.Session.Messages) != 2 {
		t.Errorf("session has %d messages, want 2 (user + assistant)", len(agent.Session.Messages))
	}
}

// TestRunTurnWithToolCall verifies a turn with tool call execution and follow-up.
func TestRunTurnWithToolCall(t *testing.T) {
	callCount := 0
	agent, h, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/event-stream")
		if callCount == 1 {
			// First call: LLM requests a tool call.
			fmt.Fprintln(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"shell","arguments":"{\"command\":\"echo hi\"}"}}]}}]}`)
			fmt.Fprintln(w)
			fmt.Fprintln(w, `data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`)
			fmt.Fprintln(w)
		} else {
			// Second call: LLM responds with text after seeing tool result.
			fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Done"}}]}`)
			fmt.Fprintln(w)
			fmt.Fprintln(w, `data: {"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":20,"completion_tokens":3,"total_tokens":23}}`)
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, "data: [DONE]")
		fmt.Fprintln(w)
	})
	defer server.Close()

	err := agent.RunTurn("run echo hi")
	if err != nil {
		t.Fatalf("RunTurn() error: %v", err)
	}
	if len(h.toolCalls) == 0 {
		t.Error("OnToolCall was not called")
	}
	if len(h.toolResults) == 0 {
		t.Error("OnToolResult was not called")
	}
	if callCount != 2 {
		t.Errorf("LLM was called %d times, want 2 (tool call + follow-up)", callCount)
	}
	if len(h.usages) == 0 {
		t.Error("OnUsage was not called")
	}
}

// TestRunTurnUnknownTool verifies handling of an unknown tool name.
func TestRunTurnUnknownTool(t *testing.T) {
	callCount := 0
	agent, _, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/event-stream")
		if callCount == 1 {
			fmt.Fprintln(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"nonexistent","arguments":"{}"}}]}}]}`)
			fmt.Fprintln(w)
			fmt.Fprintln(w, `data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`)
			fmt.Fprintln(w)
		} else {
			fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"ok"}}]}`)
			fmt.Fprintln(w)
			fmt.Fprintln(w, `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`)
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, "data: [DONE]")
		fmt.Fprintln(w)
	})
	defer server.Close()

	err := agent.RunTurn("call unknown tool")
	if err != nil {
		t.Fatalf("RunTurn() error: %v", err)
	}
}

// TestRunTurnSanitizesIncompleteToolCalls verifies incomplete trailing tool-call rounds
// are removed before the next provider request.
func TestRunTurnSanitizesIncompleteToolCalls(t *testing.T) {
	var lastMessages []map[string]interface{}
	agent, _, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var payload struct {
			Messages []map[string]interface{} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		lastMessages = payload.Messages
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"ok"}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "data: [DONE]")
		fmt.Fprintln(w)
	})
	defer server.Close()

	if err := agent.Session.AppendAll([]session.Message{
		{Role: "user", Content: "old user"},
		{Role: "assistant", Content: "", ToolCalls: []interface{}{map[string]interface{}{"id": "call_1"}}},
	}); err != nil {
		t.Fatalf("AppendAll() failed: %v", err)
	}

	if err := agent.RunTurn("new user"); err != nil {
		t.Fatalf("RunTurn() error: %v", err)
	}

	if len(lastMessages) < 3 {
		t.Fatalf("provider received %d messages, want at least 3", len(lastMessages))
	}
	if got := lastMessages[len(lastMessages)-2]["role"]; got != "user" {
		t.Fatalf("expected last preserved session message to be user, got %v", got)
	}
	if got := lastMessages[len(lastMessages)-1]["role"]; got != "user" {
		t.Fatalf("expected new user message at end of payload, got %v", got)
	}
	if len(agent.Session.Messages) != 3 {
		t.Fatalf("session has %d messages, want 3 after sanitize + response", len(agent.Session.Messages))
	}
}

// TestSetModel verifies model switching and provider recreation.
func TestSetModel(t *testing.T) {
	agent, _, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	err := agent.SetModel("test/test-model")
	if err != nil {
		t.Fatalf("SetModel() error: %v", err)
	}
	if agent.ModelID != "test/test-model" {
		t.Errorf("ModelID = %q, want 'test/test-model'", agent.ModelID)
	}
}

// TestSetModelInvalid verifies error for unknown provider.
func TestSetModelInvalid(t *testing.T) {
	agent, _, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	err := agent.SetModel("ghost/model-x")
	if err == nil {
		t.Fatal("SetModel() expected error for unknown provider, got nil")
	}
}

// TestSetModelBadFormat verifies error for malformed model ID.
func TestSetModelBadFormat(t *testing.T) {
	agent, _, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	err := agent.SetModel("no-slash")
	if err == nil {
		t.Fatal("SetModel() expected error for bad format, got nil")
	}
}

// TestSetWorkDir verifies work folder change.
func TestSetWorkDir(t *testing.T) {
	agent, _, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	newDir := t.TempDir()
	err := agent.SetWorkDir(newDir)
	if err != nil {
		t.Fatalf("SetWorkDir() error: %v", err)
	}
	if agent.WorkDir != newDir {
		t.Errorf("WorkDir = %q, want %q", agent.WorkDir, newDir)
	}
	if agent.Builder.WorkDir != newDir {
		t.Errorf("Builder.WorkDir not updated")
	}
}

// TestSetWorkDirInvalid verifies error for non-existent path.
func TestSetWorkDirInvalid(t *testing.T) {
	agent, _, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	err := agent.SetWorkDir("/nonexistent/path/xyz")
	if err == nil {
		t.Fatal("SetWorkDir() expected error for invalid path, got nil")
	}
}

// TestCloseSession verifies clean close.
func TestCloseSession(t *testing.T) {
	agent, _, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	if err := agent.CloseSession(); err != nil {
		t.Fatalf("CloseSession() error: %v", err)
	}
	if !agent.Session.ClosedCleanly {
		t.Error("session not marked as cleanly closed")
	}
}

// TestNewAgent verifies agent wiring.
func TestNewAgent(t *testing.T) {
	agent, _, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	if agent.Provider == nil {
		t.Error("Provider is nil")
	}
	if agent.Tools.Get("shell") == nil {
		t.Error("shell tool not registered")
	}
	if agent.Tools.Get("replace_block") == nil {
		t.Error("replace_block tool not registered")
	}
	if agent.Active == nil {
		t.Error("Active skills list is nil")
	}
	if agent.Builder == nil {
		t.Error("Builder is nil")
	}
}

// TestNewAgentBadModel verifies error when model ID is invalid.
func TestNewAgentBadModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	cfg := &config.Config{
		Providers: []config.Provider{
			{Name: "test", Endpoint: server.URL, APIKey: "sk-test"},
		},
		Roles: config.Roles{Default: "ghost/test-model"},
	}

	dir := t.TempDir()
	sess, _ := session.CreateInDir(dir)
	_, err := NewAgent(cfg, sess, platform.Linux, "", "", dir, &mockHandler{})
	if err == nil {
		t.Fatal("NewAgent() expected error for missing provider, got nil")
	}
}
