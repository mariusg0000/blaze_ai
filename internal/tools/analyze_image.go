// analyze_image.go — analyze_image tool implementation.
// Accepts one local image file, normalizes it for efficient multimodal vision input,
// and delegates one focused question to the configured vision role.
// Layer: tool execution. Dependencies: internal/llmcall via injected caller.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// AnalyzeImageArgs are the LLM-facing arguments for analyze_image.
//
// WHAT:  Parsed analyze_image tool inputs.
// WHY:   Vision analysis needs only one local image path and one explicit question.
// PARAMS: InputFile — local image file path; Question — exact requested analysis.
type AnalyzeImageArgs struct {
	InputFile string `json:"input_file"`
	Question  string `json:"question"`
}

// AnalyzeImageRequest is the prepared vision payload passed to the multimodal caller.
//
// WHAT:  One ready-to-send image analysis request.
// WHY:   Runtime wiring should receive both the normalized image payload and useful metadata.
// PARAMS: InputFile — original local path; Question — requested analysis; SourceMediaType —
// detected source MIME type; SourceSizeBytes — source file size; SourceWidth/SourceHeight —
// decoded source dimensions; OutputWidth/OutputHeight — resized dimensions; ImageDataURL —
// final JPEG data URL sent to the provider.
type AnalyzeImageRequest struct {
	InputFile       string
	Question        string
	SourceMediaType string
	SourceSizeBytes int64
	SourceWidth     int
	SourceHeight    int
	OutputWidth     int
	OutputHeight    int
	ImageDataURL    string
}

// AnalyzeImageTool delegates one local image question to the configured vision role.
//
// WHAT:  Validates inputs, preprocesses the image, and returns one vision-model answer.
// WHY:   Screenshots and photos need multimodal transport, not text-file delegation.
// PARAMS: caller — secondary vision helper used for the final one-shot LLM call.
type AnalyzeImageTool struct {
	caller func(ctx context.Context, req AnalyzeImageRequest) (string, error)
}

// NewAnalyzeImageTool creates an analyze_image tool.
func NewAnalyzeImageTool(caller func(ctx context.Context, req AnalyzeImageRequest) (string, error)) *AnalyzeImageTool {
	return &AnalyzeImageTool{caller: caller}
}

// Name returns the tool's unique identifier.
func (t *AnalyzeImageTool) Name() string {
	return "analyze_image"
}

// Description returns the human-readable description for the LLM.
func (t *AnalyzeImageTool) Description() string {
	return "Analyze one local image file with the configured vision model role. Use it for screenshots, photos, diagrams, maps, charts, scans, and other visual inputs. Put the exact task, required details, and desired answer shape directly in question because this tool only accepts the image path and the question."
}

// Parameters returns the JSON schema for the tool's parameters.
func (t *AnalyzeImageTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"input_file": {
				"type": "string",
				"description": "input_file = local image path to analyze; supports png, jpeg, and gif"
			},
			"question": {
				"type": "string",
				"description": "question = exact requested analysis, required details, and expected answer shape"
			}
		},
		"required": ["input_file", "question"]
	}`)
}

// FormatArgs returns a compact UI label for image analysis.
func (t *AnalyzeImageTool) FormatArgs(args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[AnalyzeImageArgs](args)
	if err != nil {
		return "Analyzing image"
	}
	name := filepath.Base(strings.TrimSpace(parsed.InputFile))
	question := strings.TrimSpace(parsed.Question)
	if name == "" {
		return "Analyzing image"
	}
	if question == "" {
		return "Analyzing image: " + name
	}
	return truncateDisplay("Analyzing image: "+name+" — "+question, 90)
}

// Execute performs the local preprocessing and delegated vision call.
func (t *AnalyzeImageTool) Execute(ctx context.Context, args json.RawMessage) string {
	if ctx != nil && ctx.Err() != nil {
		return "aborted before execution by user"
	}
	parsed, err := ParseToolCallArgs[AnalyzeImageArgs](args)
	if err != nil {
		return fmt.Sprintf("error: invalid arguments: %v", err)
	}
	if err := validateRequiredField("input_file", parsed.InputFile); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if err := validateRequiredField("question", parsed.Question); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if t.caller == nil {
		return "error: analyze_image caller is not configured"
	}
	if ctx == nil {
		ctx = context.Background()
	}
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(DefaultTimeout)*time.Second)
	defer cancel()

	prepared, err := prepareVisionImage(callCtx, strings.TrimSpace(parsed.InputFile))
	if err != nil {
		if callCtx.Err() != nil {
			return fmt.Sprintf("timeout %ds exceeded", DefaultTimeout)
		}
		return fmt.Sprintf("error: %v", err)
	}
	result, err := t.caller(callCtx, AnalyzeImageRequest{
		InputFile:       prepared.InputFile,
		Question:        strings.TrimSpace(parsed.Question),
		SourceMediaType: prepared.SourceMediaType,
		SourceSizeBytes: prepared.SourceSizeBytes,
		SourceWidth:     prepared.SourceWidth,
		SourceHeight:    prepared.SourceHeight,
		OutputWidth:     prepared.OutputWidth,
		OutputHeight:    prepared.OutputHeight,
		ImageDataURL:    prepared.DataURL,
	})
	if err != nil {
		if callCtx.Err() != nil {
			return fmt.Sprintf("timeout %ds exceeded", DefaultTimeout)
		}
		return fmt.Sprintf("error: %v", err)
	}
	return result
}
