// taskswitch.go — semantic task-switch detection via summarization LLM.
// Builds a compact transcript (reasoning stripped, tool calls/results truncated to 150 chars),
// asks the summarization model to detect task changes, and returns the switch point.
// Layer: context management. Dependencies: internal/session, internal/provider.
package compaction

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"blazeai/internal/session"
)

// DetectResult holds the outcome of a task-switch detection call.
//
// WHAT:  Parsed result from the summarization LLM.
// PARAMS: Changed — whether a task switch was detected;
//
//	Index — 0-based USER message index (not session array index) where the new task starts;
//	Summary — concise summary of messages before the switch.
type DetectResult struct {
	Changed bool
	Index   int
	Summary string
}

// TaskSwitchFile stores a completed async task-switch result.
//
// WHAT:  Persists the detector decision for later consumption by the main runtime.
// WHY:   The detector must not modify session history directly.
type TaskSwitchFile struct {
	UserIndex int    `json:"user_index"`
	Summary   string `json:"summary"`
}

// truncatedLen is the max character length for tool calls and tool results in the detection transcript.
const truncatedLen = 150

const taskSwitchTimeout = 15 * time.Second

const (
	taskSwitchFile       = "taskswitch.json"
	taskSwitchTempFile   = "taskswitch.json.tmp"
	taskSwitchPromptFile = "taskswitch_prompt.txt"
	taskSwitchReplyFile  = "taskswitch_response.txt"
)

// DetectTaskSwitch sends a compact transcript to the summarization LLM and parses the response.
//
// WHAT:  Calls the summarization model to detect if the user changed task.
// WHY:   The runtime starts this in parallel with the main LLM call and applies the result
//
//	after the turn completes.
//
// HOW:   Builds a compact transcript from session messages, sends to summarization provider,
//
//	parses response as either "null" or JSON {"index": N, "summary": "..."}.
//	Writes debug files (transcript, response, result) to the session folder.
//
// PARAMS: sess — the current session snapshot (snapshot taken before the main LLM call);
//
//	existingSummaries — previously saved summary text from disk.
//
// RETURNS: detectResult — parsed detection outcome; error if the LLM call fails.
func (m *Manager) DetectTaskSwitch(ctx context.Context, sess *session.Session, existingSummaries string) (DetectResult, error) {
	if m.SummarizationProvider == nil {
		return DetectResult{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	transcript := buildDetectionTranscript(sess, existingSummaries)
	if strings.TrimSpace(transcript) == "" {
		return DetectResult{}, nil
	}

	// Debug: write the full transcript + system prompt sent to the summarization LLM.
	sysPrompt := detectionSystemPrompt()
	fullPrompt := fmt.Sprintf("=== SYSTEM PROMPT ===\n%s\n\n=== TRANSCRIPT ===\n%s", sysPrompt, transcript)
	_ = os.WriteFile(filepath.Join(sess.Folder, "taskswitch_prompt.txt"), []byte(fullPrompt), 0644)

	resp, err := m.SummarizationProvider.Stream(
		ctx,
		[]session.Message{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: transcript},
		},
		nil, nil, nil,
	)
	if err != nil {
		return DetectResult{}, fmt.Errorf("task-switch detection call failed: %w", err)
	}

	// Debug: write the raw LLM response.
	_ = os.WriteFile(filepath.Join(sess.Folder, "taskswitch_response.txt"), []byte(resp.Content), 0644)

	result := parseDetectionResponse(resp.Content)

	return result, nil
}

// HasTaskSwitchState reports whether a task-switch protocol file exists.
func (m *Manager) HasTaskSwitchState(sessionFolder string) (bool, error) {
	_, err := os.Stat(filepath.Join(sessionFolder, taskSwitchFile))
	if err == nil {
		return true, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("cannot stat %s: %w", taskSwitchFile, err)
	}
	return false, nil
}

// RemoveTaskSwitchState deletes task-switch protocol and debug files for the current session.
func (m *Manager) RemoveTaskSwitchState(sessionFolder string) error {
	for _, name := range []string{
		taskSwitchFile,
		taskSwitchTempFile,
		taskSwitchPromptFile,
		taskSwitchReplyFile,
		"taskswitch.pending.json",
		"taskswitch.result.json",
	} {
		if err := os.Remove(filepath.Join(sessionFolder, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("cannot remove %s: %w", name, err)
		}
	}
	return nil
}

// StartTaskSwitchJob creates the task-switch marker file and starts the async detector.
func (m *Manager) StartTaskSwitchJob(parentCtx context.Context, sess *session.Session, snapshot []session.Message, existingSummaries string) error {
	if m == nil || m.SummarizationProvider == nil || sess == nil {
		return nil
	}
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	markerPath := filepath.Join(sess.Folder, taskSwitchFile)
	marker, err := os.OpenFile(markerPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return fmt.Errorf("cannot create %s: %w", taskSwitchFile, err)
	}
	if err := marker.Close(); err != nil {
		return fmt.Errorf("cannot close %s: %w", taskSwitchFile, err)
	}
	snapshotCopy := make([]session.Message, len(snapshot))
	copy(snapshotCopy, snapshot)
	go m.runTaskSwitchJob(parentCtx, sess.Folder, snapshotCopy, existingSummaries)
	return nil
}

// ConsumeTaskSwitchResult applies any finished task-switch result and clears task-switch state.
//
// WHAT:  Reads a finished detector result from taskswitch.json, validates it, and applies it.
// WHY:   Only the main runtime is allowed to mutate session history.
// RETURNS: result — consumed task-switch result or nil when the worker is still pending or absent.
func (m *Manager) ConsumeTaskSwitchResult(sess *session.Session) (*TaskSwitchFile, error) {
	if m == nil || sess == nil {
		return nil, nil
	}
	resultPath := filepath.Join(sess.Folder, taskSwitchFile)
	data, err := os.ReadFile(resultPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read %s: %w", taskSwitchFile, err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil, nil
	}
	result, err := readTaskSwitchFile(data)
	if err != nil {
		if removeErr := m.RemoveTaskSwitchProtocolFile(sess.Folder); removeErr != nil {
			return nil, fmt.Errorf("invalid %s and cannot remove it: %w", taskSwitchFile, removeErr)
		}
		return nil, fmt.Errorf("invalid %s removed: %w", taskSwitchFile, err)
	}
	if result.UserIndex <= 0 {
		if removeErr := m.RemoveTaskSwitchProtocolFile(sess.Folder); removeErr != nil {
			return nil, fmt.Errorf("invalid %s and cannot remove it: %w", taskSwitchFile, removeErr)
		}
		return nil, fmt.Errorf("invalid %s removed: user_index must be greater than 0", taskSwitchFile)
	}
	if strings.TrimSpace(result.Summary) == "" {
		if removeErr := m.RemoveTaskSwitchProtocolFile(sess.Folder); removeErr != nil {
			return nil, fmt.Errorf("invalid %s and cannot remove it: %w", taskSwitchFile, removeErr)
		}
		return nil, fmt.Errorf("invalid %s removed: summary is empty", taskSwitchFile)
	}
	if err := m.CompactByTaskSwitch(sess, result.UserIndex, result.Summary); err != nil {
		return nil, err
	}
	if err := m.RemoveTaskSwitchProtocolFile(sess.Folder); err != nil {
		return nil, err
	}
	return &result, nil
}

// RemoveTaskSwitchProtocolFile removes only the protocol file(s), not debug artifacts.
func (m *Manager) RemoveTaskSwitchProtocolFile(sessionFolder string) error {
	for _, name := range []string{taskSwitchFile, taskSwitchTempFile} {
		if err := os.Remove(filepath.Join(sessionFolder, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("cannot remove %s: %w", name, err)
		}
	}
	return nil
}

func (m *Manager) runTaskSwitchJob(parentCtx context.Context, sessionFolder string, snapshot []session.Message, existingSummaries string) {
	ctx, cancel := context.WithTimeout(parentCtx, taskSwitchTimeout)
	defer cancel()
	snapSess := &session.Session{Messages: snapshot, Folder: sessionFolder}
	detect, err := m.DetectTaskSwitch(ctx, snapSess, existingSummaries)
	if err != nil {
		_ = m.RemoveTaskSwitchProtocolFile(sessionFolder)
		return
	}
	if ctx.Err() == context.Canceled {
		_ = m.RemoveTaskSwitchProtocolFile(sessionFolder)
		return
	}
	if !detect.Changed {
		_ = m.RemoveTaskSwitchProtocolFile(sessionFolder)
		return
	}
	if _, err := os.Stat(filepath.Join(sessionFolder, taskSwitchFile)); err != nil {
		return
	}
	result := TaskSwitchFile{UserIndex: detect.Index, Summary: detect.Summary}
	_ = writeTaskSwitchJSONAtomic(filepath.Join(sessionFolder, taskSwitchFile), result)
}

func writeTaskSwitchJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal %s: %w", filepath.Base(path), err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("cannot write %s: %w", filepath.Base(tmpPath), err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("cannot rename %s to %s: %w", filepath.Base(tmpPath), filepath.Base(path), err)
	}
	return nil
}

func readTaskSwitchFile(data []byte) (TaskSwitchFile, error) {
	var result TaskSwitchFile
	if err := json.Unmarshal(data, &result); err != nil {
		return result, err
	}
	return result, nil
}

// buildDetectionTranscript creates a compact text transcript of session messages.
// Reasoning is excluded. Tool calls and tool results are truncated to truncatedLen characters.
// User and assistant messages are included in full.
//
// WHAT:  Converts session messages into a compact transcript for the detection LLM.
// WHY:   The detection prompt needs a token-efficient representation of the full conversation.
// HOW:   Iterates messages, formats by role, truncates tool content, numbers user messages.
// PARAMS: sess — the session snapshot; existingSummaries — prior summary text for context.
// RETURNS: string — compact transcript.
func buildDetectionTranscript(sess *session.Session, existingSummaries string) string {
	var sb strings.Builder

	if strings.TrimSpace(existingSummaries) != "" {
		sb.WriteString("[summary]\n")
		sb.WriteString(existingSummaries)
		sb.WriteString("\n\n")
	}

	userIndex := 0
	for _, msg := range sess.Messages {
		switch msg.Role {
		case "user":
			content, _ := msg.Content.(string)
			sb.WriteString(fmt.Sprintf("[user %d] %s\n", userIndex, content))
			userIndex++
		case "assistant":
			content, _ := msg.Content.(string)
			if content != "" {
				sb.WriteString(fmt.Sprintf("[assistant] %s\n", content))
			}
			if msg.ToolCalls != nil {
				data := serializeToolCalls(msg.ToolCalls)
				trunc := truncate(data, truncatedLen)
				sb.WriteString(fmt.Sprintf("[tool call] %s\n", trunc))
			}
		case "tool":
			content, _ := msg.Content.(string)
			trunc := truncate(content, truncatedLen)
			sb.WriteString(fmt.Sprintf("[tool result: %s] %s\n", msg.Name, trunc))
		}
	}

	return sb.String()
}

// detectionSystemPrompt returns the strict system prompt for the task-switch detector.
//
// WHAT:  Builds the summarization LLM system prompt instructing it to detect task changes.
// WHY:   The LLM must return a predictable, parseable format: null or JSON.
// RETURNS: string — the system prompt.
func detectionSystemPrompt() string {
	return `You are a task-switch detector. Analyze the conversation transcript below.

Each user message is labeled as [user 0], [user 1], [user 2], etc. These labels uniquely
identify each user message. Use them — do NOT count messages yourself.

Determine if the user has clearly switched to a substantially different task, topic, or goal.

Consider it a TASK SWITCH when:
- The user explicitly states they want to work on something new or different
- The topic, domain, or goal shifts significantly from the prior conversation
- The user dismisses, concludes, or abandons the previous topic and starts fresh

Do NOT consider it a task switch when:
- The user asks a follow-up question on the same topic
- The user adds a related or supplementary request
- The user clarifies, corrects, or iterates on the current task
- The user continues working on the same project/domain with small variations

If NO task switch is detected, respond with exactly:
null

If a task switch IS detected, respond with exactly a JSON object:
{"index": "user N", "summary": "<concise summary of all messages BEFORE this user message>"}

Replace N with the number from the [user N] label where the new task starts — for example
"user 3" if the switch occurs at the message labeled [user 3]. Do not use the session
message position or any other numbering.

The summary must cover all user and assistant messages up to but NOT including the
switch message. Focus on decisions, facts, actions taken, and their outcomes.
Omit pleasantries.

Respond ONLY with null or the JSON object. No other text.`
}

// parseDetectionResponse extracts the detection result from the LLM response text.
//
// WHAT:  Parses the summarization model's response into a DetectResult.
// WHY:   The LLM returns either "null" or a JSON object; the index field may be a number
//
//	or a label string like "user 5". Both must be handled robustly.
//
// HOW:   Trim whitespace and fences; if "null" → no change; else JSON unmarshal with
//
//	flexible index field (int or string), extract numeric value, validate.
//
// PARAMS: raw — the raw response text from the LLM.
// RETURNS: DetectResult — parsed outcome (Changed=false on null or parse failure).
func parseDetectionResponse(raw string) DetectResult {
	raw = strings.TrimSpace(raw)

	if raw == "" || raw == "null" {
		return DetectResult{}
	}

	// Strip any markdown code fences that the model might emit.
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	// Use a flexible struct where index can be either int or string.
	var parsed struct {
		Index   interface{} `json:"index"`
		Summary string      `json:"summary"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return DetectResult{}
	}

	idx := extractIndex(parsed.Index)
	if idx <= 0 || strings.TrimSpace(parsed.Summary) == "" {
		return DetectResult{}
	}

	return DetectResult{
		Changed: true,
		Index:   idx,
		Summary: strings.TrimSpace(parsed.Summary),
	}
}

// extractIndex pulls a numeric user index from the LLM response field.
// Accepts: float64 (5.0), string ("5"), string ("user 5"), string ("user_05").
// Returns the integer value, or 0 on failure.
//
// WHAT:  Extracts the user-message number from flexible LLM output.
// WHY:   LLMs may return the index as a bare integer, a quoted digit, or the full label.
// PARAMS: v — the raw JSON-decoded index field (float64 or string).
// RETURNS: int — the extracted number, or 0 if unparseable.
func extractIndex(v interface{}) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case string:
		return parseDigits(val)
	default:
		return 0
	}
}

// parseDigits extracts all consecutive digits from a string and converts to int.
// Returns 0 if no digits are found or the value overflows.
//
// WHAT:  Strips non-digit characters and parses the remaining digits as an integer.
// PARAMS: s — potentially messy string like "user 5", "5", "user_05".
// RETURNS: int — the numeric value, or 0.
func parseDigits(s string) int {
	var buf strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			buf.WriteRune(r)
		}
	}
	if buf.Len() == 0 {
		return 0
	}
	// ParseInt handles leading zeros ("05" → 5).
	var n int
	for i := 0; i < buf.Len(); i++ {
		n = n*10 + int(buf.String()[i]-'0')
	}
	return n
}

// truncate shortens a string to maxLen characters, appending "..." if truncated.
//
// WHAT:  Truncates text for compact display in detection transcripts.
// PARAMS: s — the text to truncate; maxLen — maximum length including "...".
// RETURNS: string — truncated text.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
