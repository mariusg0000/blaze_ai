package provider

import (
	"fmt"

	"blazeai/internal/config"
	"blazeai/internal/session"
	"blazeai/internal/tools"
)

type openAIChatProtocol struct{}

type openAIChatRequest struct {
	Model         string             `json:"model"`
	Messages      []session.Message  `json:"messages"`
	Tools         []tools.OpenAITool `json:"tools,omitempty"`
	Stream        bool               `json:"stream"`
	StreamOptions *streamOptions     `json:"stream_options,omitempty"`
}

func (p openAIChatProtocol) ID() string {
	return config.ProtocolOpenAIChat
}

func (p openAIChatProtocol) Validate(req Request) error {
	if req.Model.Definition.Protocol != config.ProtocolOpenAIChat {
		return fmt.Errorf("request protocol must be %q", config.ProtocolOpenAIChat)
	}
	if req.Model.Definition.OpenAIChat == nil {
		return fmt.Errorf("openai-chat variant is required")
	}
	if len(req.Tools) > 0 && !req.Model.Definition.Capabilities.Tools {
		return fmt.Errorf("model does not support tools")
	}
	return nil
}

func (p openAIChatProtocol) Lower(req Request) (any, error) {
	if err := p.Validate(req); err != nil {
		return nil, err
	}
	if req.Model.Name == "" {
		return nil, fmt.Errorf("model name is required")
	}

	messages := append([]session.Message(nil), req.Messages...)
	variant := req.Model.Definition.OpenAIChat
	if !variant.IncludeReasoningContent {
		for i := range messages {
			messages[i].Reasoning = ""
			messages[i].ReasoningEncrypted = ""
			messages[i].ReasoningPresent = false
		}
	}

	native := openAIChatRequest{
		Model:    req.Model.Name,
		Messages: messages,
		Tools:    req.Tools,
		Stream:   req.Stream,
	}
	if variant.IncludeStreamUsage {
		native.StreamOptions = &streamOptions{IncludeUsage: true}
	}
	return native, nil
}
