// compaction.go — context compaction for long sessions.
// Triggers on provider-reported prompt_tokens reaching maxContextTokens, prunes old messages
// with tool-boundary safety, summarizes pruned segments, stores summary chunks, and strips
// reasoning parts from the LLM payload while keeping session JSON intact on disk.
// Layer: context management. Dependencies: internal/session, internal/provider, internal/config.
package compaction

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"blazeai/internal/config"
	"blazeai/internal/provider"
	"blazeai/internal/session"
)

// Manager handles context compaction for a single session.
//
// WHAT:  Orchestrates compaction: trigger check, pruning, summarization, summary storage/injection.
// WHY:   Long sessions exceed model context windows; compaction keeps them usable.
// PARAMS: Config — compaction thresholds and strip reasoning settings; Provider — LLM client for summarization.
type Manager struct {
	Config   *config.Config
	Provider *provider.Client
}

// NewManager creates a compaction Manager from config and a provider client.
//
// PARAMS: cfg — loaded config with compaction thresholds; client — provider for summarization calls.
// RETURNS: *Manager — ready to check and compact.
func NewManager(cfg *config.Config, client *provider.Client) *Manager {
	return &Manager{Config: cfg, Provider: client}
}

// ShouldCompact checks whether compaction should trigger based on provider-reported usage.
//
// WHAT:  Compares prompt_tokens from the last LLM response against the maxContextTokens threshold.
// WHY:   Compaction triggers only on provider-reported tokens, never on local estimates.
// PARAMS: usage — token usage from the last assistant response; may be nil if provider didn't report.
// RETURNS: bool — true if prompt_tokens >= maxContextTokens.
func (m *Manager) ShouldCompact(usage *provider.Usage) bool {
	if usage == nil {
		return false
	}
	return usage.PromptTokens >= m.Config.Compaction.MaxContextTokens
}

// estimateTokens estimates the token count of a message using the local estimator.
// Stripped reasoning parts count as 0 tokens.
//
// WHAT:  Local token estimate for a single message.
// WHY:   Used for cut point selection during pruning.
// PARAMS: msg — the message to estimate; willStripReasoning — whether reasoning will be stripped from this message in the payload.
// RETURNS: int — estimated token count.
func (m *Manager) estimateTokens(msg session.Message, willStripReasoning bool) int {
	coef := m.Config.Compaction.TokenCoefficient
	if coef <= 0 {
		coef = 3.5
	}

	var totalChars int

	// Content part.
	if content, ok := msg.Content.(string); ok {
		totalChars += len(content)
	}

	// Reasoning part: counts as 0 if it will be stripped.
	if !willStripReasoning {
		totalChars += len(msg.Reasoning)
	}

	// Tool calls part.
	if msg.ToolCalls != nil {
		data, _ := json.Marshal(msg.ToolCalls)
		totalChars += len(data)
	}

	// Tool call ID and name.
	totalChars += len(msg.ToolCallID) + len(msg.Name)

	return int(float64(totalChars) / coef)
}

// findCutPoint walks messages from newest to oldest, summing estimated tokens,
// and returns the index where the retained tail reaches minTokens.
// Stripped reasoning parts count as 0 tokens per spec 05.
//
// WHAT:  Finds the prune boundary that retains approximately minTokens of recent messages.
// WHY:   Pruning must retain enough recent context; tool validity is enforced by the sanitizer.
// PARAMS: messages — the full message array; minTokens — target retained tokens.
// RETURNS: int — cut point index (messages before this index are pruned).
func (m *Manager) findCutPoint(messages []session.Message, minTokens int) int {
	willStrip := m.Config.StripReasoning.Enable
	preserveLast := m.Config.StripReasoning.PreserveLast

	// Determine which messages will have reasoning stripped (older than newest N with reasoning).
	stripSet := m.buildReasoningStripSet(messages, willStrip, preserveLast)

	accumulated := 0
	cutIndex := len(messages)

	for i := len(messages) - 1; i >= 0; i-- {
		tokens := m.estimateTokens(messages[i], stripSet[i])
		if accumulated+tokens > minTokens {
			break
		}
		accumulated += tokens
		cutIndex = i
	}

	return cutIndex
}

// buildReasoningStripSet returns a map indicating which messages will have reasoning stripped.
// Messages with reasoning that are older than the newest N will be stripped.
//
// WHAT:  Determines which messages have reasoning that will be stripped in the payload.
// WHY:   The token estimator needs to know which reasoning counts as 0 tokens.
// PARAMS: messages — full message array; enable — whether stripping is enabled; preserveLast — newest N to keep.
// RETURNS: map[int]bool — true for messages whose reasoning will be stripped.
func (m *Manager) buildReasoningStripSet(messages []session.Message, enable bool, preserveLast int) map[int]bool {
	stripSet := make(map[int]bool, len(messages))
	if !enable {
		return stripSet
	}

	// Count reasoning messages from the end.
	reasoningCount := 0
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Reasoning != "" {
			reasoningCount++
		}
	}

	if reasoningCount <= preserveLast {
		return stripSet
	}

	// Mark older reasoning messages for stripping.
	kept := 0
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Reasoning == "" {
			continue
		}
		if kept < preserveLast {
			kept++
			continue
		}
		stripSet[i] = true
	}

	return stripSet
}

// isAboveHardCap checks if the provider-reported tokens exceed the hard cap.
//
// PARAMS: usage — token usage from the last response.
// RETURNS: bool — true if above hard cap.
func (m *Manager) isAboveHardCap(usage *provider.Usage) bool {
	if usage == nil {
		return false
	}
	hardCap := m.Config.Compaction.MaxContextTokens + m.Config.Compaction.MaxBackoffOffsetTokens
	return usage.PromptTokens >= hardCap
}

// hardMinTokens returns the retained token target when above the hard cap.
//
// RETURNS: int — minContextTokens + maxBackoffOffsetTokens.
func (m *Manager) hardMinTokens() int {
	return m.Config.Compaction.MinContextTokens + m.Config.Compaction.MaxBackoffOffsetTokens
}

// Compact performs the full compaction flow if triggered.
// Returns true if compaction occurred, false if it was skipped.
//
// WHAT:  Checks trigger, prunes, summarizes, stores summary, rebuilds session.
// WHY:   Called after each LLM turn to keep context within limits.
// PARAMS: sess — the current session; usage — token usage from the last response.
// RETURNS: bool — true if compaction happened; error if summarization fails fatally.
func (m *Manager) Compact(sess *session.Session, usage *provider.Usage) (bool, error) {
	if !m.ShouldCompact(usage) {
		return false, nil
	}

	aboveHardCap := m.isAboveHardCap(usage)
	minTokens := m.Config.Compaction.MinContextTokens
	if aboveHardCap {
		minTokens = m.hardMinTokens()
	}

	cutIndex := m.findCutPoint(sess.Messages, minTokens)
	if cutIndex <= 0 {
		return false, nil
	}

	pruned := sess.Messages[:cutIndex]
	retained := sess.Messages[cutIndex:]
	cleanRetained, removed := session.SanitizeMessages(retained)
	pruned = append(pruned, removed...)
	retained = cleanRetained

	// Attempt summarization.
	summary, err := m.summarize(sess.Folder, pruned)
	if err != nil {
		if aboveHardCap {
			// Force prune without summary.
			sess.Messages = retained
			return true, sess.Save()
		}
		// Below hard cap: skip prune.
		return false, nil
	}

	if strings.TrimSpace(summary) == "" {
		if aboveHardCap {
			sess.Messages = retained
			return true, sess.Save()
		}
		return false, nil
	}

	// Save the summary chunk.
	if err := m.saveSummary(sess.Folder, summary); err != nil {
		return false, fmt.Errorf("cannot save summary: %w", err)
	}

	// Trim old summary files.
	if err := m.trimSummaries(sess.Folder); err != nil {
		return false, fmt.Errorf("cannot trim summaries: %w", err)
	}

	// Build synthetic summary message and prepend to retained tail.
	synthetic := m.buildSyntheticMessage(sess.Folder)
	sess.Messages = append([]session.Message{synthetic}, retained...)
	return true, sess.Save()
}

// summarize builds a transcript from pruned messages and sends it to the LLM for summarization.
//
// WHAT:  Creates a transcript of pruned messages and asks the LLM for a dense summary.
// PARAMS: sessionFolder — path to the session folder (for reading existing summaries); pruned — messages to summarize.
// RETURNS: string — summary text; error if the LLM call fails.
func (m *Manager) summarize(sessionFolder string, pruned []session.Message) (string, error) {
	transcript := m.buildTranscript(pruned)
	if strings.TrimSpace(transcript) == "" {
		return "", nil
	}

	// Load existing summaries as context.
	existing := m.loadSummaries(sessionFolder)
	summaryPrompt := buildSummaryPrompt(transcript, existing, m.Config.Compaction.SummaryMaxTokens)

	// Use the default model for summarization (per spec 05).
	resp, err := m.Provider.Stream(
		context.Background(),
		[]session.Message{
			{Role: "system", Content: summaryPrompt},
			{Role: "user", Content: "Summarize the above conversation segment."},
		},
		nil, nil,
	)
	if err != nil {
		return "", fmt.Errorf("summarization LLM call failed: %w", err)
	}

	return resp.Content, nil
}

// buildTranscript constructs a text transcript from pruned messages for the summarizer.
// Reasoning is included as [REASONING]...[/REASONING] only for the newest N reasoning messages.
//
// WHAT:  Converts pruned messages into a compact text transcript.
// WHY:   The summarizer needs a text representation of the pruned segment.
// PARAMS: pruned — the messages being removed from context.
// RETURNS: string — the transcript text.
func (m *Manager) buildTranscript(pruned []session.Message) string {
	var sb strings.Builder
	preserveLast := m.Config.StripReasoning.PreserveLast

	// Pre-compute which reasoning messages to keep (newest N).
	reasoningIndices := make(map[int]bool)
	kept := 0
	for i := len(pruned) - 1; i >= 0; i-- {
		if pruned[i].Reasoning != "" {
			if m.Config.StripReasoning.Enable {
				reasoningCount := 0
				for _, msg := range pruned {
					if msg.Reasoning != "" {
						reasoningCount++
					}
				}
				if reasoningCount > preserveLast && kept >= preserveLast {
					continue
				}
			}
			reasoningIndices[i] = true
			kept++
		}
	}

	for i, msg := range pruned {
		var parts []string

		// Include reasoning for newest N only.
		if msg.Reasoning != "" && reasoningIndices[i] {
			parts = append(parts, fmt.Sprintf("[REASONING]%s[/REASONING]", msg.Reasoning))
		}

		if content, ok := msg.Content.(string); ok && content != "" {
			parts = append(parts, content)
		}

		if msg.ToolCalls != nil {
			data, _ := json.Marshal(msg.ToolCalls)
			parts = append(parts, fmt.Sprintf("[TOOL_CALLS] %s", string(data)))
		}

		if msg.Role == "tool" {
			if content, ok := msg.Content.(string); ok {
				parts = append(parts, fmt.Sprintf("[TOOL_RESULT %s] %s", msg.Name, content))
			}
		}

		if len(parts) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("[%s] %s\n", msg.Role, strings.Join(parts, " ")))
	}

	return sb.String()
}

// buildSummaryPrompt creates the system prompt for the summarization LLM call.
//
// WHAT:  Builds the summarization instruction with existing summaries as context.
// PARAMS: transcript — the pruned segment as text; existing — previously saved summaries; maxTokens — token budget.
// RETURNS: string — the complete summarization prompt.
func buildSummaryPrompt(transcript, existing string, maxTokens int) string {
	var sb strings.Builder
	sb.WriteString("You are a conversation summarizer. Produce a dense, append-only technical summary of the conversation segment below.\n")
	sb.WriteString("Focus on facts, decisions, actions taken, and their outcomes. Omit pleasantries.\n")
	sb.WriteString(fmt.Sprintf("Keep the summary under approximately %d tokens.\n\n", maxTokens))

	if existing != "" {
		sb.WriteString("Existing historical summaries (read-only context):\n")
		sb.WriteString(existing)
		sb.WriteString("\n\n")
	}

	sb.WriteString("Conversation segment to summarize:\n")
	sb.WriteString(transcript)
	return sb.String()
}

// summariesDir returns the path to the summaries subfolder in a session.
//
// PARAMS: sessionFolder — path to the session folder.
// RETURNS: string — path to the summaries subfolder.
func summariesDir(sessionFolder string) string {
	return filepath.Join(sessionFolder, "summaries")
}

// saveSummary writes a new summary chunk to the session's summaries folder.
// File names are zero-padded sequential numbers.
//
// WHAT:  Persists a summary chunk as a numbered .md file.
// PARAMS: sessionFolder — path to the session folder; content — summary text.
// RETURNS: error if file creation fails.
func (m *Manager) saveSummary(sessionFolder, content string) error {
	dir := summariesDir(sessionFolder)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create summaries dir: %w", err)
	}

	// Find the next sequence number.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("cannot read summaries dir: %w", err)
	}
	maxNum := 0
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".md") {
			var num int
			fmt.Sscanf(name, "%d.md", &num)
			if num > maxNum {
				maxNum = num
			}
		}
	}

	nextNum := maxNum + 1
	filename := fmt.Sprintf("%06d.md", nextNum)
	path := filepath.Join(dir, filename)
	return os.WriteFile(path, []byte(content), 0644)
}

// loadSummaries reads all summary chunks from the session's summaries folder in chronological order.
//
// WHAT:  Loads existing summary files for context injection or summarizer context.
// PARAMS: sessionFolder — path to the session folder.
// RETURNS: string — concatenated summary text in chronological order; empty if none exist.
func (m *Manager) loadSummaries(sessionFolder string) string {
	dir := summariesDir(sessionFolder)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	var files []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".md") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	var sb strings.Builder
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		sb.WriteString(string(data))
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// trimSummaries deletes the oldest summary files beyond maxSummaryFiles.
//
// WHAT:  Enforces the maxSummaryFiles limit by deleting oldest files.
// PARAMS: sessionFolder — path to the session folder.
// RETURNS: error if deletion fails.
func (m *Manager) trimSummaries(sessionFolder string) error {
	maxFiles := m.Config.Compaction.MaxSummaryFiles
	dir := summariesDir(sessionFolder)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var files []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".md") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	if len(files) <= maxFiles {
		return nil
	}

	// Delete oldest files beyond maxFiles.
	toDelete := files[:len(files)-maxFiles]
	for _, name := range toDelete {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return fmt.Errorf("cannot delete %s: %w", name, err)
		}
	}
	return nil
}

// buildSyntheticMessage creates a synthetic system message containing all retained summaries.
//
// WHAT:  Bundles all summary chunks into a single synthetic message prepended to the session.
// PARAMS: sessionFolder — path to the session folder.
// RETURNS: session.Message — synthetic message with summary context.
func (m *Manager) buildSyntheticMessage(sessionFolder string) session.Message {
	summaries := m.loadSummaries(sessionFolder)

	var sb strings.Builder
	sb.WriteString("These are historical segment summaries of messages removed from context.\n")
	sb.WriteString("Older summaries appear first. Newer summaries override older ones on conflicts.\n")
	sb.WriteString("Retained messages that follow are newer than all summaries.\n\n")
	sb.WriteString(summaries)

	return session.Message{
		Role:    "system",
		Content: sb.String(),
	}
}

// syntheticPrefix is the text prefix that identifies a synthetic summary message.
// Used to detect and replace existing synthetic messages on resume.
const syntheticPrefix = "These are historical segment summaries"

// LoadSummariesForResume rebuilds the synthetic summary message when continuing a session with -c.
//
// WHAT:  Loads summaries from disk and returns a synthetic message for session resume.
// WHY:   On -c, the synthetic message must be rebuilt from summary files.
// PARAMS: sessionFolder — path to the session folder.
// RETURNS: session.Message — synthetic message; nil if no summaries exist.
func (m *Manager) LoadSummariesForResume(sessionFolder string) *session.Message {
	summaries := m.loadSummaries(sessionFolder)
	if summaries == "" {
		return nil
	}
	msg := m.buildSyntheticMessage(sessionFolder)
	return &msg
}

// RebuildForResume removes any existing synthetic summary message from the session
// and prepends a fresh one rebuilt from summary files on disk.
//
// WHAT:  Rebuilds the synthetic summary message on -c resume.
// WHY:   Spec 05 requires summaries loaded automatically and synthetic message rebuilt on -c.
// HOW:   Detects existing synthetic by prefix, removes it, rebuilds from summary files, prepends.
// PARAMS: sess — the resumed session.
// RETURNS: error if saving fails.
func (m *Manager) RebuildForResume(sess *session.Session) error {
	// Remove existing synthetic message(s) from the start of the message array.
	filtered := make([]session.Message, 0, len(sess.Messages))
	for i, msg := range sess.Messages {
		if content, ok := msg.Content.(string); ok && strings.HasPrefix(content, syntheticPrefix) {
			continue
		}
		filtered = append(filtered, sess.Messages[i])
	}
	sess.Messages = filtered

	// Rebuild synthetic from summary files.
	synthetic := m.LoadSummariesForResume(sess.Folder)
	if synthetic == nil {
		// No summaries — just save the filtered session.
		return sess.Save()
	}

	// Prepend synthetic message.
	sess.Messages = append([]session.Message{*synthetic}, sess.Messages...)
	return sess.Save()
}

// StripReasoningFromPayload replaces reasoning parts in the message array with empty text.
// Only the newest N reasoning parts are kept. The count is global across all messages.
//
// WHAT:  Removes reasoning from the payload sent to the LLM, keeping only the newest N.
// WHY:   Reduces token usage while preserving the newest reasoning context.
// HOW:   Counts messages with non-empty reasoning from the end, clears reasoning on older messages.
// PARAMS: messages — the message array to strip.
// RETURNS: []session.Message — new array with reasoning stripped on older messages.
func (m *Manager) StripReasoningFromPayload(messages []session.Message) []session.Message {
	if !m.Config.StripReasoning.Enable {
		return messages
	}

	preserveLast := m.Config.StripReasoning.PreserveLast
	result := make([]session.Message, len(messages))
	copy(result, messages)

	// Count messages with reasoning from the end (global count).
	reasoningCount := 0
	for i := len(result) - 1; i >= 0; i-- {
		if result[i].Reasoning != "" {
			reasoningCount++
		}
	}

	// If total reasoning messages <= preserveLast, keep all.
	if reasoningCount <= preserveLast {
		return result
	}

	// Walk from newest, keep reasoning for the newest N, strip the rest.
	kept := 0
	for i := len(result) - 1; i >= 0; i-- {
		if result[i].Reasoning == "" {
			continue
		}
		if kept < preserveLast {
			kept++
			continue
		}
		// Strip reasoning from this message.
		result[i].Reasoning = ""
	}

	return result
}
