// commands_test.go — tests for Telegram bridge command handling.
// Verifies local model persistence and session reset behavior without Telegram network calls.
package telegram

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blazeai/internal/config"
	"blazeai/internal/platform"
	"blazeai/internal/runtime"
	"blazeai/internal/session"
)

func newTelegramAgent(t *testing.T) (*runtime.Agent, *config.Config, *State, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	cfg := &config.Config{
		Providers:      []config.Provider{{Name: "test", Endpoint: "https://example.com", APIKey: "sk-test"}},
		FavoriteModels: []string{"test/main", "test/other"},
		Roles:          config.Roles{Default: "test/main"},
		Compaction:     config.DefaultCompaction(),
		StripReasoning: config.DefaultStripReasoning(),
	}
	workDir := t.TempDir()
	sess, err := session.Create(workDir)
	if err != nil {
		t.Fatalf("session.Create() error: %v", err)
	}
	promptsDir := filepath.Join(t.TempDir(), "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		t.Fatalf("mkdir prompts: %v", err)
	}
	writePromptFixtures(t, promptsDir)
	agent, err := runtime.NewAgent(cfg, sess, platform.Linux, os.DirFS(promptsDir), workDir, nil)
	if err != nil {
		t.Fatalf("runtime.NewAgent() error: %v", err)
	}
	state := &State{SelectedModel: "test/main"}
	statePath := filepath.Join(t.TempDir(), stateFileName)
	if err := state.SaveTo(statePath, cfg); err != nil {
		t.Fatalf("state.SaveTo() error: %v", err)
	}
	return agent, cfg, state, statePath
}

func writePromptFixtures(t *testing.T, promptsDir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(promptsDir, "sysprompt.md"), []byte("base\n{OS_PROMPT}\n{HOST_HELPERS_ADVISORY}\n{HOST_HELPERS_AVAILABLE}\n{HOST_HELPERS_OPTIONAL}\n{SKILLS_AVAILABLE}\n{SKILLS_ACTIVE}\n{MEMORIES_AVAILABLE}\n{MEMORIES_ACTIVE}\n{AGENTS_CONTENT}\n"), 0644); err != nil {
		t.Fatalf("write sysprompt.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(promptsDir, "sysprompt.linux.md"), []byte("linux"), 0644); err != nil {
		t.Fatalf("write sysprompt.linux.md: %v", err)
	}
}

func TestHandleCommandModelPersistsStateOnly(t *testing.T) {
	agent, cfg, state, statePath := newTelegramAgent(t)
	modesBefore, err := config.LoadModes(cfg.Roles.Default)
	if err != nil {
		t.Fatalf("LoadModes() before error: %v", err)
	}
	handled, response, err := HandleCommand(context.Background(), "/model test/other", agent, cfg, state, statePath)
	if err != nil {
		t.Fatalf("HandleCommand() error: %v", err)
	}
	if !handled {
		t.Fatal("HandleCommand() should handle /model")
	}
	if response != "Model set to: test/other" {
		t.Fatalf("response = %q", response)
	}
	loadedState, err := LoadStateFrom(statePath, cfg)
	if err != nil {
		t.Fatalf("LoadStateFrom() after /model error: %v", err)
	}
	if loadedState.SelectedModel != "test/other" {
		t.Fatalf("SelectedModel = %q, want test/other", loadedState.SelectedModel)
	}
	modesAfter, err := config.LoadModes(cfg.Roles.Default)
	if err != nil {
		t.Fatalf("LoadModes() after error: %v", err)
	}
	if modesBefore.Modes[0].Model != modesAfter.Modes[0].Model {
		t.Fatalf("modes model changed from %q to %q", modesBefore.Modes[0].Model, modesAfter.Modes[0].Model)
	}
}

func TestHandleCommandClearResetsConversation(t *testing.T) {
	agent, cfg, state, statePath := newTelegramAgent(t)
	if err := agent.Session.Append(session.Message{Role: "user", Content: "hello"}); err != nil {
		t.Fatalf("Append() error: %v", err)
	}
	handled, response, err := HandleCommand(context.Background(), "/clear", agent, cfg, state, statePath)
	if err != nil {
		t.Fatalf("HandleCommand() error: %v", err)
	}
	if !handled || response != "Session cleared." {
		t.Fatalf("unexpected clear response: handled=%v response=%q", handled, response)
	}
	if len(agent.Session.Messages) != 0 {
		t.Fatalf("session messages = %d, want 0", len(agent.Session.Messages))
	}
}

func TestHandleCommandExitClosesSession(t *testing.T) {
	agent, cfg, state, statePath := newTelegramAgent(t)
	handled, _, err := HandleCommand(context.Background(), "/exit", agent, cfg, state, statePath)
	if err != nil {
		t.Fatalf("HandleCommand() error: %v", err)
	}
	if !handled {
		t.Fatal("HandleCommand() should handle /exit")
	}
	loaded, err := session.Load(agent.Session.Folder)
	if err != nil {
		t.Fatalf("session.Load() error: %v", err)
	}
	if !loaded.ClosedCleanly {
		t.Fatal("ClosedCleanly = false, want true")
	}
}

func TestHandleCommandStartReturnsHelp(t *testing.T) {
	agent, cfg, state, statePath := newTelegramAgent(t)
	handled, response, err := HandleCommand(context.Background(), "/start", agent, cfg, state, statePath)
	if err != nil {
		t.Fatalf("HandleCommand() error: %v", err)
	}
	if !handled {
		t.Fatal("HandleCommand() should handle /start")
	}
	if !strings.Contains(response, "Supported commands:") {
		t.Fatalf("response = %q, want help text", response)
	}
}

func TestTelegramBotCommandsIncludeSupportedMenuEntries(t *testing.T) {
	commands := telegramBotCommands()
	if len(commands) != 5 {
		t.Fatalf("commands = %d, want 5", len(commands))
	}
	if commands[0].Command != "help" || commands[1].Command != "model" {
		t.Fatalf("unexpected leading commands: %#v", commands[:2])
	}
	if commands[len(commands)-1].Command != "exit" {
		t.Fatalf("last command = %q, want exit", commands[len(commands)-1].Command)
	}
}

var _ = http.MethodGet
