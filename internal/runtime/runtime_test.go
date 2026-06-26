// runtime_test.go — tests for the agent orchestration loop.
// Uses a mock SSE server to test RunTurn with text and tool call responses.
package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	onContent   func(string)
	onToolCall  func(string, string)
}

func (h *mockHandler) OnContent(delta string) {
	h.content = append(h.content, delta)
	if h.onContent != nil {
		h.onContent(delta)
	}
}
func (h *mockHandler) OnToolCall(name string, args string) {
	h.toolCalls = append(h.toolCalls, name)
	if h.onToolCall != nil {
		h.onToolCall(name, args)
	}
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
	writePromptFixtures(t, promptsDir)

	h := &mockHandler{}
	agent, err := NewAgent(cfg, sess, platform.Linux, os.DirFS(promptsDir), dir, h)
	if err != nil {
		t.Fatalf("NewAgent() error: %v", err)
	}
	return agent, h, server
}

// writePromptFixtures creates the prompt templates required by runtime prompt assembly.
func writePromptFixtures(t *testing.T, promptsDir string) {
	t.Helper()
	os.WriteFile(filepath.Join(promptsDir, "sysprompt.md"), []byte("# Universal System Prompt\n\nApp home is at {APP_HOME}.\nUnknown var: {UNKNOWN_VAR}.\n\nEach app-home folder has a `README.md` that documents its structure, use, and rules. When a task involves any of these folders, you MUST read its `README.md` first before inspecting or modifying any other file in that folder.\n\n## Tool Discipline\n- Keep relevant loaded skills active across follow-up turns on the same topic or task.\n- Do not unload a skill immediately after one successful action if the user is likely to continue in the same domain.\n- Unload a skill only when the user clearly changes topic or task, or when the loaded skill would interfere with the next turn.\n\n## Active State Rules\n- Only skills listed under `## Active Skills` are active right now. Do not infer current active skills from older `load_skill` or `unload_skill` tool results in the conversation history. If there is no `## Active Skills` section below, then no skills are currently active.\n- Only memories listed under `## Active Memories` are active right now. Do not infer current active memories from older `load_memory` or `unload_memory` tool results in the conversation history. If there is no `## Active Memories` section below, then no memories are currently active.\n\n{OS_PROMPT}\n\n## Host Environment Helpers\n{HOST_HELPERS_ADVISORY}\n\nAvailable helpers:\n{HOST_HELPERS_AVAILABLE}\n\nOptional helpers:\n{HOST_HELPERS_OPTIONAL}\n\n## Skills\nAvailable skills:\n{SKILLS_AVAILABLE}\n\nActive skills:\n{SKILLS_ACTIVE}\n\n## Memories\nAvailable memories:\n{MEMORIES_AVAILABLE}\n\nActive memories:\n{MEMORIES_ACTIVE}\n\n## Project Rules (AGENTS.md)\n{AGENTS_CONTENT}\n"), 0644)
	os.WriteFile(filepath.Join(promptsDir, "sysprompt.linux.md"), []byte("linux"), 0644)
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

	err := agent.RunTurn(context.Background(), "hi")
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

	err := agent.RunTurn(context.Background(), "run echo hi")
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

	err := agent.RunTurn(context.Background(), "call unknown tool")
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

	if err := agent.RunTurn(context.Background(), "new user"); err != nil {
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

// TestRunTurnAbortDuringStreamPersistsPartialHistory verifies stream abort keeps partial content and abort marker.
func TestRunTurnAbortDuringStreamPersistsPartialHistory(t *testing.T) {
	agent, h, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer does not support flushing")
		}
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Hello"}}]}`)
		fmt.Fprintln(w)
		flusher.Flush()
		<-r.Context().Done()
	})
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	h.onContent = func(delta string) {
		if delta == "Hello" {
			cancel()
		}
	}
	err := agent.RunTurn(ctx, "hi")
	if !errors.Is(err, ErrTurnAborted) {
		t.Fatalf("RunTurn() error = %v, want ErrTurnAborted", err)
	}
	if len(agent.Session.Messages) != 3 {
		t.Fatalf("session has %d messages, want 3", len(agent.Session.Messages))
	}
	if got := agent.Session.Messages[1].Role; got != "assistant" {
		t.Fatalf("assistant role = %q, want assistant", got)
	}
	if got := agent.Session.Messages[1].Content; got != "Hello" {
		t.Fatalf("assistant content = %v, want Hello", got)
	}
	if got := agent.Session.Messages[2].Content; got != userAbortMessage {
		t.Fatalf("abort marker = %v, want %q", got, userAbortMessage)
	}
}

// TestRunTurnAbortDuringToolPersistsToolResult verifies active tool abort is preserved in session.
func TestRunTurnAbortDuringToolPersistsToolResult(t *testing.T) {
	callCount := 0
	agent, h, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"shell","arguments":"{\"command\":\"sleep 30\",\"timeout\":60}"}}]}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "data: [DONE]")
		fmt.Fprintln(w)
	})
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	h.onToolCall = func(name string, args string) {
		go func() {
			time.Sleep(200 * time.Millisecond)
			cancel()
		}()
	}
	err := agent.RunTurn(ctx, "run slow command")
	if !errors.Is(err, ErrTurnAborted) {
		t.Fatalf("RunTurn() error = %v, want ErrTurnAborted", err)
	}
	if callCount != 1 {
		t.Fatalf("LLM was called %d times, want 1", callCount)
	}
	if len(agent.Session.Messages) != 4 {
		t.Fatalf("session has %d messages, want 4", len(agent.Session.Messages))
	}
	toolMsg := agent.Session.Messages[2]
	if toolMsg.Role != "tool" {
		t.Fatalf("tool role = %q, want tool", toolMsg.Role)
	}
	content, ok := toolMsg.Content.(string)
	if !ok || !strings.Contains(content, "aborted by user") {
		t.Fatalf("tool content = %v, want aborted by user", toolMsg.Content)
	}
	if got := agent.Session.Messages[3].Content; got != userAbortMessage {
		t.Fatalf("abort marker = %v, want %q", got, userAbortMessage)
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
	if agent.Compactor == nil || agent.Compactor.Provider != agent.Provider {
		t.Fatal("compactor provider not synced with agent provider")
	}
}

// TestSetModelLocal verifies local model switching without global persistence.
func TestSetModelLocal(t *testing.T) {
	agent, _, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	agent.Modes.Modes = []config.Mode{{Name: "default", Model: "test/test-model"}}
	agent.Modes.LastMode = "default"
	agent.CurrentMode = &agent.Modes.Modes[0]
	if err := agent.Modes.Save(); err != nil {
		t.Fatalf("Save() modes failed: %v", err)
	}

	err := agent.SetModelLocal("test/other-model")
	if err != nil {
		t.Fatalf("SetModelLocal() error: %v", err)
	}
	if agent.ModelID != "test/other-model" {
		t.Errorf("ModelID = %q, want 'test/other-model'", agent.ModelID)
	}
	if agent.CurrentMode.Model != "test/test-model" {
		t.Errorf("CurrentMode.Model = %q, want unchanged 'test/test-model'", agent.CurrentMode.Model)
	}
	loaded, err := config.LoadModes("test/test-model")
	if err != nil {
		t.Fatalf("LoadModes() error: %v", err)
	}
	if loaded.Modes[0].Model != "test/test-model" {
		t.Errorf("persisted mode model = %q, want unchanged 'test/test-model'", loaded.Modes[0].Model)
	}
	if agent.Compactor == nil || agent.Compactor.Provider != agent.Provider {
		t.Fatal("compactor provider not synced after local model switch")
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
	t.Setenv("HOME", t.TempDir())

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
	_, err := NewAgent(cfg, sess, platform.Linux, os.DirFS(dir), dir, &mockHandler{})
	if err == nil {
		t.Fatal("NewAgent() expected error for missing provider, got nil")
	}
}

// TestNewAgentWithMode verifies CurrentMode initialization from LastMode.
func TestNewAgentWithMode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	cfg := &config.Config{
		Providers: []config.Provider{
			{Name: "test", Endpoint: server.URL, APIKey: "sk-test"},
		},
		Roles:          config.Roles{Default: "test/model-a"},
		FavoriteModels: []string{"test/model-a", "test/model-b"},
		Compaction:     config.DefaultCompaction(),
		StripReasoning: config.DefaultStripReasoning(),
	}

	// Write modes to modes.json so NewAgent picks them up.
	modes := &config.ModesConfig{
		Modes: []config.Mode{
			{Name: "default", Model: "test/model-a"},
			{Name: "planning", Model: "test/model-b", Directive: "read-only"},
		},
		LastMode: "planning",
	}
	if err := modes.Save(); err != nil {
		t.Fatalf("Save() modes failed: %v", err)
	}

	dir := t.TempDir()
	sess, _ := session.CreateInDir(dir)
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	writePromptFixtures(t, promptsDir)

	agent, err := NewAgent(cfg, sess, platform.Linux, os.DirFS(promptsDir), dir, &mockHandler{})
	if err != nil {
		t.Fatalf("NewAgent() error: %v", err)
	}
	if agent.CurrentMode == nil {
		t.Fatal("CurrentMode is nil")
	}
	if agent.CurrentMode.Name != "planning" {
		t.Errorf("CurrentMode.Name = %q, want 'planning'", agent.CurrentMode.Name)
	}
	if agent.ModelID != "test/model-b" {
		t.Errorf("ModelID = %q, want 'test/model-b'", agent.ModelID)
	}
}

// TestNewAgentWithModeFallbackToFirstMode verifies fallback when LastMode is empty.
func TestNewAgentWithModeFallbackToFirstMode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	cfg := &config.Config{
		Providers: []config.Provider{
			{Name: "test", Endpoint: server.URL, APIKey: "sk-test"},
		},
		Roles:          config.Roles{Default: "test/model-a"},
		FavoriteModels: []string{"test/model-a"},
		Compaction:     config.DefaultCompaction(),
		StripReasoning: config.DefaultStripReasoning(),
	}

	modes := &config.ModesConfig{
		Modes: []config.Mode{
			{Name: "default", Model: "test/model-a"},
		},
	}
	if err := modes.Save(); err != nil {
		t.Fatalf("Save() modes failed: %v", err)
	}

	dir := t.TempDir()
	sess, _ := session.CreateInDir(dir)
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	writePromptFixtures(t, promptsDir)

	agent, err := NewAgent(cfg, sess, platform.Linux, os.DirFS(promptsDir), dir, &mockHandler{})
	if err != nil {
		t.Fatalf("NewAgent() error: %v", err)
	}
	if agent.CurrentMode == nil {
		t.Fatal("CurrentMode is nil, want first mode")
	}
	if agent.CurrentMode.Name != "default" {
		t.Errorf("CurrentMode.Name = %q, want 'default'", agent.CurrentMode.Name)
	}
}

// TestSetMode verifies mode switching and provider recreation.
func TestSetMode(t *testing.T) {
	agent, _, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	agent.Modes.Modes = []config.Mode{
		{Name: "default", Model: "test/test-model"},
		{Name: "planning", Model: "test/test-model", Directive: "read-only"},
	}
	agent.Modes.LastMode = "default"
	agent.CurrentMode = &agent.Modes.Modes[0]
	agent.Modes.Save()

	err := agent.SetMode("planning")
	if err != nil {
		t.Fatalf("SetMode() error: %v", err)
	}
	if agent.CurrentMode.Name != "planning" {
		t.Errorf("CurrentMode.Name = %q, want 'planning'", agent.CurrentMode.Name)
	}
	if agent.Modes.LastMode != "planning" {
		t.Errorf("LastMode = %q, want 'planning'", agent.Modes.LastMode)
	}
	loaded, err := config.LoadModes("test/test-model")
	if err != nil {
		t.Fatalf("LoadModes() error: %v", err)
	}
	if loaded.LastMode != "planning" {
		t.Errorf("persisted LastMode = %q, want 'planning'", loaded.LastMode)
	}
	if agent.Compactor == nil || agent.Compactor.Provider != agent.Provider {
		t.Fatal("compactor provider not synced after mode switch")
	}
}

// TestSetModeNotFound verifies error for non-existent mode.
func TestSetModeNotFound(t *testing.T) {
	agent, _, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	agent.Modes.Modes = []config.Mode{
		{Name: "default", Model: "test/test-model"},
	}

	err := agent.SetMode("nonexistent")
	if err == nil {
		t.Fatal("SetMode() expected error for non-existent mode, got nil")
	}
}

// TestNextMode verifies cyclic mode switching.
func TestNextMode(t *testing.T) {
	agent, _, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	agent.Modes.Modes = []config.Mode{
		{Name: "default", Model: "test/test-model"},
		{Name: "planning", Model: "test/test-model", Directive: "plan"},
		{Name: "quick", Model: "test/test-model", Directive: "fast"},
	}
	agent.Modes.LastMode = "default"
	agent.CurrentMode = &agent.Modes.Modes[0]
	agent.Modes.Save()

	// Cycle: default -> planning
	mode, err := agent.NextMode()
	if err != nil {
		t.Fatalf("NextMode() error: %v", err)
	}
	if mode.Name != "planning" {
		t.Errorf("NextMode() = %q, want 'planning'", mode.Name)
	}

	// Cycle: planning -> quick
	mode, err = agent.NextMode()
	if err != nil {
		t.Fatalf("NextMode() error: %v", err)
	}
	if mode.Name != "quick" {
		t.Errorf("NextMode() = %q, want 'quick'", mode.Name)
	}

	// Cycle: quick -> default (wrap around)
	mode, err = agent.NextMode()
	if err != nil {
		t.Fatalf("NextMode() error: %v", err)
	}
	if mode.Name != "default" {
		t.Errorf("NextMode() = %q, want 'default'", mode.Name)
	}
}

// TestNextMode verifies cyclic mode switching.
// (TestNextModeEmpty removed: NewAgent auto-creates default mode, so empty modes is unreachable.)

// TestSetModelUpdatesMode verifies that SetModel updates CurrentMode.Model.
func TestSetModelUpdatesMode(t *testing.T) {
	agent, _, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	agent.Modes.Modes = []config.Mode{
		{Name: "default", Model: "test/test-model"},
	}
	agent.Modes.LastMode = "default"
	if err := agent.Modes.Save(); err != nil {
		t.Fatalf("Save() modes failed: %v", err)
	}
	agent.CurrentMode = &agent.Modes.Modes[0]

	err := agent.SetModel("test/other-model")
	if err != nil {
		t.Fatalf("SetModel() error: %v", err)
	}
	if agent.CurrentMode.Model != "test/other-model" {
		t.Errorf("CurrentMode.Model = %q, want 'test/other-model'", agent.CurrentMode.Model)
	}
	loaded, err := config.LoadModes("test/test-model")
	if err != nil {
		t.Fatalf("LoadModes() error: %v", err)
	}
	if loaded.Modes[0].Model != "test/other-model" {
		t.Errorf("persisted mode model = %q, want 'test/other-model'", loaded.Modes[0].Model)
	}
	if loaded.LastMode != "default" {
		t.Errorf("persisted LastMode = %q, want 'default'", loaded.LastMode)
	}
}

// TestNewAgentIgnoresLastModelWhenLastModeExists verifies that the active mode wins over legacy last_model.
func TestNewAgentIgnoresLastModelWhenLastModeExists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	cfg := &config.Config{
		Providers:      []config.Provider{{Name: "test", Endpoint: server.URL, APIKey: "sk-test"}},
		Roles:          config.Roles{Default: "test/model-a"},
		FavoriteModels: []string{"test/model-a", "test/model-b", "test/model-c"},
		Compaction:     config.DefaultCompaction(),
		StripReasoning: config.DefaultStripReasoning(),
		LastModel:      "test/model-c",
	}
	modes := &config.ModesConfig{
		Modes: []config.Mode{
			{Name: "default", Model: "test/model-a"},
			{Name: "planning", Model: "test/model-b"},
		},
		LastMode: "planning",
	}
	if err := modes.Save(); err != nil {
		t.Fatalf("Save() modes failed: %v", err)
	}

	dir := t.TempDir()
	sess, _ := session.CreateInDir(dir)
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	writePromptFixtures(t, promptsDir)

	agent, err := NewAgent(cfg, sess, platform.Linux, os.DirFS(promptsDir), dir, &mockHandler{})
	if err != nil {
		t.Fatalf("NewAgent() error: %v", err)
	}
	if agent.CurrentMode == nil || agent.CurrentMode.Name != "planning" {
		t.Fatalf("CurrentMode = %#v, want planning", agent.CurrentMode)
	}
	if agent.ModelID != "test/model-b" {
		t.Errorf("ModelID = %q, want 'test/model-b'", agent.ModelID)
	}
}

// TestInjectDirective verifies directive injection appends to last message in copy.
func TestInjectDirective(t *testing.T) {
	original := []session.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hello"},
	}
	result := injectDirective(original, "be quick")

	// Original must not be mutated.
	if original[1].Content != "hello" {
		t.Errorf("original[1].Content mutated: %v", original[1].Content)
	}
	// Result last message must have directive.
	last, ok := result[1].Content.(string)
	if !ok {
		t.Fatal("result[1].Content is not string")
	}
	if !strings.Contains(last, "[MODE DIRECTIVE]") {
		t.Error("result[1].Content missing [MODE DIRECTIVE]")
	}
	if !strings.Contains(last, "be quick") {
		t.Error("result[1].Content missing directive text")
	}
	if !strings.Contains(last, "hello") {
		t.Error("result[1].Content missing original content")
	}
}

// TestInjectDirectiveEmpty verifies empty messages returns empty.
func TestInjectDirectiveEmpty(t *testing.T) {
	result := injectDirective([]session.Message{}, "directive")
	if len(result) != 0 {
		t.Errorf("injectDirective on empty returned %d messages", len(result))
	}
}

// TestNewAgentAutoCreatesDefaultMode verifies that NewAgent creates a default mode when modes are empty.
func TestNewAgentAutoCreatesDefaultMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	// Set HOME so cfg.Save() writes to a temp dir.
	t.Setenv("HOME", t.TempDir())

	cfg := &config.Config{
		Providers: []config.Provider{
			{Name: "test", Endpoint: server.URL, APIKey: "sk-test"},
		},
		Roles:          config.Roles{Default: "test/test-model"},
		FavoriteModels: []string{"test/test-model"},
		Compaction:     config.DefaultCompaction(),
		StripReasoning: config.DefaultStripReasoning(),
		// No Modes — should be auto-created.
	}

	dir := t.TempDir()
	sess, _ := session.CreateInDir(dir)
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	writePromptFixtures(t, promptsDir)

	agent, err := NewAgent(cfg, sess, platform.Linux, os.DirFS(promptsDir), dir, &mockHandler{})
	if err != nil {
		t.Fatalf("NewAgent() error: %v", err)
	}
	if len(agent.Modes.Modes) != 1 {
		t.Fatalf("Modes = %d, want 1 (auto-created)", len(agent.Modes.Modes))
	}
	if agent.Modes.Modes[0].Name != "default" {
		t.Errorf("Modes[0].Name = %q, want 'default'", agent.Modes.Modes[0].Name)
	}
	if agent.Modes.LastMode != "default" {
		t.Errorf("LastMode = %q, want 'default'", agent.Modes.LastMode)
	}
	if agent.CurrentMode == nil {
		t.Fatal("CurrentMode is nil, want auto-created default mode")
	}
}

// TestListProviderModels verifies that ListProviderModels calls the provider endpoint and returns models.
func TestListProviderModels(t *testing.T) {
	modelsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"data":[{"id":"model-a"},{"id":"model-b"},{"id":"model-c"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer modelsServer.Close()

	agent, _, _ := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {})
	agent.Config.Providers = []config.Provider{
		{Name: "test", Endpoint: modelsServer.URL, APIKey: "sk-test"},
	}

	models, err := agent.ListProviderModels("test")
	if err != nil {
		t.Fatalf("ListProviderModels() error: %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("ListProviderModels() returned %d models, want 3", len(models))
	}
	if models[0] != "model-a" || models[1] != "model-b" || models[2] != "model-c" {
		t.Errorf("ListProviderModels() = %v, want [model-a model-b model-c]", models)
	}
}

// TestListProviderModelsNotFound verifies error for unknown provider.
func TestListProviderModelsNotFound(t *testing.T) {
	agent, _, _ := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {})

	_, err := agent.ListProviderModels("nonexistent")
	if err == nil {
		t.Fatal("ListProviderModels() expected error for unknown provider")
	}
}
