// commands.go — Telegram-local slash command handling.
// Handles the approved Telegram command set directly in the bridge so unsupported
// transport-specific behavior does not reach the LLM.
// Layer: transport commands. Dependencies: internal/config, internal/runtime.
package telegram

import (
	"context"
	"fmt"
	"strings"

	"blazeai/internal/config"
	"blazeai/internal/runtime"
)

// HandleCommand processes a Telegram slash command.
//
// WHAT:  Executes bridge-local command behavior for Telegram.
// WHY:   Telegram exposes only a constrained command set in v1.
// PARAMS: input — raw Telegram message text; agent — bound runtime agent;
// state — mutable Telegram instance state; statePath — file path for state persistence.
// RETURNS: handled — true if recognized; response — chat text to send; error on command failure.
func HandleCommand(_ context.Context, input string, agent *runtime.Agent, cfg *config.Config, state *State, statePath string) (bool, string, error) {
	parts := strings.SplitN(strings.TrimSpace(input), " ", 2)
	cmd := normalizeCommand(parts[0])
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}

	switch cmd {
	case "/help":
		return true, telegramHelpText(), nil
	case "/model":
		if arg == "" {
			return true, formatModelInfo(agent, cfg), nil
		}
		if err := agent.SetModelLocal(arg); err != nil {
			return true, "", fmt.Errorf("cannot set model: %w", err)
		}
		state.SelectedModel = arg
		if err := state.SaveTo(statePath, cfg); err != nil {
			return true, "", fmt.Errorf("cannot persist telegram state: %w", err)
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
		return true, "Session closed cleanly. Bridge stays online.", nil
	default:
		return false, "", nil
	}
}

func normalizeCommand(cmd string) string {
	base := strings.TrimSpace(cmd)
	if idx := strings.Index(base, "@"); idx > 0 {
		return base[:idx]
	}
	return base
}

func telegramHelpText() string {
	return strings.Join([]string{
		"Supported commands:",
		"/help - show this help",
		"/model [provider/model_name] - show or change the instance model",
		"/clear - clear the current conversation",
		"/new - same as /clear in v1",
		"/exit - close the current session cleanly without stopping the bot",
		"This bot accepts messages only from its configured Telegram chat.",
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
