// review.go — session listing and compact transcript extraction for learning review.
// Scans terminal and Telegram session folders under app_home and emits compact Markdown
// transcripts that can be reviewed by ask_a_friend. Layer: learning support.
// Dependencies: internal/platform, internal/session, internal/tools.
package learning

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"blazeai/internal/platform"
	"blazeai/internal/session"
	"blazeai/internal/tools"
)

const (
	maxListLimit              = 30
	maxTranscriptContentChars = 1200
	maxToolSummaryChars       = 100
)

// SessionInfo identifies one persisted session available for learning review.
//
// WHAT:  Captures location and metadata for one reviewable session.
// WHY:   The learning workflow needs stable paths, transport labels, and learning.md status.
// PARAMS: SessionPath — absolute path to session.json; SessionDir — parent folder;
//
//	LearningPath — target learning.md path; Transport — terminal or telegram; UpdatedAt — session mtime.
type SessionInfo struct {
	SessionPath   string    `json:"session_path"`
	SessionDir    string    `json:"session_dir"`
	LearningPath  string    `json:"learning_path"`
	Transport     string    `json:"transport"`
	UpdatedAt     time.Time `json:"updated_at"`
	HasLearningMD bool      `json:"has_learning_md"`
}

// DiscoverRecentSessions returns the newest sessions across terminal and Telegram storage.
//
// WHAT:  Scans app_home for recent session.json files.
// WHY:   The learning workflow must review the newest real sessions from both transports.
// PARAMS: limit — max sessions to return, capped at 30; includeTerminal/includeTelegram — source toggles.
// RETURNS: []SessionInfo — newest-first session list; error if scanning fails.
func DiscoverRecentSessions(limit int, includeTerminal, includeTelegram bool) ([]SessionInfo, error) {
	if limit <= 0 {
		limit = maxListLimit
	}
	if limit > maxListLimit {
		return nil, fmt.Errorf("limit exceeds %d", maxListLimit)
	}
	home, err := platform.AppHome()
	if err != nil {
		return nil, err
	}
	infos := make([]SessionInfo, 0)
	if includeTerminal {
		terminal, err := scanTerminalSessions(home)
		if err != nil {
			return nil, err
		}
		infos = append(infos, terminal...)
	}
	if includeTelegram {
		telegram, err := scanTelegramSessions(home)
		if err != nil {
			return nil, err
		}
		infos = append(infos, telegram...)
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].UpdatedAt.After(infos[j].UpdatedAt)
	})
	if len(infos) > limit {
		infos = infos[:limit]
	}
	return infos, nil
}

// FindSession looks up one allowed session.json path under app_home.
//
// WHAT:  Validates and loads metadata for one session file path.
// WHY:   session_review_extract must reject arbitrary files outside approved session roots.
// PARAMS: sessionPath — absolute session.json path to validate.
// RETURNS: SessionInfo — resolved session metadata; error if the path is invalid or missing.
func FindSession(sessionPath string) (SessionInfo, error) {
	home, err := platform.AppHome()
	if err != nil {
		return SessionInfo{}, err
	}
	cleanPath := filepath.Clean(strings.TrimSpace(sessionPath))
	if cleanPath == "" {
		return SessionInfo{}, fmt.Errorf("session_path is required")
	}
	if !filepath.IsAbs(cleanPath) {
		return SessionInfo{}, fmt.Errorf("session_path must be absolute: %s", sessionPath)
	}
	allowedProjects := filepath.Join(home, "projects") + string(os.PathSeparator)
	allowedTelegram := filepath.Join(home, "telegram") + string(os.PathSeparator)
	if !strings.HasPrefix(cleanPath, allowedProjects) && !strings.HasPrefix(cleanPath, allowedTelegram) {
		return SessionInfo{}, fmt.Errorf("session_path is outside app_home session roots: %s", sessionPath)
	}
	if filepath.Base(cleanPath) != "session.json" {
		return SessionInfo{}, fmt.Errorf("session_path must point to session.json: %s", sessionPath)
	}
	stat, err := os.Stat(cleanPath)
	if err != nil {
		return SessionInfo{}, fmt.Errorf("cannot stat session file %s: %w", cleanPath, err)
	}
	transport := "terminal"
	if strings.HasPrefix(cleanPath, allowedTelegram) {
		transport = "telegram"
	}
	sessionDir := filepath.Dir(cleanPath)
	learningPath := filepath.Join(sessionDir, "learning.md")
	_, learningErr := os.Stat(learningPath)
	return SessionInfo{
		SessionPath:   cleanPath,
		SessionDir:    sessionDir,
		LearningPath:  learningPath,
		Transport:     transport,
		UpdatedAt:     stat.ModTime(),
		HasLearningMD: learningErr == nil,
	}, nil
}

// ExtractCompactTranscript returns a structured Markdown transcript for one session.json.
//
// WHAT:  Loads one session file and emits a compact review-oriented transcript.
// WHY:   ask_a_friend needs a reduced, deterministic context instead of raw session JSON.
// PARAMS: sessionPath — absolute path to session.json.
// RETURNS: string — compact Markdown transcript; error if loading fails.
func ExtractCompactTranscript(sessionPath string) (string, error) {
	info, err := FindSession(sessionPath)
	if err != nil {
		return "", err
	}
	sess, err := session.Load(info.SessionDir)
	if err != nil {
		return "", err
	}
	return buildTranscript(info, sess.Messages), nil
}

// scanTerminalSessions walks app_home/projects/*/sessions/*/session.json.
func scanTerminalSessions(home string) ([]SessionInfo, error) {
	root := filepath.Join(home, "projects")
	projects, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("cannot read project sessions root %s: %w", root, err)
	}
	infos := make([]SessionInfo, 0)
	for _, project := range projects {
		if !project.IsDir() {
			continue
		}
		sessionsRoot := filepath.Join(root, project.Name(), "sessions")
		sessions, err := os.ReadDir(sessionsRoot)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("cannot read project session dir %s: %w", sessionsRoot, err)
		}
		for _, entry := range sessions {
			if !entry.IsDir() {
				continue
			}
			info, err := FindSession(filepath.Join(sessionsRoot, entry.Name(), "session.json"))
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, err
			}
			infos = append(infos, info)
		}
	}
	return infos, nil
}

// scanTelegramSessions walks app_home/telegram/*/session/session.json.
func scanTelegramSessions(home string) ([]SessionInfo, error) {
	root := filepath.Join(home, "telegram")
	instances, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("cannot read telegram root %s: %w", root, err)
	}
	infos := make([]SessionInfo, 0)
	for _, entry := range instances {
		if !entry.IsDir() {
			continue
		}
		if entry.Name() == "." || entry.Name() == ".." {
			continue
		}
		sessionPath := filepath.Join(root, entry.Name(), "session", "session.json")
		info, err := FindSession(sessionPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			if strings.Contains(err.Error(), "cannot stat session file") {
				continue
			}
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// buildTranscript renders one compact Markdown transcript from persisted messages.
func buildTranscript(info SessionInfo, messages []session.Message) string {
	var sb strings.Builder
	sb.WriteString("# Session Review Extract\n\n")
	sb.WriteString("## Session\n")
	sb.WriteString("- path: " + info.SessionPath + "\n")
	sb.WriteString("- transport: " + info.Transport + "\n")
	sb.WriteString("- updated_at: " + info.UpdatedAt.Format(time.RFC3339) + "\n")
	if info.HasLearningMD {
		sb.WriteString("- learning_md: present\n")
	} else {
		sb.WriteString("- learning_md: missing\n")
	}
	sb.WriteString("\n## Transcript\n\n")
	for _, msg := range messages {
		role := strings.TrimSpace(msg.Role)
		switch role {
		case "system", "user", "assistant":
			content := summarizeMessageContent(msg.Content, maxTranscriptContentChars)
			if content != "" {
				sb.WriteString("[" + strings.ToUpper(role) + "]\n")
				sb.WriteString(content + "\n\n")
			}
			for _, call := range parseToolCalls(msg.ToolCalls) {
				sb.WriteString("[TOOL CALL]\n")
				sb.WriteString("name: " + call.Name + "\n")
				if call.Purpose != "" {
					sb.WriteString("purpose: " + call.Purpose + "\n")
				}
				if call.Payload != "" {
					sb.WriteString("payload: " + call.Payload + "\n")
				}
				sb.WriteString("\n")
			}
		case "tool":
			status, summary := summarizeToolResult(msg.Name, msg.Content)
			sb.WriteString("[TOOL RESULT]\n")
			sb.WriteString("name: " + strings.TrimSpace(msg.Name) + "\n")
			sb.WriteString("status: " + status + "\n")
			sb.WriteString("summary: " + summary + "\n\n")
		}
	}
	return strings.TrimSpace(sb.String())
}

type toolCallSummary struct {
	Name    string
	Purpose string
	Payload string
}

// parseToolCalls converts generic persisted tool_calls data into compact summaries.
func parseToolCalls(raw interface{}) []toolCallSummary {
	if raw == nil {
		return nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var calls []tools.OpenAIToolCall
	if err := json.Unmarshal(data, &calls); err != nil {
		return nil
	}
	result := make([]toolCallSummary, 0, len(calls))
	for _, call := range calls {
		payload, purpose := summarizeToolArguments(call.Function.Name, call.Function.Arguments)
		result = append(result, toolCallSummary{
			Name:    strings.TrimSpace(call.Function.Name),
			Purpose: purpose,
			Payload: payload,
		})
	}
	return result
}

// summarizeToolArguments extracts compact payload and purpose hints from tool arguments JSON.
func summarizeToolArguments(name, raw string) (string, string) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return truncateText(strings.TrimSpace(raw), maxToolSummaryChars), ""
	}
	purpose, _ := args["purpose"].(string)
	purpose = truncateText(strings.TrimSpace(purpose), maxToolSummaryChars)
	for _, key := range []string{"command", "file_path", "name", "arguments", "session_path", "question"} {
		if value := stringifyJSONValue(args[key]); value != "" {
			return truncateText(value, maxToolSummaryChars), purpose
		}
	}
	compact := compactJSON(args)
	return truncateText(compact, maxToolSummaryChars), purpose
}

// summarizeToolResult classifies one tool result and keeps only the useful short outcome.
func summarizeToolResult(name string, raw interface{}) (string, string) {
	text := stringifyJSONValue(raw)
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "ok", "empty result"
	}
	if strings.HasPrefix(trimmed, "timeout ") {
		return "timeout", truncateText(firstLine(trimmed), maxToolSummaryChars)
	}
	if strings.HasPrefix(trimmed, "error:") || strings.HasPrefix(trimmed, "aborted") {
		return "error", truncateText(firstLine(trimmed), maxToolSummaryChars)
	}
	if strings.TrimSpace(name) == "shell" || strings.Contains(trimmed, "exit_code:") {
		return summarizeShellResult(trimmed)
	}
	return "ok", truncateText(firstLine(trimmed), maxToolSummaryChars)
}

// summarizeShellResult classifies shell-style output using exit_code/stdout/stderr blocks.
func summarizeShellResult(text string) (string, string) {
	exitCode := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "exit_code:") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "exit_code:"))
			parsed, err := strconv.Atoi(value)
			if err == nil {
				exitCode = parsed
			}
			break
		}
	}
	stdout := extractSectionValue(text, "stdout:\n")
	stderr := extractSectionValue(text, "stderr:\n")
	if exitCode != 0 {
		summary := stderr
		if summary == "" {
			summary = stdout
		}
		if summary == "" {
			summary = fmt.Sprintf("exit_code: %d", exitCode)
		}
		return "error", truncateText(firstLine(summary), maxToolSummaryChars)
	}
	summary := stdout
	if summary == "" {
		summary = stderr
	}
	if summary == "" {
		summary = "completed"
	}
	return "ok", truncateText(firstLine(summary), maxToolSummaryChars)
}

// extractSectionValue returns the first line of a stdout/stderr block.
func extractSectionValue(text, marker string) string {
	idx := strings.Index(text, marker)
	if idx < 0 {
		return ""
	}
	section := text[idx+len(marker):]
	if next := strings.Index(section, "\nstdout:\n"); next >= 0 {
		section = section[:next]
	}
	if next := strings.Index(section, "\nstderr:\n"); next >= 0 {
		section = section[:next]
	}
	return strings.TrimSpace(section)
}

// summarizeMessageContent compacts generic message content to Markdown-safe text.
func summarizeMessageContent(content interface{}, maxChars int) string {
	text := stringifyJSONValue(content)
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return truncateText(text, maxChars)
}

// stringifyJSONValue converts arbitrary decoded JSON content to readable text.
func stringifyJSONValue(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return compactJSON(v)
	}
}

// compactJSON renders a value into one-line JSON when plain string handling is unavailable.
func compactJSON(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(bytes.TrimSpace(data))
}

// truncateText keeps text compact and marks truncation explicitly.
func truncateText(text string, maxChars int) string {
	if maxChars <= 0 || len(text) <= maxChars {
		return text
	}
	return text[:maxChars-14] + "... [truncated]"
}

// firstLine returns the first non-empty line from a text block.
func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
