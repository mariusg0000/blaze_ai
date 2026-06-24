// runtime.go — agent core orchestration loop and handler contract.
// Defines the Handler interface (the only boundary between agent core and transports),
// the Agent struct that ties all packages together, and the RunTurn loop that drives
// prompt building, LLM streaming, tool execution, and message persistence.
// Layer: agent core. Dependencies: internal/config, internal/provider, internal/prompt,
// internal/session, internal/tools, internal/skills, internal/memories.
package runtime

import (
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
	"blazeai/internal/memories"
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
// The agent core calls these methods during execution; each transport implements them.
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
}

// Agent is the core runtime that ties all packages together and drives the conversation loop.
//
// WHAT:  Holds all runtime state and orchestrates the LLM call cycle.
// WHY:   One struct ties config, session, skills, memories, prompt, tools, and provider together.
// PARAMS: Config — loaded configuration; Session — current conversation session;
//
//	Active — in-memory active skills list; Memories — in-memory active memory list; Builder — prompt assembler;
//	Tools — tool registry; Provider — LLM client; Handler — transport callbacks;
//	ModelID — current provider/model_name; WorkDir — current work folder; OS — detected platform.
type Agent struct {
	Config    *config.Config
	Session   *session.Session
	Active    *skills.ActiveList
	Memories  *memories.ActiveList
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
// Initializes the prompt builder, tool registry, provider client, and active skill/memory lists.
//
// WHAT:  Constructs the runtime agent with all dependencies wired.
// WHY:   The main entrypoint calls this to assemble the agent before starting the REPL.
// PARAMS: cfg — loaded config; sess — session (new or resumed); os — detected platform;
//
//	builtinSkillsFS — filesystem with builtin skill .md files;
//	promptsFS — filesystem with sysprompt.md and sysprompt.<os>.md;
//	workDir — initial work folder; handler — transport implementation.
//
// RETURNS: *Agent — ready to run; error if provider client cannot be created.
func NewAgent(cfg *config.Config, sess *session.Session, os platform.OS, builtinSkillsFS, promptsFS fs.FS, workDir string, handler Handler) (*Agent, error) {
	modelID := cfg.LastModel
	if modelID == "" {
		modelID = cfg.Roles.Default
	}

	// Auto-create default mode if no modes exist.
	if len(cfg.Modes) == 0 {
		cfg.Modes = []config.Mode{
			{Name: "default", Model: modelID},
		}
		cfg.LastMode = "default"
		// Best-effort save: ignore error if config is read-only.
		_ = cfg.Save()
	}

	// Resolve active mode: mode model takes priority over LastModel.
	var currentMode *config.Mode
	if cfg.LastMode != "" {
		for i := range cfg.Modes {
			if cfg.Modes[i].Name == cfg.LastMode {
				currentMode = &cfg.Modes[i]
				break
			}
		}
	}
	if currentMode != nil {
		modelID = currentMode.Model
	} else if len(cfg.Modes) > 0 {
		currentMode = &cfg.Modes[0]
		modelID = currentMode.Model
	}

	client, err := provider.NewClient(cfg, modelID)
	if err != nil {
		return nil, fmt.Errorf("cannot create provider client: %w", err)
	}

	active := skills.NewActiveList()
	memoriesList := memories.NewActiveList()

	registry := tools.NewRegistry()
	registry.Register(tools.NewShellTool(os))
	registry.Register(tools.NewLoadSkillTool(active))
	registry.Register(tools.NewUnloadSkillTool(active))
	registry.Register(tools.NewLoadMemoryTool(memoriesList))
	registry.Register(tools.NewUnloadMemoryTool(memoriesList))
	registry.Register(tools.NewReplaceBlockTool())

	builder := &prompt.Builder{
		PromptsFS:       promptsFS,
		BuiltinSkillsFS: builtinSkillsFS,
		WorkDir:         workDir,
		OS:              os,
		OSInfo:          platform.OSInfo(),
		HelperSetup:     cfg.HelperSetup,
	}

	agent := &Agent{
		Config:      cfg,
		Session:     sess,
		Active:      active,
		Memories:    memoriesList,
		Builder:     builder,
		Tools:       registry,
		Provider:    client,
		Handler:     handler,
		Compactor:   compaction.NewManager(cfg, client),
		ModelID:     modelID,
		CurrentMode: currentMode,
		WorkDir:     workDir,
		OS:          os,
	}

	registry.Register(tools.NewTaskWriteTool(func() string { return agent.WorkDir }))
	registry.Register(tools.NewTaskReadTool(func() string { return agent.WorkDir }))

	return agent, nil
}

// RunTurn processes one user message: builds the prompt, streams the LLM response,
// executes any tool calls, persists all messages, and loops if tools were called.
// Returns nil when the turn completes (no more pending tool calls).
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
		messages, err := a.Builder.Build(a.Session, a.Active, a.Memories)
		if err != nil {
			return fmt.Errorf("cannot build prompt: %w", err)
		}

		// Strip reasoning parts from payload (keep newest N, global count).
		messages = a.Compactor.StripReasoningFromPayload(messages)

		// Write full built prompt to session folder for debugging.
		promptPath := filepath.Join(a.Session.Folder, "prompt.json")
		data, err := json.MarshalIndent(messages, "", "  ")
		if err == nil {
			raw := strings.ReplaceAll(string(data), "\\n", "\n")
			_ = os.WriteFile(promptPath, []byte(raw), 0644)
		}

		// Inject volatile mode directive into the last message (copy, never mutate session).
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
			a.Handler.OnToolCall(tc.Name, a.Tools.FormatArgs(tc.Name, tc.Arguments))

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

		// Loop back to LLM with tool results included in session history.
	}
}

// shouldPersistAssistantMessage reports whether a partial or complete assistant message is worth saving.
func shouldPersistAssistantMessage(msg session.Message) bool {
	content, _ := msg.Content.(string)
	return content != "" || msg.Reasoning != "" || msg.ToolCalls != nil
}

// injectDirective appends the mode directive to the last message in a copy of the slice.
// The original messages slice is never mutated — only the returned copy is modified.
//
// WHAT:  Appends a volatile mode directive to the last message's content.
// WHY:   The directive is never persisted in session.json but must reach the LLM.
// PARAMS: messages — the prompt messages; directive — the mode directive text.
// RETURNS: []session.Message — a copy with the directive appended to the last element.
func injectDirective(messages []session.Message, directive string) []session.Message {
	if len(messages) == 0 {
		return messages
	}
	out := make([]session.Message, len(messages))
	copy(out, messages)
	last := &out[len(out)-1]
	content, _ := last.Content.(string)
	last.Content = content + "\n\n[MODE DIRECTIVE]\n" + directive
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
//
// WHAT:  Ensures the persisted session is valid for provider APIs before prompt build/stream.
// WHY:   Interrupted turns can leave an assistant tool_calls message without matching tool results.
// HOW:   Sanitizes the in-memory session and persists it immediately.
// RETURNS: error if sanitizing or saving fails.
func (a *Agent) sanitizeSession() error {
	if err := a.Session.Sanitize(); err != nil {
		return fmt.Errorf("cannot sanitize session: %w", err)
	}
	if err := a.Session.Save(); err != nil {
		return fmt.Errorf("cannot save sanitized session: %w", err)
	}
	return nil
}

// SetModel changes the current model and recreates the provider client.
// The model ID must be a valid provider/model_name in config.
//
// WHAT:  Switches the active model at runtime.
// WHY:   /model command calls this to change the model.
// PARAMS: modelID — full provider/model_name identifier.
// RETURNS: error if the model is invalid or provider client cannot be created.
func (a *Agent) SetModel(modelID string) error {
	if err := validateModelInConfig(a.Config, modelID); err != nil {
		return err
	}
	client, err := provider.NewClient(a.Config, modelID)
	if err != nil {
		return fmt.Errorf("cannot create provider client: %w", err)
	}
	a.Provider = client
	a.ModelID = modelID
	a.Config.LastModel = modelID
	// If a mode is active, update the mode's model so it persists with the mode.
	if a.CurrentMode != nil {
		a.CurrentMode.Model = modelID
	}
	if err := a.Config.Save(); err != nil {
		return fmt.Errorf("cannot persist model selection: %w", err)
	}
	return nil
}

// SetWorkDir changes the current work folder and updates the prompt builder.
//
// WHAT:  Changes the work folder for tool execution and AGENTS.md resolution.
// WHY:   /cd command calls this to change the work folder.
// PARAMS: dir — the new work directory path.
// RETURNS: error if the path is invalid.
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

// ReloadModes re-reads modes from config.json on disk to pick up hot changes.
//
// WHAT:  Hot-reloads the modes list from the persisted config file.
// WHY:   When the skill creates/edits modes at runtime, the in-memory config is stale.
// HOW:   Delegates to Config.ReloadModesFromDisk() which re-reads and validates.
// RETURNS: error if the file cannot be read or modes are invalid.
func (a *Agent) ReloadModes() error {
	return a.Config.ReloadModesFromDisk()
}

// ListProviderModels fetches the list of available model IDs from a configured provider.
//
// WHAT:  Creates a raw client for the named provider and calls its /models endpoint.
// WHY:   Interactive /model command needs to show available models for user selection.
// HOW:   Finds the provider in config, calls provider.NewClientRaw + ListModels.
// PARAMS: providerName — the name of a configured provider.
// RETURNS: []string — sorted model IDs; error if provider not found or fetch fails.
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
// Updates the model, recreates the provider client, and persists LastMode to config.
//
// WHAT:  Changes the active work mode at runtime.
// WHY:   Tab cycling and /mode commands call this to switch modes.
// PARAMS: name — the mode name to activate.
// RETURNS: error if mode not found or model invalid.
func (a *Agent) SetMode(name string) error {
	// Hot-reload modes from disk to pick up changes made by the skill.
	if err := a.ReloadModes(); err != nil {
		return fmt.Errorf("cannot reload modes: %w", err)
	}
	for i := range a.Config.Modes {
		if a.Config.Modes[i].Name == name {
			mode := &a.Config.Modes[i]
			client, err := provider.NewClient(a.Config, mode.Model)
			if err != nil {
				return fmt.Errorf("cannot create provider client for mode %q: %w", name, err)
			}
			a.Provider = client
			a.ModelID = mode.Model
			a.CurrentMode = mode
			a.Config.LastMode = name
			a.Config.LastModel = mode.Model
			if err := a.Config.Save(); err != nil {
				return fmt.Errorf("cannot persist mode switch: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("mode not found: %s", name)
}

// NextMode returns the next mode in the config list cyclically.
//
// WHAT:  Cycles to the next work mode.
// WHY:   Tab key calls this to switch to the next mode.
// HOW:   Hot-reloads modes from disk first, then advances cyclically.
// RETURNS: *config.Mode — the next mode; error if no modes are configured.
func (a *Agent) NextMode() (*config.Mode, error) {
	// Hot-reload modes from disk to pick up changes made by the skill.
	if err := a.ReloadModes(); err != nil {
		return nil, fmt.Errorf("cannot reload modes: %w", err)
	}
	if len(a.Config.Modes) == 0 {
		return nil, fmt.Errorf("no modes configured")
	}
	if a.CurrentMode == nil {
		mode := &a.Config.Modes[0]
		if err := a.SetMode(mode.Name); err != nil {
			return nil, err
		}
		return a.CurrentMode, nil
	}
	// Find current index and advance cyclically.
	for i := range a.Config.Modes {
		if a.Config.Modes[i].Name == a.CurrentMode.Name {
			nextIdx := (i + 1) % len(a.Config.Modes)
			next := &a.Config.Modes[nextIdx]
			if err := a.SetMode(next.Name); err != nil {
				return nil, err
			}
			return a.CurrentMode, nil
		}
	}
	// Current mode not in list (corrupted state) — reset to first.
	mode := &a.Config.Modes[0]
	if err := a.SetMode(mode.Name); err != nil {
		return nil, err
	}
	return a.CurrentMode, nil
}

// CloseSession marks the session as cleanly closed.
//
// WHAT:  Clean close for /exit.
// RETURNS: error if persisting fails.
func (a *Agent) CloseSession() error {
	return a.Session.Close()
}

// ResetConversation clears the current session history and loaded context.
//
// WHAT:  Restarts the current session in place without changing its folder name.
// WHY:   /clear and /new need a clean prompt state with only the sysprompt and no active skills or memories.
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
	if a.Memories != nil {
		a.Memories.Clear()
	}
	if err := a.Session.Reset(); err != nil {
		return fmt.Errorf("cannot reset session: %w", err)
	}
	return nil
}

// CloseSession marks the session as cleanly closed.

// validateModelInConfig checks that a model ID exists in the config's providers and favorite models.
//
// WHAT:  Validates that a model ID is known to config.
// WHY:   /model must reject models not in config.
// PARAMS: cfg — loaded config; modelID — the model to validate.
// RETURNS: error if the model format is invalid or provider is unknown.
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
