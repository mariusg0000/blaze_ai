// llmcall.go — one-shot secondary LLM consultation helper.
// Resolves a configured role to a model, builds a strict no-tools prompt, and returns
// plain text for ask_a_friend or other focused review paths.
// Layer: secondary LLM calling. Dependencies: internal/config, internal/provider,
// internal/session, internal/tools.
package llmcall

import (
	"context"
	"fmt"
	"strings"

	"blazeai/internal/config"
	"blazeai/internal/provider"
	"blazeai/internal/session"
	"blazeai/internal/tools"
)

// StreamClient is the narrow provider surface needed for one-shot consultation.
type StreamClient interface {
	Stream(ctx context.Context, messages []session.Message, toolDefs []tools.OpenAITool, onContent func(string)) (*provider.Response, error)
}

// ClientFactory builds a provider client for a concrete provider/model_name identifier.
type ClientFactory func(cfg *config.Config, modelID string) (StreamClient, error)

// Request describes one one-shot delegated LLM call.
//
// WHAT:  Bundles the user-facing consultation inputs for a secondary model.
// WHY:   ask_a_friend and future review helpers need one stable request shape.
// PARAMS: Role — configured model role; Purpose — concise objective; Question — focused ask;
//
//	Context — supporting evidence; OutputFormat — exact requested answer format.
type Request struct {
	Role         string
	Purpose      string
	Question     string
	Context      string
	OutputFormat string
}

// Caller resolves configured roles and performs strict one-shot model calls.
//
// WHAT:  Holds config and client construction for secondary LLM consultation.
// WHY:   Tool code should delegate provider setup and prompt building to one place.
// PARAMS: cfg — runtime config; factory — provider client constructor.
type Caller struct {
	cfg     *config.Config
	factory ClientFactory
}

// New creates a role-based one-shot caller.
func New(cfg *config.Config, factory ClientFactory) *Caller {
	return &Caller{cfg: cfg, factory: factory}
}

// Call sends one strict text-only consultation to the configured role model.
//
// WHAT:  Builds a no-tools request and returns the final assistant text.
// WHY:   Secondary consultation must stay narrow, deterministic, and non-recursive.
// PARAMS: ctx — cancellation context; req — role, purpose, question, context, and format.
// RETURNS: string — consultant answer; error if config, provider, or response is invalid.
func (c *Caller) Call(ctx context.Context, req Request) (string, error) {
	if c == nil {
		return "", fmt.Errorf("secondary LLM caller is nil")
	}
	if c.cfg == nil {
		return "", fmt.Errorf("secondary LLM config is nil")
	}
	if c.factory == nil {
		return "", fmt.Errorf("secondary LLM client factory is nil")
	}
	modelID, err := c.cfg.ModelForRole(req.Role)
	if err != nil {
		return "", err
	}
	client, err := c.factory(c.cfg, modelID)
	if err != nil {
		return "", fmt.Errorf("cannot create secondary LLM client: %w", err)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	resp, err := client.Stream(ctx, buildMessages(req), nil, nil)
	if err != nil {
		return "", fmt.Errorf("secondary LLM call failed: %w", err)
	}
	if resp == nil {
		return "", fmt.Errorf("secondary LLM returned no response")
	}
	if len(resp.ToolCalls) > 0 {
		return "", fmt.Errorf("secondary LLM attempted tool calls; ask_a_friend forbids delegated tools")
	}
	content := strings.TrimSpace(resp.Content)
	if content == "" {
		return "", fmt.Errorf("secondary LLM returned empty content")
	}
	return content, nil
}

// buildMessages creates the strict system and user messages for one-shot consultation.
func buildMessages(req Request) []session.Message {
	return []session.Message{
		{
			Role: "system",
			Content: strings.TrimSpace(`You are a focused expert consultant.
Return only the requested answer.
Do not call tools.
Do not invent hidden steps.
If the supplied context is insufficient, say so briefly and explain exactly what is missing.`),
		},
		{
			Role: "user",
			Content: strings.TrimSpace(fmt.Sprintf(`Purpose:
%s

Question:
%s

Context:
%s

Required output format:
%s`, req.Purpose, req.Question, req.Context, req.OutputFormat)),
		},
	}
}
