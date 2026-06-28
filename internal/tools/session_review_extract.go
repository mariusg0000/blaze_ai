// session_review_extract.go — session_review_extract tool implementation.
// Exposes narrow mechanical session listing and transcript extraction for the
// session-learning-review skill. Layer: tool execution. Dependencies: internal/learning.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SessionReviewSession holds one list entry returned by the session extractor.
//
// WHAT:  Mirrors the session metadata exposed to the tool and the model.
// PARAMS: SessionPath — absolute session.json path; SessionDir — parent folder;
//
//	LearningPath — target learning.md path; Transport — terminal or telegram;
//	UpdatedAt — RFC3339 timestamp; HasLearningMD — whether learning.md already exists.
type SessionReviewSession struct {
	SessionPath   string `json:"session_path"`
	SessionDir    string `json:"session_dir"`
	LearningPath  string `json:"learning_path"`
	Transport     string `json:"transport"`
	UpdatedAt     string `json:"updated_at"`
	HasLearningMD bool   `json:"has_learning_md"`
}

// SessionReviewExtractArgs are the arguments for session_review_extract.
//
// WHAT:  Parsed arguments for listing recent sessions or extracting one transcript.
// PARAMS: Action — list or extract; Limit — max sessions for list; IncludeTerminal/includeTelegram — source toggles;
//
//	SessionPath — required for extract; Purpose — concise UI summary.
type SessionReviewExtractArgs struct {
	Action          string `json:"action"`
	Limit           *int   `json:"limit,omitempty"`
	IncludeTerminal *bool  `json:"include_terminal,omitempty"`
	IncludeTelegram *bool  `json:"include_telegram,omitempty"`
	SessionPath     string `json:"session_path,omitempty"`
	Purpose         string `json:"purpose"`
}

// SessionReviewExtractTool lists reviewable sessions and extracts compact transcripts.
type SessionReviewExtractTool struct {
	listFunc    func(limit int, includeTerminal, includeTelegram bool) ([]SessionReviewSession, error)
	extractFunc func(sessionPath string) (string, error)
}

// NewSessionReviewExtractTool creates a session_review_extract tool.
func NewSessionReviewExtractTool(listFunc func(limit int, includeTerminal, includeTelegram bool) ([]SessionReviewSession, error), extractFunc func(sessionPath string) (string, error)) *SessionReviewExtractTool {
	return &SessionReviewExtractTool{listFunc: listFunc, extractFunc: extractFunc}
}

// Name returns the tool's unique identifier.
func (t *SessionReviewExtractTool) Name() string {
	return "session_review_extract"
}

// FormatArgs returns a compact UI label for session review helper actions.
func (t *SessionReviewExtractTool) FormatArgs(args json.RawMessage) string {
	parsed, err := ParseToolCallArgs[SessionReviewExtractArgs](args)
	if err != nil {
		return "Reviewing sessions"
	}
	purpose := strings.TrimSpace(parsed.Purpose)
	if purpose != "" {
		return truncateDisplay("Review sessions: "+purpose, 80)
	}
	if strings.TrimSpace(parsed.Action) == "extract" && strings.TrimSpace(parsed.SessionPath) != "" {
		return truncateDisplay("Extracting session: "+parsed.SessionPath, 80)
	}
	return "Reviewing sessions"
}

// Description returns the human-readable description for the LLM.
func (t *SessionReviewExtractTool) Description() string {
	return "action=list|extract → list newest sessions or extract one compact transcript for learning review"
}

// Parameters returns the JSON schema for the tool's parameters.
func (t *SessionReviewExtractTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"purpose": {
				"type": "string",
				"description": "purpose = concise review helper summary"
			},
			"action": {
				"type": "string",
				"description": "action = list | extract"
			},
			"limit": {
				"type": "integer",
				"description": "limit = max sessions for list; optional = true; max = 30"
			},
			"include_terminal": {
				"type": "boolean",
				"description": "include_terminal = include project sessions; optional = true; default = true"
			},
			"include_telegram": {
				"type": "boolean",
				"description": "include_telegram = include telegram sessions; optional = true; default = true"
			},
			"session_path": {
				"type": "string",
				"description": "session_path = absolute path to app_home ... /session.json; required for extract"
			}
		},
		"required": ["purpose", "action"]
	}`)
}

// Execute runs the requested list or extract action.
func (t *SessionReviewExtractTool) Execute(ctx context.Context, args json.RawMessage) string {
	if ctx != nil && ctx.Err() != nil {
		return "aborted before execution by user"
	}
	parsed, err := ParseToolCallArgs[SessionReviewExtractArgs](args)
	if err != nil {
		return fmt.Sprintf("error: invalid arguments: %v", err)
	}
	if strings.TrimSpace(parsed.Purpose) == "" {
		return "error: purpose is required"
	}
	action := strings.TrimSpace(parsed.Action)
	switch action {
	case "list":
		if t.listFunc == nil {
			return "error: session review list helper is not configured"
		}
		limit := 30
		if parsed.Limit != nil {
			limit = *parsed.Limit
		}
		includeTerminal := true
		if parsed.IncludeTerminal != nil {
			includeTerminal = *parsed.IncludeTerminal
		}
		includeTelegram := true
		if parsed.IncludeTelegram != nil {
			includeTelegram = *parsed.IncludeTelegram
		}
		infos, err := t.listFunc(limit, includeTerminal, includeTelegram)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		data, err := json.MarshalIndent(struct {
			Sessions []SessionReviewSession `json:"sessions"`
		}{Sessions: infos}, "", "  ")
		if err != nil {
			return fmt.Sprintf("error: cannot marshal session list: %v", err)
		}
		return string(data)
	case "extract":
		if t.extractFunc == nil {
			return "error: session review extract helper is not configured"
		}
		transcript, err := t.extractFunc(parsed.SessionPath)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return transcript
	default:
		return "error: action must be list or extract"
	}
}
