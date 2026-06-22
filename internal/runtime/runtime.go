// runtime.go — agent core orchestration loop and handler contract.
// Defines the Handler interface (the only boundary between agent core and transports),
// the Agent struct that ties all packages together, and the RunTurn loop that drives
// prompt building, LLM streaming, tool execution, and message persistence.
// Layer: agent core. Dependencies: internal/config, internal/provider, internal/prompt,
// internal/session, internal/tools, internal/skills.
package runtime

import (
	"encoding/json"
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

// Handler is the contract between the agent core and transports.
// The agent core calls these methods during execution; each transport implements them.
//
// WHAT:  The only boundary between agent core and user-facing transport.
// WHY:   Console and web both implement this interface over the same core.
type Handler interface {
	// OnContent is called for each streaming text chunk from the LLM.
	OnContent(delta string)
	// OnToolCall is called before a tool is executed.
	OnToolCall(name string, args json.RawMessage)
	// OnToolResult is called after a tool has finished.
	OnToolResult(name string, result string)
}

// Agent is the core runtime that ties all packages together and drives the conversation loop.
//
// WHAT:  Holds all runtime state and orchestrates the LLM call cycle.
// WHY:   One struct ties config, session, skills, prompt, tools, and provider together.
// PARAMS: Config — loaded configuration; Session — current conversation session;
//         Active — in-memory active skills list; Builder — prompt assembler;
//         Tools — tool registry; Provider — LLM client; Handler — transport callbacks;
//         ModelID — current provider/model_name; WorkDir — current work folder; OS — detected platform.
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
//         builtinSkillsDir — path to builtin skills; promptsDir — path to prompt files;
//         workDir — initial work folder; handler — transport implementation.
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
//         appends assistant + tool result messages, loops if tools were called.
// PARAMS: userInput — the user's text input.
// RETURNS: error if the LLM call or tool execution fails fatally.
func (a *Agent) RunTurn(userInput string) error {
	// Append user message to session.
	if err := a.Session.Append(session.Message{
		Role:    "user",
		Content: userInput,
	}); err != nil {
		return fmt.Errorf("cannot persist user message: %w", err)
	}

	for {
		// Build full prompt from disk + session history.
		messages, err := a.Builder.Build(a.Session, a.Active)
		if err != nil {
			return fmt.Errorf("cannot build prompt: %w", err)
		}

		// Stream LLM response.
		toolDefs := tools.AllToOpenAI(a.Tools)
		resp, err := a.Provider.Stream(messages, toolDefs, a.Handler.OnContent)
		if err != nil {
			return fmt.Errorf("LLM stream failed: %w", err)
		}

		// Build assistant message.
		assistantMsg := session.Message{
			Role:    "assistant",
			Content: resp.Content,
		}
		if len(resp.ToolCalls) > 0 {
			assistantMsg.ToolCalls = resp.ToolCalls
		}

		// Persist assistant message.
		if err := a.Session.Append(assistantMsg); err != nil {
			return fmt.Errorf("cannot persist assistant message: %w", err)
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
		for _, tc := range resp.ToolCalls {
			a.Handler.OnToolCall(tc.Name, tc.Arguments)

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

			result := tool.Execute(tc.Arguments)
			a.Handler.OnToolResult(tc.Name, result)

			if err := a.Session.Append(session.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
				Name:       tc.Name,
			}); err != nil {
				return fmt.Errorf("cannot persist tool result: %w", err)
			}
		}

		// Loop back to LLM with tool results included in session history.
	}
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
