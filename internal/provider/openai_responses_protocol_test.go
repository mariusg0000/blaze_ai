package provider

import (
	"encoding/json"
	"strings"
	"testing"

	"blazeai/internal/config"
	"blazeai/internal/session"
	"blazeai/internal/tools"
)

func TestOpenAIResponsesProtocolLowerNormal(t *testing.T) {
	req := Request{
		Model: ModelReference{
			Name: "gpt-5.4",
			Definition: config.ModelDefinition{
				Protocol:     config.ProtocolOpenAIResponses,
				Capabilities: config.ModelCapabilities{Tools: true},
				Responses:    &config.ResponsesVariant{Lite: false},
			},
		},
		Messages: []session.Message{{Role: "user", Content: "hello"}},
		Tools: []tools.OpenAITool{{
			Type: "function",
			Function: tools.FunctionDef{
				Name:        "lookup",
				Description: "Look up a value",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		}},
	}

	native, err := (openAIResponsesProtocol{}).Lower(req)
	if err != nil {
		t.Fatalf("Lower() error = %v", err)
	}
	data, err := json.Marshal(native)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	jsonText := string(data)
	for _, expected := range []string{
		`"model":"gpt-5.4"`,
		`"input":[`,
		`"tools":[`,
		`"store":false`,
		`"include":["reasoning.encrypted_content"]`,
		`"text":{"verbosity":"low"}`,
	} {
		if !strings.Contains(jsonText, expected) {
			t.Errorf("lowered JSON missing %s: %s", expected, jsonText)
		}
	}
}

func TestOpenAIResponsesProtocolLowerLite(t *testing.T) {
	req := Request{
		Model: ModelReference{
			Name: "gpt-5.6-luna",
			Definition: config.ModelDefinition{
				Protocol:     config.ProtocolOpenAIResponses,
				Capabilities: config.ModelCapabilities{Tools: true},
				Responses:    &config.ResponsesVariant{Lite: true},
			},
		},
		Messages: []session.Message{{Role: "system", Content: "instructions"}, {Role: "user", Content: "hello"}},
		Tools: []tools.OpenAITool{{
			Type:     "function",
			Function: tools.FunctionDef{Name: "lookup", Parameters: json.RawMessage(`{"type":"object"}`)},
		}},
	}

	native, err := (openAIResponsesProtocol{}).Lower(req)
	if err != nil {
		t.Fatalf("Lower() error = %v", err)
	}
	data, err := json.Marshal(native)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	jsonText := string(data)
	for _, expected := range []string{
		`"model":"gpt-5.6-luna"`,
		`"type":"additional_tools"`,
		`"reasoning":{"context":"all_turns"}`,
		`"store":false`,
	} {
		if !strings.Contains(jsonText, expected) {
			t.Errorf("lowered JSON missing %s: %s", expected, jsonText)
		}
	}
}

func TestOpenAIResponsesProtocolLowerErrors(t *testing.T) {
	base := Request{
		Model: ModelReference{
			Name: "model",
			Definition: config.ModelDefinition{
				Protocol:     config.ProtocolOpenAIResponses,
				Capabilities: config.ModelCapabilities{Tools: false},
				Responses:    &config.ResponsesVariant{},
			},
		},
		Tools: []tools.OpenAITool{{Type: "function"}},
	}

	tests := []struct {
		name string
		req  Request
	}{
		{name: "nil responses variant", req: func() Request {
			req := base
			req.Tools = nil
			req.Model.Definition.Responses = nil
			return req
		}()},
		{name: "other protocol", req: func() Request {
			req := base
			req.Tools = nil
			req.Model.Definition.Protocol = config.ProtocolOpenAIChat
			return req
		}()},
		{name: "empty model name", req: func() Request {
			req := base
			req.Tools = nil
			req.Model.Name = ""
			return req
		}()},
		{name: "tools disabled", req: base},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := (openAIResponsesProtocol{}).Lower(test.req); err == nil {
				t.Fatal("Lower() error = nil")
			}
		})
	}
}
