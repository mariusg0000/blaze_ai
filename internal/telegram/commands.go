// commands.go — Telegram-local slash command handling.
// Handles the approved Telegram command set directly in the bridge so unsupported
// transport-specific behavior does not reach the LLM.
// Layer: transport commands. Dependencies: internal/config, internal/runtime.
package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"blazeai/internal/config"
	"blazeai/internal/runtime"
)

type botCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

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
	case "/help", "/start":
		return true, telegramHelpText(), nil
	case "/model":
		if arg == "" {
			response, err := beginModelSelection(agent, cfg, state, statePath)
			return true, response, err
		}
		if err := agent.SetModelLocal(arg); err != nil {
			return true, "", fmt.Errorf("cannot set model: %w", err)
		}
		clearModelSelection(state)
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

const (
	modelStageProvider = "provider"
	modelStageModel    = "model"
)

// beginModelSelection starts the same provider-then-model flow used by the console.
//
// WHAT: Lists providers or immediately lists models when only one provider exists.
// HOW: Stores the next selection stage in Telegram state because replies arrive as separate updates.
func beginModelSelection(agent *runtime.Agent, cfg *config.Config, state *State, statePath string) (string, error) {
	if len(cfg.Providers) == 0 {
		return "", fmt.Errorf("no providers configured")
	}
	clearModelSelection(state)
	if len(cfg.Providers) > 1 {
		state.PendingStage = modelStageProvider
		if err := state.SaveTo(statePath, cfg); err != nil {
			return "", fmt.Errorf("cannot persist telegram model selection: %w", err)
		}
		lines := []string{"Select provider:"}
		for i, provider := range cfg.Providers {
			lines = append(lines, fmt.Sprintf("%d. %s (%s)", i+1, provider.Name, provider.Endpoint))
		}
		return strings.Join(lines, "\n"), nil
	}
	return selectProviderForModels(agent, cfg, state, statePath, cfg.Providers[0].Name)
}

// HandleModelSelection consumes a numeric Telegram reply when /model is awaiting a choice.
//
// WHAT: Advances provider selection or applies the selected model.
// RETURNS: handled is false when no model selection is pending.
func HandleModelSelection(input string, agent *runtime.Agent, cfg *config.Config, state *State, statePath string) (handled bool, response string, err error) {
	if state.PendingStage == "" {
		return false, "", nil
	}
	num, parseErr := strconv.Atoi(strings.TrimSpace(input))
	if parseErr != nil {
		return true, "", fmt.Errorf("invalid selection: enter a number")
	}
	if state.PendingStage == modelStageProvider {
		if num < 1 || num > len(cfg.Providers) {
			return true, fmt.Sprintf("invalid provider selection: enter 1-%d", len(cfg.Providers)), nil
		}
		response, err := selectProviderForModels(agent, cfg, state, statePath, cfg.Providers[num-1].Name)
		return true, response, err
	}
	if state.PendingStage == modelStageModel {
		if num < 1 || num > len(state.PendingModels) {
			return true, fmt.Sprintf("invalid model selection: enter 1-%d", len(state.PendingModels)), nil
		}
		modelID := state.PendingProvider + "/" + state.PendingModels[num-1]
		if err := agent.SetModelLocal(modelID); err != nil {
			return true, "", fmt.Errorf("cannot set model: %w", err)
		}
		state.SelectedModel = modelID
		clearModelSelection(state)
		if err := state.SaveTo(statePath, cfg); err != nil {
			return true, "", fmt.Errorf("cannot persist telegram state: %w", err)
		}
		return true, fmt.Sprintf("Model set to: %s", modelID), nil
	}
	clearModelSelection(state)
	if err := state.SaveTo(statePath, cfg); err != nil {
		return true, "", fmt.Errorf("cannot clear invalid telegram model selection: %w", err)
	}
	return true, "model selection was reset; send /model to start again", nil
}

// selectProviderForModels fetches and displays the live model list for one provider.
func selectProviderForModels(agent *runtime.Agent, cfg *config.Config, state *State, statePath, providerName string) (string, error) {
	models, err := agent.ListProviderModels(providerName)
	if err != nil {
		return "", fmt.Errorf("cannot list models: %w", err)
	}
	if len(models) == 0 {
		return "", fmt.Errorf("provider %s returned no models", providerName)
	}
	state.PendingStage = modelStageModel
	state.PendingProvider = providerName
	state.PendingModels = models
	if err := state.SaveTo(statePath, cfg); err != nil {
		return "", fmt.Errorf("cannot persist telegram model selection: %w", err)
	}
	lines := []string{fmt.Sprintf("Select model from %s:", providerName)}
	for i, model := range models {
		marker := ""
		if providerName+"/"+model == state.SelectedModel {
			marker = " (current)"
		}
		lines = append(lines, fmt.Sprintf("%d. %s%s", i+1, model, marker))
	}
	return strings.Join(lines, "\n"), nil
}

// clearModelSelection removes an incomplete interactive selection.
func clearModelSelection(state *State) {
	state.PendingStage = ""
	state.PendingProvider = ""
	state.PendingModels = nil
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
		"/model [provider/model_name] - select or change the instance model",
		"/clear - clear the current conversation",
		"/new - same as /clear in v1",
		"/exit - close the current session cleanly without stopping the bot",
		"This bot accepts messages only from its configured Telegram chat.",
	}, "\n")
}

func telegramBotCommands() []botCommand {
	return []botCommand{
		{Command: "help", Description: "show supported commands"},
		{Command: "model", Description: "show or change the instance model"},
		{Command: "clear", Description: "clear the current conversation"},
		{Command: "new", Description: "clear the current conversation"},
		{Command: "exit", Description: "close the current session cleanly"},
	}
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
