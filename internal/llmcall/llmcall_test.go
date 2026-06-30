// llmcall_test.go — tests for one-shot role-based secondary LLM calls.
package llmcall

import (
	"context"
	"errors"
	"testing"

	"blazeai/internal/config"
	"blazeai/internal/provider"
	"blazeai/internal/session"
	"blazeai/internal/tools"
)

type fakeStreamClient struct {
	resp     *provider.Response
	err      error
	messages []session.Message
	toolDefs []tools.OpenAITool
}

func (f *fakeStreamClient) Stream(ctx context.Context, messages []session.Message, toolDefs []tools.OpenAITool, onContent func(string), onReasoning func(string)) (*provider.Response, error) {
	f.messages = messages
	f.toolDefs = toolDefs
	return f.resp, f.err
}

// TestCallerCallSuccess verifies role resolution and strict no-tools one-shot prompting.
func TestCallerCallSuccess(t *testing.T) {
	fakeClient := &fakeStreamClient{resp: &provider.Response{Content: "Concise answer"}}
	var gotModelID string
	caller := New(&config.Config{Roles: config.Roles{Advisor: "test/strong-model"}}, func(cfg *config.Config, modelID string) (StreamClient, error) {
		gotModelID = modelID
		return fakeClient, nil
	})
	result, err := caller.Call(context.Background(), Request{
		Role:         "advisor",
		Purpose:      "review plan",
		Question:     "What is the main risk?",
		Context:      "Current plan and constraints.",
		OutputFormat: "markdown findings",
	})
	if err != nil {
		t.Fatalf("Call() error: %v", err)
	}
	if result != "Concise answer" {
		t.Fatalf("Call() = %q, want %q", result, "Concise answer")
	}
	if gotModelID != "test/strong-model" {
		t.Fatalf("modelID = %q, want %q", gotModelID, "test/strong-model")
	}
	if len(fakeClient.messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(fakeClient.messages))
	}
	if fakeClient.messages[0].Role != "system" || fakeClient.messages[1].Role != "user" {
		t.Fatalf("unexpected message roles: %#v", fakeClient.messages)
	}
	if len(fakeClient.toolDefs) != 0 {
		t.Fatalf("toolDefs = %d, want 0", len(fakeClient.toolDefs))
	}
}

// TestCallerCallRejectsToolCalls verifies the secondary call stays tool-free.
func TestCallerCallRejectsToolCalls(t *testing.T) {
	caller := New(&config.Config{Roles: config.Roles{Advisor: "test/strong-model"}}, func(cfg *config.Config, modelID string) (StreamClient, error) {
		return &fakeStreamClient{resp: &provider.Response{Content: "", ToolCalls: []tools.ToolCall{{ID: "1", Name: "shell", Arguments: []byte(`{"command":"ls"}`)}}}}, nil
	})
	_, err := caller.Call(context.Background(), Request{
		Role:         "advisor",
		Purpose:      "review plan",
		Question:     "What is the main risk?",
		Context:      "Current plan and constraints.",
		OutputFormat: "markdown findings",
	})
	if err == nil {
		t.Fatal("Call() expected error for delegated tool call, got nil")
	}
}

// TestCallerCallMissingRole verifies missing role configuration fails clearly.
func TestCallerCallMissingRole(t *testing.T) {
	caller := New(&config.Config{Roles: config.Roles{}}, func(cfg *config.Config, modelID string) (StreamClient, error) {
		return &fakeStreamClient{}, nil
	})
	_, err := caller.Call(context.Background(), Request{
		Role:         "advisor",
		Purpose:      "review plan",
		Question:     "What is the main risk?",
		Context:      "Current plan and constraints.",
		OutputFormat: "markdown findings",
	})
	if err == nil {
		t.Fatal("Call() expected error for missing role config, got nil")
	}
}

// TestCallerCallFactoryError verifies provider construction errors are wrapped.
func TestCallerCallFactoryError(t *testing.T) {
	caller := New(&config.Config{Roles: config.Roles{Advisor: "test/strong-model"}}, func(cfg *config.Config, modelID string) (StreamClient, error) {
		return nil, errors.New("boom")
	})
	_, err := caller.Call(context.Background(), Request{
		Role:         "advisor",
		Purpose:      "review plan",
		Question:     "What is the main risk?",
		Context:      "Current plan and constraints.",
		OutputFormat: "markdown findings",
	})
	if err == nil {
		t.Fatal("Call() expected error for factory failure, got nil")
	}
}

// TestCallerCallVisionImageRequest verifies multimodal user content for vision requests.
func TestCallerCallVisionImageRequest(t *testing.T) {
	fakeClient := &fakeStreamClient{resp: &provider.Response{Content: "Image answer"}}
	caller := New(&config.Config{Roles: config.Roles{Vision: "test/vision-model"}}, func(cfg *config.Config, modelID string) (StreamClient, error) {
		return fakeClient, nil
	})
	result, err := caller.Call(context.Background(), Request{
		Role:         "vision",
		Purpose:      "analyze a screenshot",
		Question:     "Describe the visible text",
		Context:      "local screenshot metadata",
		OutputFormat: "concise markdown",
		ImageDataURL: "data:image/jpeg;base64,abc",
	})
	if err != nil {
		t.Fatalf("Call() error: %v", err)
	}
	if result != "Image answer" {
		t.Fatalf("Call() = %q, want %q", result, "Image answer")
	}
	parts, ok := fakeClient.messages[1].Content.([]messageContentPart)
	if !ok {
		t.Fatalf("user content type = %T, want []messageContentPart", fakeClient.messages[1].Content)
	}
	if len(parts) != 2 {
		t.Fatalf("content parts = %d, want 2", len(parts))
	}
	if parts[0].Type != "text" || parts[1].Type != "image_url" {
		t.Fatalf("unexpected part types: %#v", parts)
	}
	if parts[1].ImageURL == nil || parts[1].ImageURL.URL != "data:image/jpeg;base64,abc" {
		t.Fatalf("unexpected image URL payload: %#v", parts[1].ImageURL)
	}
}
