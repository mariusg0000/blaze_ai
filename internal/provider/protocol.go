package provider

import (
	"blazeai/internal/config"
	"blazeai/internal/session"
	"blazeai/internal/tools"
)

type ModelReference struct {
	ID         string
	Name       string
	Definition config.ModelDefinition
}

type Request struct {
	Model           ModelReference
	Messages        []session.Message
	Tools           []tools.OpenAITool
	Stream          bool
	ProviderOptions map[string]any
}

type Protocol interface {
	ID() string
	Validate(Request) error
	Lower(Request) (any, error)
}
