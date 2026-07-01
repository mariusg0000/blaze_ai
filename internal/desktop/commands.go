// commands.go — desktop-local command handling.
// Handles a small transport-local command set directly in the desktop UI so
// fixed-session and local model behavior do not depend on the LLM.
// Layer: transport commands. Dependencies: internal/config, internal/runtime.
package desktop

import (
	"fmt"
	"strings"

	"blazeai/internal/config"
	"blazeai/internal/runtime"
)

// HandleCommand processes a desktop slash command.
//
// WHAT:  Executes transport-local commands for the desktop companion.
// WHY:   Desktop model/session actions should be immediate and deterministic.
// PARAMS: input — raw user input; agent — bound runtime agent; cfg — global config;
// state — mutable desktop state; statePath — state file path.
// RETURNS: handled — true if recognized; response — UI text to show; error on failure.
func HandleCommand(input string, agent *runtime.Agent, cfg *config.Config, state *State, statePath string) (bool, string, error) {
	parts := strings.SplitN(strings.TrimSpace(input), " ", 2)
	cmd := strings.TrimSpace(parts[0])
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}

	switch cmd {
	case "/help", "/start":
		return true, helpText(), nil
	case "/model":
		if arg == "" {
			return true, formatModelInfo(agent, cfg), nil
		}
		if err := agent.SetModelLocal(arg); err != nil {
			return true, "", fmt.Errorf("cannot set model: %w", err)
		}
		state.SetSelectedModel(arg)
		if err := state.SaveTo(statePath, cfg); err != nil {
			return true, "", fmt.Errorf("cannot persist desktop state: %w", err)
		}
		return true, fmt.Sprintf("Model set to: %s", arg), nil
	case "/clear", "/new":
		if err := agent.ResetConversation(); err != nil {
			return true, "", fmt.Errorf("cannot reset session: %w", err)
		}
		return true, "Session cleared.", nil
	case "/exit":
		if err := agent.CloseSession(); err != nil {
			return true, "", fmt.Errorf("cannot close session: %w", err)
		}
		return true, "Session closed cleanly. Desktop app stays online.", nil
	default:
		return false, "", nil
	}
}

func helpText() string {
	return strings.Join([]string{
		"Supported commands:",
		"/help - show this help",
		"/model [provider/model_name] - show or change the desktop model",
		"/clear - clear the current conversation",
		"/new - same as /clear for the fixed session",
		"/exit - close the current session cleanly without quitting the app",
	}, "\n")
}

func formatModelInfo(agent *runtime.Agent, cfg *config.Config) string {
	lines := []string{fmt.Sprintf("Current model: %s", agent.ModelID)}
	if len(cfg.FavoriteModels) == 0 {
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "Favorite models:")
	for _, modelID := range cfg.FavoriteModels {
		marker := "  "
		if modelID == agent.ModelID {
			marker = "> "
		}
		lines = append(lines, marker+modelID)
	}
	return strings.Join(lines, "\n")
}
