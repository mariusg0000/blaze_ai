// runtime.go — agent core orchestration loop and handler contract.
// Defines the Handler interface (the only boundary between agent core and transports),
// the Agent struct that ties all packages together, and the RunTurn loop that drives
// prompt building, LLM streaming, tool execution, and message persistence.
// Layer: agent core. Dependencies: internal/config, internal/provider, internal/prompt,
// internal/session, internal/tools, internal/skills.
package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"blazeai/internal/compaction"
	"blazeai/internal/config"
	"blazeai/internal/llmcall"
	"blazeai/internal/platform"
	"blazeai/internal/prompt"
	"blazeai/internal/provider"
	"blazeai/internal/session"
	"blazeai/internal/skills"
	"blazeai/internal/tools"
)

// ErrTurnAborted reports that the active turn was canceled by the user.
var ErrTurnAborted = errors.New("turn aborted")

const userAbortMessage = "User requested an urgent abort. The previous assistant turn was interrupted before completion. Tool execution may have produced partial side effects before cancellation. Do not continue the interrupted response. Wait for the user's next instruction."

// Handler is the contract between the agent core and transports.
//
// WHAT:  The only boundary between agent core and user-facing transport.
// WHY:   Console and web both implement this interface over the same core.
type Handler interface {
	// OnContent is called for each streaming text chunk from the LLM.
	OnContent(delta string)
	// OnToolCall is called before a tool is executed with a formatted display string.
	OnToolCall(name string, args string)
	// OnToolResult is called after a tool has finished.
	OnToolResult(name string, result string)
	// OnUsage is called after each provider response with prompt token count.
	OnUsage(promptTokens int)

	// RequestSudoApproval is called before executing a shell command that requires sudo.
	// The handler prompts the user for confirmation, then reads a hidden password if approved.
	// approved: false means the user declined — the tool call is skipped.
	// password: empty on decline; the sudo password on approval. Never stored in session JSON.
	RequestSudoApproval(command string) (approved bool, password string)
}

// Agent is the core runtime that ties all packages together and drives the conversation loop.
//
// WHAT:  Holds all runtime state and orchestrates the LLM call cycle.
// WHY:   One struct ties config, session, skills, prompt, tools, and provider together.
// PARAMS: Config — loaded configuration; Session — current conversation session;
//
//	Active — in-memory active skills list; Builder — prompt assembler;
//	Tools — tool registry; Provider — LLM client; Handler — transport callbacks;
//	ModelID — current provider/model_name; WorkDir — current work folder; OS — detected platform.
type Agent struct {
	Config    *config.Config
	Modes     *config.ModesConfig
	Session   *session.Session
	Active    *skills.ActiveList
	Builder   *prompt.Builder
	Tools     *tools.Registry
	Provider  *provider.Client
	Handler   Handler
	Compactor *compaction.Manager

	ModelID     string
	CurrentMode *config.Mode
	WorkDir     string
	OS          platform.OS
}

// NewAgent creates an Agent from a loaded config, session, and detected OS.
//
// WHAT:  Constructs the runtime agent with all dependencies wired.
// WHY:   The main entrypoint calls this to assemble the agent before starting the REPL.
// PARAMS: cfg — loaded config; sess — session (new or resumed); os — detected platform;
//
//	promptsFS — filesystem with sysprompt.md and sysprompt.<os>.md;
//	workDir — initial work folder; handler — transport implementation.
//
// RETURNS: *Agent — ready to run; error if provider client cannot be created.
func NewAgent(cfg *config.Config, sess *session.Session, os platform.OS, promptsFS fs.FS, workDir string, handler Handler) (*Agent, error) {
	modelID := cfg.Roles.Default

	// Try migration first: extract modes from config.json if they exist there.
	_ = config.MigrateFromConfig()

	// Load modes from modes.json with fallback to the configured default role model.
	modes, err := config.LoadModes(modelID)
	if err != nil {
		return nil, fmt.Errorf("cannot load modes: %w", err)
	}

	// Auto-create default mode if no modes exist after load/migration.
	if len(modes.Modes) == 0 {
		modes = config.DefaultMode(modelID)
		_ = modes.Save()
	}

	// Resolve active mode: mode model takes priority over LastModel.
	var currentMode *config.Mode
	if modes.LastMode != "" {
		for i := range modes.Modes {
			if modes.Modes[i].Name == modes.LastMode {
				currentMode = &modes.Modes[i]
				break
			}
		}
	}
	if currentMode != nil {
		modelID = currentMode.Model
	} else if len(modes.Modes) > 0 {
		currentMode = &modes.Modes[0]
		modelID = currentMode.Model
	}

	client, err := provider.NewClient(cfg, modelID)
	if err != nil {
		return nil, fmt.Errorf("cannot create provider client: %w", err)
	}

	// Create a dedicated summarization client if the summarization role is configured with a different model.
	summarizationClient := client
	if summarizationModel, err := cfg.ModelForRole("summarization"); err == nil && summarizationModel != modelID {
		summarizationClient, err = provider.NewClient(cfg, summarizationModel)
		if err != nil {
			return nil, fmt.Errorf("cannot create summarization client: %w", err)
		}
	}

	active := skills.NewActiveList()

	builder := &prompt.Builder{
		PromptsFS:   promptsFS,
		WorkDir:     workDir,
		OS:          os,
		OSInfo:      platform.OSInfo(),
		HelperSetup: cfg.HelperSetup,
	}

	agent := &Agent{
		Config:      cfg,
		Modes:       modes,
		Session:     sess,
		Active:      active,
		Builder:     builder,
		Provider:    client,
		Handler:     handler,
		Compactor:   compaction.NewManager(cfg, client, summarizationClient),
		ModelID:     modelID,
		CurrentMode: currentMode,
		WorkDir:     workDir,
		OS:          os,
	}

	// Build resolver for skill tools: resolves names against current discovery.
	skillResolver := func(name string) (string, error) {
		all, err := skills.DiscoverAll(agent.WorkDir)
		if err != nil {
			return "", fmt.Errorf("skill discovery failed: %w", err)
		}
		return skills.Resolve(name, all)
	}
	runnableSkillResolver := func(name string) (string, *skills.Skill, error) {
		all, err := skills.DiscoverAll(agent.WorkDir)
		if err != nil {
			return "", nil, fmt.Errorf("skill discovery failed: %w", err)
		}
		resolved, err := skills.Resolve(name, all)
		if err != nil {
			return "", nil, err
		}
		skill := all[resolved]
		if skill == nil {
			return "", nil, fmt.Errorf("skill not found: %s", name)
		}
		return resolved, skill, nil
	}
	oneShotCaller := llmcall.New(cfg, func(cfg *config.Config, modelID string) (llmcall.StreamClient, error) {
		return provider.NewClient(cfg, modelID)
	})

	registry := tools.NewRegistry()
	registry.Register(tools.NewShellTool(os))
	registry.Register(tools.NewLoadSkillTool(active, skillResolver))
	registry.Register(tools.NewUnloadSkillTool(active, skillResolver))
	registry.Register(tools.NewRunSkillTool(os, runnableSkillResolver, func() string { return agent.WorkDir }))
	registry.Register(tools.NewAskFriendTool(func(ctx context.Context, args tools.AskFriendArgs) (string, error) {
		return oneShotCaller.Call(ctx, llmcall.Request{
			Role:         strings.TrimSpace(args.Role),
			Purpose:      strings.TrimSpace(args.Purpose),
			Question:     strings.TrimSpace(args.Question),
			Context:      strings.TrimSpace(args.Context),
			OutputFormat: strings.TrimSpace(args.OutputFormat),
		})
	}))
	registry.Register(tools.NewReplaceBlockTool(func() string { return agent.WorkDir }))
	registry.Register(tools.NewTaskWriteTool(func() string { return agent.WorkDir }))
	registry.Register(tools.NewTaskReadTool(func() string { return agent.WorkDir }))
	agent.Tools = registry

	return agent, nil
}

// RunTurn processes one user message: builds the prompt, streams the LLM response,
// executes any tool calls, persists all messages, and loops if tools were called.
//
// WHAT:  Executes one full conversation turn including tool call loop.
// WHY:   The REPL calls this for each user input.
// HOW:   Appends user message, builds prompt, streams response, executes tool calls,
//
//	appends assistant + tool result messages, loops if tools were called.
//
// PARAMS: ctx — turn cancellation context; userInput — the user's text input.
// RETURNS: error if the LLM call or tool execution fails fatally.
func (a *Agent) RunTurn(ctx context.Context, userInput string) error {
	if a.Handler == nil {
		return fmt.Errorf("runtime handler is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if err := a.sanitizeSession(); err != nil {
		return err
	}

	// Append user message to session.
	if err := a.Session.Append(session.Message{
		Role:    "user",
		Content: userInput,
	}); err != nil {
		return fmt.Errorf("cannot persist user message: %w", err)
	}

	for {
		if err := a.sanitizeSession(); err != nil {
			return err
		}

		// Build full prompt from disk + session history.
		messages, err := a.Builder.Build(a.Session, a.Active)
		if err != nil {
			return fmt.Errorf("cannot build prompt: %w", err)
		}

		// Strip reasoning parts from payload (keep newest N, global count).
		messages = a.Compactor.StripReasoningFromPayload(messages)

		// Write full built prompt to session folder for debugging.
		promptPath := filepath.Join(a.Session.Folder, "prompt.json")
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(messages); err == nil {
			raw := strings.ReplaceAll(buf.String(), "\\n", "\n")
			_ = os.WriteFile(promptPath, []byte(raw), 0644)
		}

		// Inject volatile mode directive into the latest user message only (copy, never mutate session).
		if a.CurrentMode != nil && strings.TrimSpace(a.CurrentMode.Directive) != "" {
			messages = injectDirective(messages, a.CurrentMode.Directive)
		}

		// Stream LLM response.
		toolDefs := tools.AllToOpenAI(a.Tools)
		resp, err := a.Provider.Stream(ctx, messages, toolDefs, a.Handler.OnContent)
		if err != nil && !errors.Is(err, provider.ErrAborted) {
			return fmt.Errorf("LLM stream failed: %w", err)
		}
		if resp == nil {
			resp = &provider.Response{}
		}

		// Report prompt token usage to the transport.
		if resp.Usage != nil && a.Handler != nil {
			a.Handler.OnUsage(resp.Usage.PromptTokens)
		}

		// Build assistant message.
		assistantMsg := session.Message{
			Role:      "assistant",
			Content:   resp.Content,
			Reasoning: resp.Reasoning,
		}
		if len(resp.ToolCalls) > 0 {
			openaiCalls := make([]tools.OpenAIToolCall, 0, len(resp.ToolCalls))
			for _, tc := range resp.ToolCalls {
				openaiCalls = append(openaiCalls, tools.ToOpenAIToolCall(tc))
			}
			assistantMsg.ToolCalls = openaiCalls
		}

		if shouldPersistAssistantMessage(assistantMsg) {
			if err := a.Session.Append(assistantMsg); err != nil {
				return fmt.Errorf("cannot persist assistant message: %w", err)
			}
		}

		if errors.Is(err, provider.ErrAborted) {
			if err := a.appendAbortedToolResults(resp.ToolCalls, 0); err != nil {
				return err
			}
			if err := a.appendAbortMarker(); err != nil {
				return err
			}
			return ErrTurnAborted
		}

		// No tool calls — check compaction and finish turn.
		if len(resp.ToolCalls) == 0 {
			if a.Compactor != nil {
				if _, err := a.Compactor.Compact(a.Session, resp.Usage); err != nil {
					return fmt.Errorf("compaction failed: %w", err)
				}
			}
			return nil
		}

		// Execute tool calls and append results.
		for idx, tc := range resp.ToolCalls {
			if ctx.Err() != nil {
				if err := a.appendAbortedToolResults(resp.ToolCalls, idx); err != nil {
					return err
				}
				if err := a.appendAbortMarker(); err != nil {
					return err
				}
				return ErrTurnAborted
			}
			// Reset sudo password before each tool call to prevent cross-call leaks.
			os.Unsetenv("BLAZE_SUDO_PASSWORD")

			// Detect sudo in shell commands and collect password.
			if tc.Name == "shell" {
				var shellArgs struct {
					Command string `json:"command"`
				}
				if err := json.Unmarshal([]byte(tc.Arguments), &shellArgs); err == nil && containsSudo(shellArgs.Command) {
					approved, password := a.Handler.RequestSudoApproval(shellArgs.Command)
					if !approved {
						result := "error: sudo command declined by user"
						a.Handler.OnToolResult(tc.Name, result)
						if err := a.Session.Append(session.Message{
							Role:       "tool",
							Content:    result,
							ToolCallID: tc.ID,
							Name:       tc.Name,
						}); err != nil {
							return fmt.Errorf("cannot persist tool error: %w", err)
						}
						continue
					}
					os.Setenv("BLAZE_SUDO_PASSWORD", password)
				}
			}

			tool := a.Tools.Get(tc.Name)
			if tool == nil {
				result := fmt.Sprintf("error: unknown tool: %s", tc.Name)
				a.Handler.OnToolResult(tc.Name, result)
				if err := a.Session.Append(session.Message{
					Role:       "tool",
					Content:    result,
					ToolCallID: tc.ID,
					Name:       tc.Name,
				}); err != nil {
					return fmt.Errorf("cannot persist tool error: %w", err)
				}
				continue
			}
			a.Handler.OnToolCall(tc.Name, a.Tools.FormatArgs(tc.Name, tc.Arguments))

			result := tool.Execute(ctx, tc.Arguments)
			a.Handler.OnToolResult(tc.Name, result)

			if err := a.Session.Append(session.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
				Name:       tc.Name,
			}); err != nil {
				return fmt.Errorf("cannot persist tool result: %w", err)
			}

			if ctx.Err() != nil {
				if err := a.appendAbortedToolResults(resp.ToolCalls, idx+1); err != nil {
					return err
				}
				if err := a.appendAbortMarker(); err != nil {
					return err
				}
				return ErrTurnAborted
			}
		}

		// Check compaction after tool execution — context may be at or over limit.
		if a.Compactor != nil {
			if _, err := a.Compactor.Compact(a.Session, resp.Usage); err != nil {
				return fmt.Errorf("compaction failed: %w", err)
			}
		}

		// Loop back to LLM with tool results included in session history.
	}
}

// shouldPersistAssistantMessage reports whether a partial or complete assistant message is worth saving.
func shouldPersistAssistantMessage(msg session.Message) bool {
	content, _ := msg.Content.(string)
	return content != "" || msg.Reasoning != "" || msg.ToolCalls != nil
}

// injectDirective appends the mode directive to the most recent user message in a copy of the
// slice. Tool messages must stay unchanged so tool results are sent back exactly as produced.
func injectDirective(messages []session.Message, directive string) []session.Message {
	if len(messages) == 0 {
		return messages
	}
	out := make([]session.Message, len(messages))
	copy(out, messages)
	for i := len(out) - 1; i >= 0; i-- {
		if out[i].Role != "user" {
			continue
		}
		content, _ := out[i].Content.(string)
		out[i].Content = content + "\n\n[MODE DIRECTIVE]\n" + directive
		return out
	}
	return out
}

// appendAbortMarker records the user's abort as a new user message in session history.
func (a *Agent) appendAbortMarker() error {
	if err := a.Session.Append(session.Message{Role: "user", Content: userAbortMessage}); err != nil {
		return fmt.Errorf("cannot persist abort marker: %w", err)
	}
	return nil
}

// appendAbortedToolResults adds tool result messages for unexecuted tool calls after an abort.
func (a *Agent) appendAbortedToolResults(toolCalls []tools.ToolCall, start int) error {
	if start < 0 {
		start = 0
	}
	for i := start; i < len(toolCalls); i++ {
		if err := a.Session.Append(session.Message{
			Role:       "tool",
			Content:    "aborted before execution by user",
			ToolCallID: toolCalls[i].ID,
			Name:       toolCalls[i].Name,
		}); err != nil {
			return fmt.Errorf("cannot persist aborted tool result: %w", err)
		}
	}
	return nil
}

// sanitizeSession removes any trailing incomplete tool-call round before an LLM call.
func (a *Agent) sanitizeSession() error {
	if err := a.Session.Sanitize(); err != nil {
		return fmt.Errorf("cannot sanitize session: %w", err)
	}
	if err := a.Session.Save(); err != nil {
		return fmt.Errorf("cannot save sanitized session: %w", err)
	}
	return nil
}

// applyModel validates a model ID, recreates the provider client, and updates in-memory runtime state.
//
// WHAT:  Applies a model change to the running agent without any persistence side effects.
// WHY:   Console persists globally, while future transports like Telegram need local-only switching.
// PARAMS: modelID — provider/model_name identifier to activate.
// RETURNS: error if model validation or provider client creation fails.
func (a *Agent) applyModel(modelID string) error {
	if err := validateModelInConfig(a.Config, modelID); err != nil {
		return err
	}
	client, err := provider.NewClient(a.Config, modelID)
	if err != nil {
		return fmt.Errorf("cannot create provider client: %w", err)
	}
	a.Provider = client
	a.ModelID = modelID
	if a.Compactor != nil {
		a.Compactor.Provider = client
	}
	return nil
}

// SetModel changes the current model, recreates the provider client, and persists the selection globally.
func (a *Agent) SetModel(modelID string) error {
	if err := a.applyModel(modelID); err != nil {
		return err
	}
	if a.CurrentMode != nil {
		a.CurrentMode.Model = modelID
		if err := a.Modes.Save(); err != nil {
			return fmt.Errorf("cannot persist mode model selection: %w", err)
		}
		return nil
	}
	a.Config.LastModel = modelID
	if err := a.Config.Save(); err != nil {
		return fmt.Errorf("cannot persist legacy model selection: %w", err)
	}
	return nil
}

// SetModelLocal changes the current model only in runtime memory.
//
// WHAT:  Applies a model switch without writing to global config or modes state.
// WHY:   Non-console transports can persist model selection in their own instance state.
// PARAMS: modelID — provider/model_name identifier to activate.
// RETURNS: error if model validation or provider client creation fails.
func (a *Agent) SetModelLocal(modelID string) error {
	return a.applyModel(modelID)
}

// SetWorkDir changes the current work folder and updates the prompt builder.
func (a *Agent) SetWorkDir(dir string) error {
	if dir == "" {
		return fmt.Errorf("path is empty")
	}
	if !isDir(dir) {
		return fmt.Errorf("not a directory: %s", dir)
	}
	a.WorkDir = dir
	a.Builder.WorkDir = dir
	return nil
}

// ListProviderModels fetches the list of available model IDs from a configured provider.
func (a *Agent) ListProviderModels(providerName string) ([]string, error) {
	for _, p := range a.Config.Providers {
		if p.Name == providerName {
			client := provider.NewClientRaw(p.Endpoint, p.APIKey)
			return client.ListModels()
		}
	}
	return nil, fmt.Errorf("provider not found: %s", providerName)
}

// SetMode switches the active work mode by name.
func (a *Agent) SetMode(name string) error {
	for i := range a.Modes.Modes {
		if a.Modes.Modes[i].Name == name {
			mode := &a.Modes.Modes[i]
			if err := a.applyModel(mode.Model); err != nil {
				return fmt.Errorf("cannot apply provider client for mode %q: %w", name, err)
			}
			a.CurrentMode = mode
			a.Modes.LastMode = name
			if err := a.Modes.Save(); err != nil {
				return fmt.Errorf("cannot persist mode switch: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("mode not found: %s", name)
}

// NextMode returns the next mode in the config list cyclically.
func (a *Agent) NextMode() (*config.Mode, error) {
	if len(a.Modes.Modes) == 0 {
		return nil, fmt.Errorf("no modes configured")
	}
	if a.CurrentMode == nil {
		mode := &a.Modes.Modes[0]
		if err := a.SetMode(mode.Name); err != nil {
			return nil, err
		}
		return a.CurrentMode, nil
	}
	for i := range a.Modes.Modes {
		if a.Modes.Modes[i].Name == a.CurrentMode.Name {
			nextIdx := (i + 1) % len(a.Modes.Modes)
			next := &a.Modes.Modes[nextIdx]
			if err := a.SetMode(next.Name); err != nil {
				return nil, err
			}
			return a.CurrentMode, nil
		}
	}
	mode := &a.Modes.Modes[0]
	if err := a.SetMode(mode.Name); err != nil {
		return nil, err
	}
	return a.CurrentMode, nil
}

// CloseSession marks the session as cleanly closed.
func (a *Agent) CloseSession() error {
	return a.Session.Close()
}

// ResetConversation clears the current session history and loaded context.
//
// WHAT:  Restarts the current session in place without changing its folder name.
// WHY:   /clear and /new need a clean prompt state with only the sysprompt and no active skills.
// RETURNS: error if clearing summaries or persisting the reset session fails.
func (a *Agent) ResetConversation() error {
	if a.Compactor != nil {
		if err := a.Compactor.ClearSummaries(a.Session.Folder); err != nil {
			return fmt.Errorf("cannot clear summaries: %w", err)
		}
	}
	if err := os.Remove(filepath.Join(a.Session.Folder, "prompt.json")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot clear prompt debug file: %w", err)
	}
	if a.Active != nil {
		a.Active.Clear()
	}
	if err := a.Session.Reset(); err != nil {
		return fmt.Errorf("cannot reset session: %w", err)
	}
	return nil
}

// validateModelInConfig checks that a model ID exists in the config's providers and favorite models.
func validateModelInConfig(cfg *config.Config, modelID string) error {
	if err := validateModelFormat(modelID); err != nil {
		return err
	}
	providerName, _ := config.SplitModelID(modelID)
	if cfg.ProviderByName(providerName) == nil {
		return fmt.Errorf("provider not found: %s", providerName)
	}
	return nil
}

// validateModelFormat checks the provider/model_name format.
func validateModelFormat(model string) error {
	idx := strings.Index(model, "/")
	if idx <= 0 || idx == len(model)-1 {
		return fmt.Errorf("model must be in provider/model_name format")
	}
	if strings.Contains(model[idx+1:], "/") {
		return fmt.Errorf("model must be in provider/model_name format")
	}
	return nil
}

// isDir checks if a path exists and is a directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// containsSudo reports whether a shell command string invokes sudo.
//
// WHAT:  Checks for sudo at a shell command boundary (start of line, after pipe, semicolon, or logic operator).
// WHY:   Sudo commands need password approval before execution.
// RETURNS: bool — true if the command contains a sudo invocation.
func containsSudo(command string) bool {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return false
	}
	if strings.HasPrefix(cmd, "sudo ") || cmd == "sudo" {
		return true
	}
	for _, sep := range []string{"| sudo ", "| sudo\t", "; sudo ", "; sudo\t", "&& sudo ", "&& sudo\t", "|| sudo ", "|| sudo\t"} {
		if strings.Contains(cmd, sep) {
			return true
		}
	}
	return false
}
