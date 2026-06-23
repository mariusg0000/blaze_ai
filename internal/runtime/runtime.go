// runtime.go — agent core orchestration loop and handler contract.
// Defines the Handler interface (the only boundary between agent core and transports),
// the Agent struct that ties all packages together, and the RunTurn loop that drives
// prompt building, LLM streaming, tool execution, and message persistence.
// Layer: agent core. Dependencies: internal/config, internal/provider, internal/prompt,
// internal/session, internal/tools, internal/skills.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"blazeai/internal/compaction"
	"blazeai/internal/config"
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
// WHY:   One struct ties config, session, skills, prompt, tools, and provider together.
// PARAMS: Config — loaded configuration; Session — current conversation session;
//
//	Active — in-memory active skills list; Builder — prompt assembler;
//	Tools — tool registry; Provider — LLM client; Handler — transport callbacks;
//	ModelID — current provider/model_name; WorkDir — current work folder; OS — detected platform.
type Agent struct {
	Config    *config.Config
	Session   *session.Session
	Active    *skills.ActiveList
	Builder   *prompt.Builder
	Tools     *tools.Registry
	Provider  *provider.Client
	Handler   Handler
	Compactor *compaction.Manager

	ModelID string
	WorkDir string
	OS      platform.OS
}

// NewAgent creates an Agent from a loaded config, session, and detected OS.
// Initializes the prompt builder, tool registry, provider client, and active skills list.
//
// WHAT:  Constructs the runtime agent with all dependencies wired.
// WHY:   The main entrypoint calls this to assemble the agent before starting the REPL.
// PARAMS: cfg — loaded config; sess — session (new or resumed); os — detected platform;
//
//	builtinSkillsDir — path to builtin skills; promptsDir — path to prompt files;
//	workDir — initial work folder; handler — transport implementation.
//
// RETURNS: *Agent — ready to run; error if provider client cannot be created.
func NewAgent(cfg *config.Config, sess *session.Session, os platform.OS, builtinSkillsDir, promptsDir, workDir string, handler Handler) (*Agent, error) {
	modelID := cfg.LastModel
	if modelID == "" {
		modelID = cfg.Roles.Default
	}

	client, err := provider.NewClient(cfg, modelID)
	if err != nil {
		return nil, fmt.Errorf("cannot create provider client: %w", err)
	}

	registry := tools.NewRegistry()
	registry.Register(tools.NewShellTool(os))
	registry.Register(tools.NewReplaceBlockTool())

	active := skills.NewActiveList()

	builder := &prompt.Builder{
		PromptsDir:       promptsDir,
		BuiltinSkillsDir: builtinSkillsDir,
		WorkDir:          workDir,
		OS:               os,
		OSInfo:           platform.OSInfo(),
		HelperSetup:      cfg.HelperSetup,
	}

	return &Agent{
		Config:    cfg,
		Session:   sess,
		Active:    active,
		Builder:   builder,
		Tools:     registry,
		Provider:  client,
		Handler:   handler,
		Compactor: compaction.NewManager(cfg, client),
		ModelID:   modelID,
		WorkDir:   workDir,
		OS:        os,
	}, nil
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
		messages, err := a.Builder.Build(a.Session, a.Active)
		if err != nil {
			return fmt.Errorf("cannot build prompt: %w", err)
		}

		// Strip reasoning parts from payload (keep newest N, global count).
		messages = a.Compactor.StripReasoningFromPayload(messages)

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

// CloseSession marks the session as cleanly closed.
//
// WHAT:  Clean close for /exit.
// RETURNS: error if persisting fails.
func (a *Agent) CloseSession() error {
	return a.Session.Close()
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
