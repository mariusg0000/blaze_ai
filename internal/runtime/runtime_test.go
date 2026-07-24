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
	"testing/fstest"
	"time"

	"blazeai/internal/agents"
	"blazeai/internal/config"
	"blazeai/internal/platform"
	"blazeai/internal/session"
)

// mockHandler captures handler calls for verification.
type mockHandler struct {
	content            []string
	toolCalls          []string
	toolResults        []string
	usages             []int
	systemMsgs         []string
	maintenanceCalls   []string
	maintenanceResults []string
	onContent          func(string)
	onToolCall         func(string, string)
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
func (h *mockHandler) OnUsage(promptTokens, cachedTokens, uncachedTokens int) {
	h.usages = append(h.usages, promptTokens)
}
func (h *mockHandler) OnSystem(message string) { h.systemMsgs = append(h.systemMsgs, message) }
func (h *mockHandler) OnMaintenanceCall(name string, args string) {
	h.maintenanceCalls = append(h.maintenanceCalls, name+": "+args)
}
func (h *mockHandler) OnMaintenanceResult(name string, result string) {
	h.maintenanceResults = append(h.maintenanceResults, name+": "+result)
}
func (h *mockHandler) RequestSudoApproval(context.Context, string) (bool, string, error) {
	return false, "", nil
}

// writeAgentFixtures creates a minimal interactive agent definition in the temp HOME's agents dir.
func writeAgentFixtures(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	agentsHome := filepath.Join(home, "blazeai")
	agentsDir := filepath.Join(agentsHome, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("cannot create agents dir: %v", err)
	}
	def := "---\nname: default\ndescription: Test agent\ntype: interactive\nmodel: test/test-model\ntools:\n  - shell\n  - read_file\n  - write_file\n  - replace_block\n  - ask_a_friend\n  - analyze_image\n  - load_skill\n  - task_write\n  - task_read\n---\nDefault test agent instructions.\n"
	if err := os.WriteFile(filepath.Join(agentsDir, "default.md"), []byte(def), 0600); err != nil {
		t.Fatalf("cannot write agent definition: %v", err)
	}
}

// setupAgent creates a fully wired Agent with a mock SSE server.
func setupAgent(t *testing.T, handler http.HandlerFunc) (*Agent, *mockHandler, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)

	// Override HOME so config.Save() writes to a temp directory.
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Create interactive agent definition in the agent dir.
	agentsHome := filepath.Join(home, "blazeai")
	agentsDir := filepath.Join(agentsHome, "agents")
	os.MkdirAll(agentsDir, 0755)
	def := "---\nname: default\ndescription: Test agent\ntype: interactive\nmodel: test/test-model\ntools:\n  - shell\n  - read_file\n  - write_file\n  - replace_block\n  - ask_a_friend\n  - analyze_image\n  - load_skill\n  - task_write\n  - task_read\n---\nDefault test agent instructions.\n"
	if err := os.WriteFile(filepath.Join(agentsDir, "default.md"), []byte(def), 0600); err != nil {
		t.Fatalf("cannot write agent definition: %v", err)
	}

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
	agent, err := NewAgent(cfg, sess, platform.Linux, os.DirFS(promptsDir), fstest.MapFS{}, dir, h, "console")
	if err != nil {
		t.Fatalf("NewAgent() error: %v", err)
	}
	return agent, h, server
}

// writePromptFixtures creates the prompt templates required by runtime prompt assembly.
func writePromptFixtures(t *testing.T, promptsDir string) {
	t.Helper()
	content := "# Universal System Prompt\n\nApp home is at {APP_HOME}.\nUnknown var: {UNKNOWN_VAR}.\n\nEach app-home folder has a `README.md` that documents its structure, use, and rules. When a task involves any of these folders, you MUST read its `README.md` first before inspecting or modifying any other file in that folder.\n\n{OS_PROMPT}\n\n## Transport\n{TRANSPORT_PROMPT}\n\n{TRANSPORT_CONTEXT}\n\n## Host Environment Helpers\n{HOST_HELPERS_ADVISORY}\n\nAvailable helpers:\n{HOST_HELPERS_AVAILABLE}\n\nOptional helpers:\n{HOST_HELPERS_OPTIONAL}\n\n## Skills\nBefore performing any task, scan available skill descriptions. If a domain or system mentioned in the request appears in a skill's description, you MUST load that skill first. Do not act on an unfamiliar domain without loading the relevant skill.\n\nAvailable skills:\n{SKILLS_AVAILABLE}\n\n## Project Rules (AGENTS.md)\n{AGENTS_CONTENT}\n"
	os.WriteFile(filepath.Join(promptsDir, "sysprompt.md"), []byte(content), 0644)
	os.WriteFile(filepath.Join(promptsDir, "sysprompt.linux.md"), []byte("linux"), 0644)
	os.WriteFile(filepath.Join(promptsDir, "transport.console.md"), []byte("console transport"), 0644)
	os.WriteFile(filepath.Join(promptsDir, "transport.telegram.md"), []byte("telegram transport"), 0644)
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

// TestRunTurnDebugPromptGate verifies prompt.json is controlled only by DebugPrompt.
func TestRunTurnDebugPromptGate(t *testing.T) {
	var providerMessages []map[string]interface{}
	response := func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []map[string]interface{} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode provider request: %v", err)
		}
		providerMessages = payload.Messages
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"ok"}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "data: [DONE]")
		fmt.Fprintln(w)
	}
	for _, tc := range []struct {
		name     string
		debug    bool
		wantFile bool
	}{
		{name: "omitted", wantFile: false},
		{name: "enabled", debug: true, wantFile: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agent, _, server := setupAgent(t, response)
			defer server.Close()
			agent.Config.DebugPrompt = tc.debug
			if tc.debug {
				// Set a directive on the current agent definition for testing.
				directive := "use concise mode"
				agent.CurrentAgent = &agents.Definition{
					Name:         "default",
					Type:         agents.TypeInteractive,
					Model:        agent.ModelID,
					Directive:    directive,
					ToolNames:    []string{"shell"},
					Instructions: "test instructions",
				}
			}
			if err := agent.RunTurn(context.Background(), "debug gate"); err != nil {
				t.Fatalf("RunTurn() error: %v", err)
			}
			path := filepath.Join(agent.Session.Folder, "prompt.json")
			data, err := os.ReadFile(path)
			if !tc.wantFile {
				if !os.IsNotExist(err) {
					t.Fatalf("prompt.json error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ReadFile(prompt.json) error: %v", err)
			}
			if len(data) == 0 {
				t.Fatal("prompt.json is empty")
			}
			if !strings.Contains(string(data), "debug gate") {
				t.Fatalf("prompt.json does not contain built user message: %q", data)
			}
			if !strings.Contains(string(data), "use concise mode") {
				t.Fatalf("prompt.json does not contain agent directive: %q", data)
			}
			providerData, err := json.Marshal(providerMessages)
			if err != nil {
				t.Fatalf("marshal provider messages: %v", err)
			}
			if !strings.Contains(string(providerData), "use concise mode") {
				t.Fatalf("provider-bound messages do not contain agent directive: %s", providerData)
			}
		})
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

	if len(lastMessages) < 2 {
		t.Fatalf("provider received %d messages, want at least 2", len(lastMessages))
	}
	if got := lastMessages[len(lastMessages)-1]["role"]; got != "user" {
		t.Fatalf("expected new user message at end of payload, got %v", got)
	}
	if len(agent.Session.Messages) != 2 {
		t.Fatalf("session has %d messages, want 2 after sanitize + response", len(agent.Session.Messages))
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

	err := agent.SetModelLocal("test/other-model")
	if err != nil {
		t.Fatalf("SetModelLocal() error: %v", err)
	}
	if agent.ModelID != "test/other-model" {
		t.Errorf("ModelID = %q, want 'test/other-model'", agent.ModelID)
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
	if agent.Tools.Get("ask_a_friend") == nil {
		t.Error("ask_a_friend tool not registered")
	}
	if agent.Tools.Get("analyze_image") == nil {
		t.Error("analyze_image tool not registered")
	}
	if agent.Builder == nil {
		t.Error("Builder is nil")
	}
}

// TestNewAgentBadModel verifies error when model ID is invalid.
func TestNewAgentBadModel(t *testing.T) {
	home := isolatedHome(t)

	// Create an interactive definition with the same bad model as the role default.
	writeAgentFile(t, home, "default.md", "---\nname: default\ndescription: Test\ntype: interactive\nmodel: ghost/test-model\ntools:\n  - shell\n---\n")

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
	_, err := NewAgent(cfg, sess, platform.Linux, os.DirFS(dir), fstest.MapFS{}, dir, &mockHandler{}, "console")
	if err == nil {
		t.Fatal("NewAgent() expected error for missing provider, got nil")
	}
}

// isolatedHome sets HOME to a temp dir and returns it.
// Use the returned path for all agent/config file creation.
func isolatedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

// writeAgentFile writes an agent definition file under the given home dir.
func writeAgentFile(t *testing.T, home, filename, content string) {
	t.Helper()
	agentsDir := filepath.Join(home, "blazeai", "agents")
	os.MkdirAll(agentsDir, 0755)
	if err := os.WriteFile(filepath.Join(agentsDir, filename), []byte(content), 0600); err != nil {
		t.Fatalf("cannot write agent definition %s: %v", filename, err)
	}
}

// writeAgentsConfig writes an agents.json config file under the given home dir.
func writeAgentsConfig(t *testing.T, home, content string) {
	t.Helper()
	configDir := filepath.Join(home, "blazeai", "config")
	os.MkdirAll(configDir, 0755)
	if err := os.WriteFile(filepath.Join(configDir, "agents.json"), []byte(content), 0600); err != nil {
		t.Fatalf("cannot write agents.json: %v", err)
	}
}

// TestNewAgentLoadsPersistedInteractiveAgent verifies CurrentAgent initialization from LastAgent.
func TestNewAgentLoadsPersistedInteractiveAgent(t *testing.T) {
	home := isolatedHome(t)

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

	writeAgentFile(t, home, "planner.md", "---\nname: planner\ndescription: Plan tasks\ntype: interactive\nmodel: test/model-b\ntools:\n  - shell\n---\nPlan instructions.\n")
	writeAgentsConfig(t, home, `{"agents":[{"name":"planner","model":"test/model-b"}],"last_agent":"planner"}`)

	dir := t.TempDir()
	sess, _ := session.CreateInDir(dir)
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	writePromptFixtures(t, promptsDir)

	agent, err := NewAgent(cfg, sess, platform.Linux, os.DirFS(promptsDir), fstest.MapFS{}, dir, &mockHandler{}, "console")
	if err != nil {
		t.Fatalf("NewAgent() error: %v", err)
	}
	if agent.CurrentAgent == nil {
		t.Fatal("CurrentAgent is nil")
	}
	if agent.CurrentAgent.Name != "planner" {
		t.Errorf("CurrentAgent.Name = %q, want 'planner'", agent.CurrentAgent.Name)
	}
	if agent.ModelID != "test/model-b" {
		t.Errorf("ModelID = %q, want 'test/model-b'", agent.ModelID)
	}
}

// TestNewAgentInitializesAgentStateFromDefinitions verifies state initialization
// when no persisted state exists but interactive definitions are loaded.
func TestNewAgentInitializesAgentStateFromDefinitions(t *testing.T) {
	home := isolatedHome(t)

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

	// Create an interactive agent definition.
	agentsHome := filepath.Join(home, "blazeai")
	agentsDir := filepath.Join(agentsHome, "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "default.md"), []byte("---\nname: default\ndescription: Default agent\ntype: interactive\nmodel: test/model-a\ntools:\n  - shell\n---\nDefault instructions.\n"), 0600)

	dir := t.TempDir()
	sess, _ := session.CreateInDir(dir)
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	writePromptFixtures(t, promptsDir)

	agent, err := NewAgent(cfg, sess, platform.Linux, os.DirFS(promptsDir), fstest.MapFS{}, dir, &mockHandler{}, "console")
	if err != nil {
		t.Fatalf("NewAgent() error: %v", err)
	}
	if agent.CurrentAgent == nil {
		t.Fatal("CurrentAgent is nil")
	}
	if agent.CurrentAgent.Name != "default" {
		t.Errorf("CurrentAgent.Name = %q, want 'default'", agent.CurrentAgent.Name)
	}
}

// TestSetAgentPersistsLastAgentAndRefreshesCapabilities verifies agent switching
// updates CurrentAgent, persists LastAgent, and refreshes capabilities.
func TestSetAgentPersistsLastAgentAndRefreshesCapabilities(t *testing.T) {
	home := isolatedHome(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	cfg := &config.Config{
		Providers: []config.Provider{
			{Name: "test", Endpoint: server.URL, APIKey: "sk-test"},
		},
		Roles:          config.Roles{Default: "test/test-model"},
		FavoriteModels: []string{"test/test-model"},
		Compaction:     config.DefaultCompaction(),
		StripReasoning: config.DefaultStripReasoning(),
	}

	// Create interactive agent definitions.
	agentsHome := filepath.Join(home, "blazeai")
	agentsDir := filepath.Join(agentsHome, "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "default.md"), []byte("---\nname: default\ndescription: Default\ntype: interactive\nmodel: test/test-model\ntools:\n  - shell\n---\n"), 0600)
	os.WriteFile(filepath.Join(agentsDir, "planning.md"), []byte("---\nname: planning\ndescription: Planning\ntype: interactive\nmodel: test/test-model\ndirective: read-only\ntools:\n  - shell\n---\n"), 0600)

	dir := t.TempDir()
	sess, _ := session.CreateInDir(dir)
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	writePromptFixtures(t, promptsDir)

	agent, err := NewAgent(cfg, sess, platform.Linux, os.DirFS(promptsDir), fstest.MapFS{}, dir, &mockHandler{}, "console")
	if err != nil {
		t.Fatalf("NewAgent() error: %v", err)
	}
	if agent.CurrentAgent == nil || agent.CurrentAgent.Name != "default" {
		t.Fatalf("CurrentAgent = %v, want default", agent.CurrentAgent)
	}

	err = agent.SetAgent("planning")
	if err != nil {
		t.Fatalf("SetAgent() error: %v", err)
	}
	if agent.CurrentAgent.Name != "planning" {
		t.Errorf("CurrentAgent.Name = %q, want 'planning'", agent.CurrentAgent.Name)
	}
	if agent.AgentStates.LastAgent != "planning" {
		t.Errorf("LastAgent = %q, want 'planning'", agent.AgentStates.LastAgent)
	}
	if agent.Compactor == nil || agent.Compactor.Provider != agent.Provider {
		t.Fatal("compactor provider not synced after agent switch")
	}
}

// TestSetAgentRejectsExecutor verifies SetAgent rejects executor type names.
func TestSetAgentRejectsExecutor(t *testing.T) {
	home := isolatedHome(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	cfg := &config.Config{
		Providers: []config.Provider{
			{Name: "test", Endpoint: server.URL, APIKey: "sk-test"},
		},
		Roles:          config.Roles{Default: "test/test-model"},
		FavoriteModels: []string{"test/test-model"},
		Compaction:     config.DefaultCompaction(),
		StripReasoning: config.DefaultStripReasoning(),
	}

	// Create an interactive + executor definition.
	agentsHome := filepath.Join(home, "blazeai")
	agentsDir := filepath.Join(agentsHome, "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "default.md"), []byte("---\nname: default\ndescription: Default\ntype: interactive\nmodel: test/test-model\ntools:\n  - shell\n---\n"), 0600)
	os.WriteFile(filepath.Join(agentsDir, "coder.md"), []byte("---\nname: coder\ndescription: Code executor\ntype: executor\ntools:\n  - shell\n---\n"), 0600)

	dir := t.TempDir()
	sess, _ := session.CreateInDir(dir)
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	writePromptFixtures(t, promptsDir)

	agent, err := NewAgent(cfg, sess, platform.Linux, os.DirFS(promptsDir), fstest.MapFS{}, dir, &mockHandler{}, "console")
	if err != nil {
		t.Fatalf("NewAgent() error: %v", err)
	}

	err = agent.SetAgent("coder")
	if err == nil {
		t.Fatal("SetAgent() expected error for executor name, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("SetAgent() error = %v, want 'not found' for executor", err)
	}
}

// TestNextAgentCyclesInteractiveDefinitions verifies cyclic agent switching.
func TestNextAgentCyclesInteractiveDefinitions(t *testing.T) {
	home := isolatedHome(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	cfg := &config.Config{
		Providers: []config.Provider{
			{Name: "test", Endpoint: server.URL, APIKey: "sk-test"},
		},
		Roles:          config.Roles{Default: "test/test-model"},
		FavoriteModels: []string{"test/test-model"},
		Compaction:     config.DefaultCompaction(),
		StripReasoning: config.DefaultStripReasoning(),
	}

	// Create three interactive agent definitions.
	agentsHome := filepath.Join(home, "blazeai")
	agentsDir := filepath.Join(agentsHome, "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "alpha.md"), []byte("---\nname: alpha\ndescription: Alpha\ntype: interactive\nmodel: test/test-model\ntools:\n  - shell\n---\n"), 0600)
	os.WriteFile(filepath.Join(agentsDir, "beta.md"), []byte("---\nname: beta\ndescription: Beta\ntype: interactive\nmodel: test/test-model\ntools:\n  - shell\n---\n"), 0600)
	os.WriteFile(filepath.Join(agentsDir, "gamma.md"), []byte("---\nname: gamma\ndescription: Gamma\ntype: interactive\nmodel: test/test-model\ntools:\n  - shell\n---\n"), 0600)

	dir := t.TempDir()
	sess, _ := session.CreateInDir(dir)
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	writePromptFixtures(t, promptsDir)

	agent, err := NewAgent(cfg, sess, platform.Linux, os.DirFS(promptsDir), fstest.MapFS{}, dir, &mockHandler{}, "console")
	if err != nil {
		t.Fatalf("NewAgent() error: %v", err)
	}

	// Cycle: alpha -> beta
	def, err := agent.NextAgent()
	if err != nil {
		t.Fatalf("NextAgent() error: %v", err)
	}
	if def.Name != "beta" {
		t.Errorf("NextAgent() = %q, want 'beta'", def.Name)
	}

	// Cycle: beta -> gamma
	def, err = agent.NextAgent()
	if err != nil {
		t.Fatalf("NextAgent() error: %v", err)
	}
	if def.Name != "gamma" {
		t.Errorf("NextAgent() = %q, want 'gamma'", def.Name)
	}

	// Cycle: gamma -> alpha (wrap around)
	def, err = agent.NextAgent()
	if err != nil {
		t.Fatalf("NextAgent() error: %v", err)
	}
	if def.Name != "alpha" {
		t.Errorf("NextAgent() = %q, want 'alpha'", def.Name)
	}
}

// TestSetModelPersistsPerInteractiveAgent verifies that SetModel updates the
// persisted model for the current interactive agent in agents.json.
func TestSetModelPersistsPerInteractiveAgent(t *testing.T) {
	home := isolatedHome(t)

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

	agentsHome := filepath.Join(home, "blazeai")
	agentsDir := filepath.Join(agentsHome, "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "default.md"), []byte("---\nname: default\ndescription: Default\ntype: interactive\nmodel: test/model-a\ntools:\n  - shell\n---\n"), 0600)

	dir := t.TempDir()
	sess, _ := session.CreateInDir(dir)
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	writePromptFixtures(t, promptsDir)

	agent, err := NewAgent(cfg, sess, platform.Linux, os.DirFS(promptsDir), fstest.MapFS{}, dir, &mockHandler{}, "console")
	if err != nil {
		t.Fatalf("NewAgent() error: %v", err)
	}

	err = agent.SetModel("test/other-model")
	if err != nil {
		t.Fatalf("SetModel() error: %v", err)
	}
	if agent.ModelID != "test/other-model" {
		t.Errorf("ModelID = %q, want 'test/other-model'", agent.ModelID)
	}
}

// TestNextFavoriteModel verifies cycling through favorite models.
func TestNextFavoriteModel(t *testing.T) {
	agent, _, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	agent.Config.FavoriteModels = []string{"test/model-a", "test/model-b", "test/model-c"}
	agent.ModelID = "test/model-a"

	err := agent.NextFavoriteModel()
	if err != nil {
		t.Fatalf("NextFavoriteModel() error: %v", err)
	}
	if agent.ModelID != "test/model-b" {
		t.Errorf("ModelID = %q, want 'test/model-b'", agent.ModelID)
	}

	// Second cycle: b -> c
	err = agent.NextFavoriteModel()
	if err != nil {
		t.Fatalf("NextFavoriteModel() error: %v", err)
	}
	if agent.ModelID != "test/model-c" {
		t.Errorf("ModelID = %q, want 'test/model-c'", agent.ModelID)
	}

	// Third cycle: c -> a (wrap around)
	err = agent.NextFavoriteModel()
	if err != nil {
		t.Fatalf("NextFavoriteModel() error: %v", err)
	}
	if agent.ModelID != "test/model-a" {
		t.Errorf("ModelID = %q, want 'test/model-a' (wrap)", agent.ModelID)
	}
}

// TestNextFavoriteModelEmpty verifies no-op when favorites list is empty.
func TestNextFavoriteModelEmpty(t *testing.T) {
	agent, _, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	agent.Config.FavoriteModels = nil
	agent.ModelID = "test/test-model"

	err := agent.NextFavoriteModel()
	if err != nil {
		t.Fatalf("NextFavoriteModel() error: %v", err)
	}
	if agent.ModelID != "test/test-model" {
		t.Errorf("ModelID changed to %q, want unchanged 'test/test-model'", agent.ModelID)
	}
}

// TestNextFavoriteModelSingle verifies no-op when only one favorite exists.
func TestNextFavoriteModelSingle(t *testing.T) {
	agent, _, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	agent.Config.FavoriteModels = []string{"test/solo"}
	agent.ModelID = "test/solo"

	err := agent.NextFavoriteModel()
	if err != nil {
		t.Fatalf("NextFavoriteModel() error: %v", err)
	}
	if agent.ModelID != "test/solo" {
		t.Errorf("ModelID changed to %q, want unchanged 'test/solo'", agent.ModelID)
	}
}

// TestNextFavoriteModelNotInList verifies cycling picks first when current model is not in favorites.
func TestNextFavoriteModelNotInList(t *testing.T) {
	agent, _, server := setupAgent(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	agent.Config.FavoriteModels = []string{"test/a", "test/b"}
	agent.ModelID = "test/alien" // not in favorites

	err := agent.NextFavoriteModel()
	if err != nil {
		t.Fatalf("NextFavoriteModel() error: %v", err)
	}
	if agent.ModelID != "test/a" {
		t.Errorf("ModelID = %q, want 'test/a' (first in list)", agent.ModelID)
	}
}

// TestNewAgentIgnoresLastModelWhenLastAgentExists verifies that the active agent wins over legacy last_model.
func TestNewAgentIgnoresLastModelWhenLastAgentExists(t *testing.T) {
	home := isolatedHome(t)

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

	// Create interactive definitions.
	agentsHome := filepath.Join(home, "blazeai")
	agentsDir := filepath.Join(agentsHome, "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "default.md"), []byte("---\nname: default\ndescription: Default\ntype: interactive\nmodel: test/model-a\ntools:\n  - shell\n---\n"), 0600)
	os.WriteFile(filepath.Join(agentsDir, "planning.md"), []byte("---\nname: planning\ndescription: Planning\ntype: interactive\nmodel: test/model-b\ntools:\n  - shell\n---\n"), 0600)

	// Persist agent state selecting "planning".
	configDir := filepath.Join(agentsHome, "config")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "agents.json"), []byte(`{"agents":[{"name":"default","model":"test/model-a"},{"name":"planning","model":"test/model-b"}],"last_agent":"planning"}`), 0600)

	dir := t.TempDir()
	sess, _ := session.CreateInDir(dir)
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	writePromptFixtures(t, promptsDir)

	agent, err := NewAgent(cfg, sess, platform.Linux, os.DirFS(promptsDir), fstest.MapFS{}, dir, &mockHandler{}, "console")
	if err != nil {
		t.Fatalf("NewAgent() error: %v", err)
	}
	if agent.CurrentAgent == nil || agent.CurrentAgent.Name != "planning" {
		t.Fatalf("CurrentAgent = %v, want planning", agent.CurrentAgent)
	}
	if agent.ModelID != "test/model-b" {
		t.Errorf("ModelID = %q, want 'test/model-b'", agent.ModelID)
	}
}

// TestInjectDirective verifies directive injection appends to the latest user message in copy.
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
	if !strings.Contains(last, "[AGENT DIRECTIVE]") {
		t.Error("result[1].Content missing [AGENT DIRECTIVE]")
	}
	if !strings.Contains(last, "be quick") {
		t.Error("result[1].Content missing directive text")
	}
	if !strings.Contains(last, "hello") {
		t.Error("result[1].Content missing original content")
	}
}

// TestInjectDirectiveSkipsToolTail verifies tool results stay untouched and the directive is
// applied to the latest user message instead.
func TestInjectDirectiveSkipsToolTail(t *testing.T) {
	original := []session.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hello"},
		{Role: "tool", Content: "tool output"},
	}
	result := injectDirective(original, "be quick")

	toolContent, ok := result[2].Content.(string)
	if !ok {
		t.Fatal("result[2].Content is not string")
	}
	if toolContent != "tool output" {
		t.Fatalf("tool content mutated: %q", toolContent)
	}
	userContent, ok := result[1].Content.(string)
	if !ok {
		t.Fatal("result[1].Content is not string")
	}
	if !strings.Contains(userContent, "[AGENT DIRECTIVE]") || !strings.Contains(userContent, "be quick") {
		t.Fatalf("user content missing directive: %q", userContent)
	}
}

// TestInjectDirectiveEmpty verifies empty messages returns empty.
func TestInjectDirectiveEmpty(t *testing.T) {
	result := injectDirective([]session.Message{}, "directive")
	if len(result) != 0 {
		t.Errorf("injectDirective on empty returned %d messages", len(result))
	}
}

// TestRunTurnInjectsInteractiveDirectiveEphemerally verifies that the agent directive
// is injected into the provider-bound user message but never persisted to the session.
func TestRunTurnInjectsInteractiveDirectiveEphemerally(t *testing.T) {
	var providerMessages []map[string]interface{}
	response := func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []map[string]interface{} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode provider request: %v", err)
		}
		providerMessages = payload.Messages
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"ok"}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "data: [DONE]")
		fmt.Fprintln(w)
	}

	home := isolatedHome(t)

	server := httptest.NewServer(http.HandlerFunc(response))
	defer server.Close()

	cfg := &config.Config{
		Providers: []config.Provider{
			{Name: "test", Endpoint: server.URL, APIKey: "sk-test"},
		},
		Roles:          config.Roles{Default: "test/test-model"},
		FavoriteModels: []string{"test/test-model"},
		Compaction:     config.DefaultCompaction(),
		StripReasoning: config.DefaultStripReasoning(),
	}

	// Create interactive definition with a directive.
	agentsHome := filepath.Join(home, "blazeai")
	agentsDir := filepath.Join(agentsHome, "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "default.md"), []byte("---\nname: default\ndescription: Default\ntype: interactive\nmodel: test/test-model\ndirective: Be concise always.\ntools:\n  - shell\n---\n"), 0600)

	dir := t.TempDir()
	sess, _ := session.CreateInDir(dir)
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	writePromptFixtures(t, promptsDir)

	agent, err := NewAgent(cfg, sess, platform.Linux, os.DirFS(promptsDir), fstest.MapFS{}, dir, &mockHandler{}, "console")
	if err != nil {
		t.Fatalf("NewAgent() error: %v", err)
	}

	err = agent.RunTurn(context.Background(), "hello")
	if err != nil {
		t.Fatalf("RunTurn() error: %v", err)
	}

	// Session must NOT contain the directive — it is ephemeral.
	for _, msg := range agent.Session.Messages {
		content, _ := msg.Content.(string)
		if strings.Contains(content, "[AGENT DIRECTIVE]") {
			t.Fatalf("session message contains ephemeral directive: role=%q content=%q", msg.Role, content)
		}
	}

	// Provider-bound messages must contain the directive.
	providerData, err := json.Marshal(providerMessages)
	if err != nil {
		t.Fatalf("marshal provider messages: %v", err)
	}
	if !strings.Contains(string(providerData), "[AGENT DIRECTIVE]") {
		t.Fatalf("provider-bound messages missing [AGENT DIRECTIVE]: %s", providerData)
	}
	if !strings.Contains(string(providerData), "Be concise always.") {
		t.Fatalf("provider-bound messages missing directive text: %s", providerData)
	}
}

// TestNewAgentBootstrapsDefaultInteractiveAgent verifies that NewAgent automatically
// creates a default.md interactive definition on a fresh app home with no definitions
// and no agents.json state.
func TestNewAgentBootstrapsDefaultInteractiveAgent(t *testing.T) {
	home := isolatedHome(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	cfg := &config.Config{
		Providers: []config.Provider{
			{Name: "test", Endpoint: server.URL, APIKey: "sk-test"},
		},
		Roles:          config.Roles{Default: "test/test-model"},
		FavoriteModels: []string{"test/test-model"},
		Compaction:     config.DefaultCompaction(),
		StripReasoning: config.DefaultStripReasoning(),
	}

	dir := t.TempDir()
	sess, _ := session.CreateInDir(dir)
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	writePromptFixtures(t, promptsDir)

	agent, err := NewAgent(cfg, sess, platform.Linux, os.DirFS(promptsDir), fstest.MapFS{}, dir, &mockHandler{}, "console")
	if err != nil {
		t.Fatalf("NewAgent() error: %v", err)
	}

	// Verify default.md was created.
	agentsDir := filepath.Join(home, "blazeai", "agents")
	target := filepath.Join(agentsDir, "default.md")
	if _, statErr := os.Stat(target); statErr != nil {
		t.Fatalf("default.md not created: %v", statErr)
	}

	// Parse the generated definition through the standard parser.
	def, err := agents.ParseFile(target)
	if err != nil {
		t.Fatalf("agents.ParseFile() error: %v", err)
	}
	if def.Name != "default" {
		t.Errorf("definition Name = %q, want 'default'", def.Name)
	}
	if def.Type != agents.TypeInteractive {
		t.Errorf("definition Type = %q, want 'interactive'", def.Type)
	}
	if def.Model != "test/test-model" {
		t.Errorf("definition Model = %q, want 'test/test-model'", def.Model)
	}
	if def.Instructions == "" {
		t.Error("definition body is empty")
	}

	// Verify agent is wired correctly.
	if agent.CurrentAgent == nil {
		t.Fatal("CurrentAgent is nil")
	}
	if agent.CurrentAgent.Name != "default" {
		t.Errorf("CurrentAgent.Name = %q, want 'default'", agent.CurrentAgent.Name)
	}
	if agent.ModelID != "test/test-model" {
		t.Errorf("ModelID = %q, want 'test/test-model'", agent.ModelID)
	}

	// Verify agents.json has LastAgent: default.
	agentsConfigPath := filepath.Join(home, "blazeai", "config", "agents.json")
	if _, statErr := os.Stat(agentsConfigPath); statErr != nil {
		t.Fatalf("agents.json not created: %v", statErr)
	}
	agentsCfg, err := config.LoadAgentsFrom(agentsConfigPath)
	if err != nil {
		t.Fatalf("LoadAgentsFrom() error: %v", err)
	}
	if agentsCfg.LastAgent != "default" {
		t.Errorf("LastAgent = %q, want 'default'", agentsCfg.LastAgent)
	}
}

// TestNewAgentDoesNotBootstrapWhenAgentStateExists verifies that NewAgent does not
// create a default.md when agents.json already exists, even with no interactive definitions.
func TestNewAgentDoesNotBootstrapWhenAgentStateExists(t *testing.T) {
	home := isolatedHome(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	cfg := &config.Config{
		Providers: []config.Provider{
			{Name: "test", Endpoint: server.URL, APIKey: "sk-test"},
		},
		Roles:          config.Roles{Default: "test/test-model"},
		FavoriteModels: []string{"test/test-model"},
		Compaction:     config.DefaultCompaction(),
		StripReasoning: config.DefaultStripReasoning(),
	}

	// Create agents.json but no definition files.
	writeAgentsConfig(t, home, `{"agents":[],"last_agent":""}`)

	dir := t.TempDir()
	sess, _ := session.CreateInDir(dir)
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	writePromptFixtures(t, promptsDir)

	_, err := NewAgent(cfg, sess, platform.Linux, os.DirFS(promptsDir), fstest.MapFS{}, dir, &mockHandler{}, "console")
	if err == nil {
		t.Fatal("NewAgent() expected error for no interactive definitions with existing state, got nil")
	}
	if !strings.Contains(err.Error(), "no interactive agent") {
		t.Fatalf("NewAgent() error = %v, want 'no interactive agent'", err)
	}

	// Verify default.md was NOT created.
	agentsDir := filepath.Join(home, "blazeai", "agents")
	target := filepath.Join(agentsDir, "default.md")
	if _, statErr := os.Stat(target); statErr == nil {
		t.Fatal("default.md should not exist when agents.json state exists")
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
