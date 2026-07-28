package provider

import (
	"encoding/json"
	"strings"
	"testing"

	"blazeai/internal/config"
	"blazeai/internal/session"
	"blazeai/internal/tools"
)

func TestOpenAIChatProtocolLowerEnabledProfile(t *testing.T) {
	message := session.Message{
		Role:               "assistant",
		Content:            "answer",
		Reasoning:          "thinking",
		ReasoningPresent:   true,
		ReasoningEncrypted: "encrypted",
	}
	req := Request{
		Model: ModelReference{
			ID:   "openai/model",
			Name: "model",
			Definition: config.ModelDefinition{
				Protocol:     config.ProtocolOpenAIChat,
				Capabilities: config.ModelCapabilities{Tools: true},
				OpenAIChat: &config.OpenAIChatVariant{
					IncludeStreamUsage:      true,
					IncludeReasoningContent: true,
				},
			},
		},
		Messages: []session.Message{message},
		Tools: []tools.OpenAITool{{
			Type: "function",
			Function: tools.FunctionDef{
				Name:        "lookup",
				Description: "Look up a value",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		}},
		Stream: true,
	}

	native, err := (openAIChatProtocol{}).Lower(req)
	if err != nil {
		t.Fatalf("Lower() error = %v", err)
	}
	data, err := json.Marshal(native)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	jsonText := string(data)
	for _, expected := range []string{
		`"model":"model"`,
		`"stream":true`,
		`"tools":[`,
		`"stream_options":{"include_usage":true}`,
		`"reasoning_content":"thinking"`,
		`"reasoning_encrypted_content":"encrypted"`,
	} {
		if !strings.Contains(jsonText, expected) {
			t.Errorf("lowered JSON missing %s: %s", expected, jsonText)
		}
	}
}

func TestOpenAIChatProtocolLowerDisabledProfile(t *testing.T) {
	message := session.Message{
		Role:               "assistant",
		Content:            "answer",
		Reasoning:          "thinking",
		ReasoningPresent:   true,
		ReasoningEncrypted: "encrypted",
	}
	req := Request{
		Model: ModelReference{
			Name: "glm-5.2",
			Definition: config.ModelDefinition{
				Protocol:     config.ProtocolOpenAIChat,
				Capabilities: config.ModelCapabilities{Tools: true},
				OpenAIChat:   &config.OpenAIChatVariant{},
			},
		},
		Messages: []session.Message{message},
		Tools: []tools.OpenAITool{{
			Type: "function",
			Function: tools.FunctionDef{
				Name: "lookup",
			},
		}},
		Stream: true,
	}

	native, err := (openAIChatProtocol{}).Lower(req)
	if err != nil {
		t.Fatalf("Lower() error = %v", err)
	}
	data, err := json.Marshal(native)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	jsonText := string(data)
	for _, expected := range []string{`"model":"glm-5.2"`, `"stream":true`, `"tools":[`, `"content":"answer"`} {
		if !strings.Contains(jsonText, expected) {
			t.Errorf("lowered JSON missing %s: %s", expected, jsonText)
		}
	}
	for _, omitted := range []string{"stream_options", "reasoning_content", "reasoning_encrypted_content"} {
		if strings.Contains(jsonText, omitted) {
			t.Errorf("lowered JSON contains %s: %s", omitted, jsonText)
		}
	}
	if req.Messages[0] != message {
		t.Fatalf("Lower() mutated input message: %#v", req.Messages[0])
	}
}

func TestOpenAIChatProtocolLowerErrors(t *testing.T) {
	base := Request{
		Model: ModelReference{
			Name: "model",
			Definition: config.ModelDefinition{
				Protocol:     config.ProtocolOpenAIChat,
				Capabilities: config.ModelCapabilities{Tools: false},
				OpenAIChat:   &config.OpenAIChatVariant{},
			},
		},
		Tools: []tools.OpenAITool{{Type: "function"}},
	}

	tests := []struct {
		name string
		req  Request
	}{
		{name: "tools disabled", req: base},
		{name: "empty model name", req: func() Request {
			req := base
			req.Tools = nil
			req.Model.Name = ""
			return req
		}()},
		{name: "non-openai-chat protocol", req: func() Request {
			req := base
			req.Tools = nil
			req.Model.Definition.Protocol = config.ProtocolOpenAIResponses
			return req
		}()},
		{name: "nil profile", req: func() Request {
			req := base
			req.Tools = nil
			req.Model.Definition.OpenAIChat = nil
			return req
		}()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := (openAIChatProtocol{}).Lower(test.req); err == nil {
				t.Fatal("Lower() error = nil")
			}
		})
	}
}
