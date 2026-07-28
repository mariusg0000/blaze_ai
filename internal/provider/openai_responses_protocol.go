package provider

import (
	"fmt"

	"blazeai/internal/config"
)

type openAIResponsesProtocol struct{}

func (p openAIResponsesProtocol) ID() string {
	return config.ProtocolOpenAIResponses
}

func (p openAIResponsesProtocol) Validate(req Request) error {
	if req.Model.Definition.Protocol != config.ProtocolOpenAIResponses {
		return fmt.Errorf("request protocol must be %q", config.ProtocolOpenAIResponses)
	}
	if req.Model.Definition.Responses == nil {
		return fmt.Errorf("responses variant is required")
	}
	if len(req.Tools) > 0 && !req.Model.Definition.Capabilities.Tools {
		return fmt.Errorf("model does not support tools")
	}
	return nil
}

func (p openAIResponsesProtocol) Lower(req Request) (any, error) {
	if err := p.Validate(req); err != nil {
		return nil, err
	}
	if req.Model.Name == "" {
		return nil, fmt.Errorf("model name is required")
	}
	if req.Model.Definition.Responses.Lite {
		return buildChatGPTLiteRequest(req)
	}
	return buildChatGPTRequest(req)
}
