// ask_friend.go — ask_a_friend tool implementation.
// Delegates one focused text-only subproblem to a configured secondary model role and
// returns the plain-text answer as a normal tool result. Layer: tool execution.
// Dependencies: internal/llmcall.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	maxAskFriendContextChars   = 300000
	maxAskFriendInputFileBytes = 500000
)

// AskFriendArgs are the arguments for ask_a_friend.
//
// WHAT:  Parsed ask_a_friend tool inputs.
// PARAMS: Role — configured model role; Purpose — concise objective; Question — focused ask;
//
//	Context — supporting evidence; InputFile — optional file to include verbatim; OutputFormat — exact answer shape; Timeout — optional seconds.
type AskFriendArgs struct {
	Role         string `json:"role"`
	Purpose      string `json:"purpose"`
	Question     string `json:"question"`
	Context      string `json:"context"`
	InputFile    string `json:"input_file,omitempty"`
	OutputFormat string `json:"output_format"`
	Timeout      *int   `json:"timeout,omitempty"`
}

// AskFriendTool delegates one no-tools consultation to a configured secondary role.
//
// WHAT:  Validates ask_a_friend arguments and returns one consultant answer.
// WHY:   Some text-heavy tasks need a stronger or specialized second opinion without a nested agent.
// PARAMS: caller — secondary LLM helper used for role resolution and API calls.
type AskFriendTool struct {
	caller func(ctx context.Context, args AskFriendArgs) (string, error)
}

// NewAskFriendTool creates an ask_a_friend tool.
func NewAskFriendTool(caller func(ctx context.Context, args AskFriendArgs) (string, error)) *AskFriendTool {
	return &AskFriendTool{caller: caller}
}

// Name returns the tool's unique identifier.
func (t *AskFriendTool) Name() string {
	return "ask_a_friend"
}

// FormatArgs returns a compact UI label for delegated consultation.
func (t *AskFriendTool) FormatArgs(args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[AskFriendArgs](args)
	if err != nil {
		return "Consulting secondary model"
	}
	purpose := strings.TrimSpace(parsed.Purpose)
	role := strings.TrimSpace(parsed.Role)
	if purpose == "" && role == "" {
		return "Consulting secondary model"
	}
	if purpose == "" {
		return truncateDisplay("Consulting "+role, 50)
	}
	if role == "" {
		return "Consulting: " + purpose
	}
	return "Consulting " + role + ": " + purpose
}

// Description returns the human-readable description for the LLM.
func (t *AskFriendTool) Description() string {
	return "Delegate one focused text-only question to a configured secondary model role with no tools. Use it only when an independent summarization, review, risk check, or trade-off analysis would improve the current task. Provide all required context because the secondary model cannot see the current conversation. For screenshots, photos, charts, or other images, use analyze_image instead."
}

// Parameters returns the JSON schema for the tool's parameters.
func (t *AskFriendTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"role": {
				"type": "string",
				"description": "role = configured model role; allowed = advisor | summarization | vision"
			},
			"purpose": {
				"type": "string",
				"description": "purpose = exactly 3 user-visible sentences. Sentence 1 must name the secondary model role, the consultation topic/question, and any relevant input file or context source. Sentence 2 must describe the scope or focus area of the consultation. Sentence 3 must explain what answer the consultation should produce and how that result solves or advances the task."
			},
			"question": {
				"type": "string",
				"description": "question = focused ask for the secondary model"
			},
			"context": {
				"type": "string",
				"description": "context = supporting evidence; required = true"
			},
			"input_file": {
				"type": "string",
				"description": "input_file = optional readable text file path to include in the consultation; max size = 500000 bytes; images must use analyze_image instead"
			},
			"output_format": {
				"type": "string",
				"description": "output_format = exact required answer shape"
			},
			"timeout": {
				"type": "integer",
				"description": "timeout = seconds; optional = true; default = 60"
			}
		},
		"required": ["role", "purpose", "question", "context", "output_format"]
	}`)
}

// Execute performs the delegated one-shot call and returns the answer text.
func (t *AskFriendTool) Execute(ctx context.Context, args json.RawMessage) string {
	if ctx != nil && ctx.Err() != nil {
		return "aborted before execution by user"
	}
	parsed, err := ParseToolCallArgs[AskFriendArgs](args)
	if err != nil {
		return fmt.Sprintf("error: invalid arguments: %v", err)
	}
	if err := validateAskFriendArgs(parsed); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if t.caller == nil {
		return "error: ask_a_friend caller is not configured"
	}
	timeoutSec := DefaultTimeout
	if parsed.Timeout != nil && *parsed.Timeout > 0 {
		timeoutSec = *parsed.Timeout
	}
	if ctx == nil {
		ctx = context.Background()
	}
	prepared, err := prepareAskFriendArgs(parsed)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()
	result, err := t.caller(callCtx, prepared)
	if err != nil {
		if callCtx.Err() != nil {
			return fmt.Sprintf("timeout %ds exceeded", timeoutSec)
		}
		return fmt.Sprintf("error: %v", err)
	}
	return result
}

// prepareAskFriendArgs injects optional file content into the secondary-model context.
func prepareAskFriendArgs(args AskFriendArgs) (AskFriendArgs, error) {
	prepared := args
	inputFile := strings.TrimSpace(args.InputFile)
	if inputFile == "" {
		return prepared, nil
	}
	stat, err := os.Stat(inputFile)
	if err != nil {
		return AskFriendArgs{}, fmt.Errorf("cannot stat input_file %s: %w", inputFile, err)
	}
	if !stat.Mode().IsRegular() {
		return AskFriendArgs{}, fmt.Errorf("input_file is not a regular file: %s", inputFile)
	}
	mediaType, err := detectInputMediaType(inputFile)
	if err != nil {
		return AskFriendArgs{}, err
	}
	if isImageMediaType(mediaType) {
		return AskFriendArgs{}, fmt.Errorf("input_file is an image; use analyze_image: %s", inputFile)
	}
	if stat.Size() > maxAskFriendInputFileBytes {
		return AskFriendArgs{}, fmt.Errorf("input_file exceeds %d bytes: %s", maxAskFriendInputFileBytes, inputFile)
	}
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return AskFriendArgs{}, fmt.Errorf("cannot read input_file %s: %w", inputFile, err)
	}
	prepared.InputFile = inputFile
	prepared.Context = strings.TrimSpace(prepared.Context) + "\n\n[INPUT FILE]\npath: " + inputFile + "\nsize_bytes: " + fmt.Sprintf("%d", len(data)) + "\ncontent:\n" + string(data)
	return prepared, nil
}

// validateAskFriendArgs enforces the constrained ask_a_friend contract.
func validateAskFriendArgs(args AskFriendArgs) error {
	role := strings.TrimSpace(args.Role)
	switch role {
	case "advisor", "summarization", "vision":
	default:
		return fmt.Errorf("role must be one of advisor, summarization, or vision")
	}
	if err := validateRequiredField("purpose", args.Purpose); err != nil {
		return err
	}
	if err := validateRequiredField("question", args.Question); err != nil {
		return err
	}
	if err := validateMaxField("context", args.Context, maxAskFriendContextChars); err != nil {
		return err
	}
	if err := validateRequiredField("output_format", args.OutputFormat); err != nil {
		return err
	}
	return nil
}

// validateRequiredField rejects empty string fields.
func validateRequiredField(name, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

// validateMaxField rejects empty or oversized string fields.
func validateMaxField(name, value string, maxChars int) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if len(value) > maxChars {
		return fmt.Errorf("%s exceeds %d characters", name, maxChars)
	}
	return nil
}
