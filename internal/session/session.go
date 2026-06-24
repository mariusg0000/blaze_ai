// session.go — file-based session persistence.
// Each session is a folder under app_home/sessions/ with a random name containing session.json.
// session.json holds the complete message array exactly as sent to the LLM and a closed_cleanly flag.
// New sessions start by default; -c resumes the last cleanly closed session.
// Layer: session storage. Dependencies: internal/platform (app home path resolution).
package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"blazeai/internal/platform"
	"blazeai/internal/tools"
)

// ErrNoCleanSession is returned when -c is used but no cleanly closed session exists.
var ErrNoCleanSession = errors.New("no cleanly closed session found")

// ErrNoSessions is returned when -r is used but no sessions exist at all.
var ErrNoSessions = errors.New("no sessions found")

// ErrSessionNotFound is returned when a session folder or file does not exist.
var ErrSessionNotFound = errors.New("session not found")

// sessionJSONName is the filename for the session data inside each session folder.
const sessionJSONName = "session.json"

// Message represents a single message in the conversation, exactly as sent to the LLM.
// The structure follows the OpenAI chat message format with role and content.
// Tool calls and tool results are preserved as-is for session replay.
// Reasoning is stored intact on disk and stripped only from the LLM payload.
//
// WHAT:  One message in the conversation history.
// WHY:   session.json stores the complete message array for prompt rebuilding and resume.
// PARAMS: Role — sender role; Content — message text; Reasoning — reasoning text (kept intact on disk);
//
//	ToolCalls — optional tool call array; ToolCallID — optional tool result reference ID;
//	Name — optional tool name for results.
type Message struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content,omitempty"`
	Reasoning  string      `json:"reasoning,omitempty"`
	ToolCalls  interface{} `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	Name       string      `json:"name,omitempty"`
}

// Session holds the persisted state of a single conversation session.
//
// WHAT:  The on-disk representation of a session.
// WHY:   Tracks the message array and clean-close status for persistence and resume.
// PARAMS: Messages — full conversation array; ClosedCleanly — true only when closed via /exit;
//
//	Folder — absolute path to the session folder.
type Session struct {
	Messages      []Message `json:"messages"`
	ClosedCleanly bool      `json:"closed_cleanly"`
	Folder        string    `json:"-"`
}

// randomName generates a random folder name for a new session.
// Format: timestamp prefix + random hex suffix for uniqueness and sortability.
//
// WHAT:  Generates a unique session folder name.
// WHY:   Each session needs a unique folder; the timestamp prefix enables chronological sorting.
// RETURNS: string — folder name like "20260622-095500-a1b2c3d4".
func randomName() string {
	now := time.Now().Format("20060102-150405")
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%s", now, hex.EncodeToString(b))
}

// Create makes a new session folder with a random name and writes an empty session.json.
// Sessions are stored under app_home/projects/<project>/sessions/.
//
// WHAT:  Creates a new session on disk with an empty message array.
// WHY:   Every new conversation starts with a fresh session folder.
// HOW:   Resolves the project sessions dir from workDir, generates a random name, writes initial session.json.
// PARAMS: workDir — the current working directory used to locate the project folder.
// RETURNS: *Session — ready-to-use session with empty messages; error if folder creation fails.
func Create(workDir string) (*Session, error) {
	dir, err := platform.EnsureProjectDir(workDir)
	if err != nil {
		return nil, err
	}
	name := randomName()
	folder := filepath.Join(dir, name)
	if err := os.MkdirAll(folder, 0755); err != nil {
		return nil, fmt.Errorf("cannot create session folder %s: %w", folder, err)
	}
	s := &Session{
		Messages:      []Message{},
		ClosedCleanly: false,
		Folder:        folder,
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return s, nil
}

// Load reads session.json from a given session folder path.
//
// WHAT:  Loads a session from disk by folder path.
// WHY:   -c resume and session inspection need to read existing sessions.
// PARAMS: folder — absolute path to the session folder.
// RETURNS: *Session — loaded session; error if folder or session.json is missing or malformed.
func Load(folder string) (*Session, error) {
	path := filepath.Join(folder, sessionJSONName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, folder)
		}
		return nil, fmt.Errorf("cannot read session file: %w", err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("cannot parse session file: %w", err)
	}
	s.Folder = folder
	return &s, nil
}

// save writes the session to session.json in its folder.
//
// WHAT:  Persists the session state to disk.
// WHY:   The session is updated as the conversation progresses.
// RETURNS: error if marshaling or writing fails.
func (s *Session) save() error {
	path := filepath.Join(s.Folder, sessionJSONName)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal session: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("cannot write session file %s: %w", path, err)
	}
	return nil
}

// Save writes the session to disk, preserving the full message array and closed_cleanly flag.
//
// WHAT:  Public persistence method for the session.
// WHY:   The runtime saves after each turn; Close saves with closed_cleanly=true.
// RETURNS: error if writing fails.
func (s *Session) Save() error {
	return s.save()
}

// Append adds a message to the session and saves to disk.
//
// WHAT:  Appends one message to the conversation and persists.
// WHY:   Each user input, assistant response, and tool result is appended as it occurs.
// PARAMS: msg — the message to append.
// RETURNS: error if saving fails.
func (s *Session) Append(msg Message) error {
	s.Messages = append(s.Messages, msg)
	return s.save()
}

// AppendAll adds multiple messages to the session and saves to disk.
//
// WHAT:  Appends several messages at once and persists.
// WHY:   A single LLM turn may produce an assistant message plus multiple tool results.
// PARAMS: msgs — the messages to append.
// RETURNS: error if saving fails.
func (s *Session) AppendAll(msgs []Message) error {
	s.Messages = append(s.Messages, msgs...)
	return s.save()
}

// Reset clears the session history and marks it as open.
//
// WHAT:  Removes all conversation messages from the session.
// WHY:   /clear and /new need to restart the current session without changing its folder name.
// RETURNS: error if persistence fails.
func (s *Session) Reset() error {
	s.Messages = []Message{}
	s.ClosedCleanly = false
	return s.save()
}

// Close marks the session as cleanly closed and saves.
//
// WHAT:  Sets closed_cleanly to true and persists.
// WHY:   Only sessions closed via /exit are marked clean; -c resumes only clean sessions.
// RETURNS: error if saving fails.
func (s *Session) Close() error {
	s.ClosedCleanly = true
	return s.save()
}

// Sanitize removes invalid tool-call history from the session.
// Orphan tool messages are dropped. An incomplete assistant tool-call round
// truncates the history from that assistant message onward.
//
// WHAT:  Strips incomplete assistant/tool-call rounds from the session.
// WHY:   Interrupted sessions can leave assistant messages with tool_calls that have
//
//	no corresponding tool results, and later user messages cannot repair that history.
//
// RETURNS: error if saving the sanitized session fails.
func (s *Session) Sanitize() error {
	sanitized, _ := SanitizeMessages(s.Messages)
	s.Messages = sanitized
	return nil
}

// SanitizeMessages validates a message slice and returns the sanitized messages plus any removed ones.
// Orphan tool messages are removed. If an assistant tool-call round is incomplete,
// the history is truncated from that assistant message onward and the truncated tail is returned in removed.
//
// WHAT:  Sanitizes an arbitrary message slice without mutating session state.
// WHY:   Runtime and compaction both need the same validity rules for tool-call history.
// HOW:   Drops orphan tool messages, keeps complete assistant/tool rounds, truncates on incomplete rounds.
// PARAMS: messages — the message slice to validate.
// RETURNS: []Message — sanitized messages; []Message — removed or truncated messages in original order.
func SanitizeMessages(messages []Message) ([]Message, []Message) {
	sanitized := make([]Message, 0, len(messages))
	removed := make([]Message, 0)

	for i := 0; i < len(messages); {
		msg := messages[i]

		if msg.Role == "tool" {
			removed = append(removed, msg)
			i++
			continue
		}

		expectedIDs := assistantToolCallIDs(msg.ToolCalls)
		if msg.Role != "assistant" || len(expectedIDs) == 0 {
			sanitized = append(sanitized, msg)
			i++
			continue
		}

		groupStart := len(sanitized)
		sanitized = append(sanitized, msg)

		expected := make(map[string]struct{}, len(expectedIDs))
		for _, id := range expectedIDs {
			expected[id] = struct{}{}
		}
		seen := make(map[string]struct{}, len(expectedIDs))

		j := i + 1
		for j < len(messages) && messages[j].Role == "tool" {
			toolMsg := messages[j]
			if toolMsg.ToolCallID == "" {
				removed = append(removed, toolMsg)
				j++
				continue
			}
			if _, ok := expected[toolMsg.ToolCallID]; !ok {
				removed = append(removed, toolMsg)
				j++
				continue
			}
			if _, duplicate := seen[toolMsg.ToolCallID]; duplicate {
				removed = append(removed, toolMsg)
				j++
				continue
			}
			seen[toolMsg.ToolCallID] = struct{}{}
			sanitized = append(sanitized, toolMsg)
			j++
		}

		if len(seen) != len(expected) {
			removed = append(removed, messages[i:]...)
			return sanitized[:groupStart], removed
		}

		i = j
	}

	return sanitized, removed
}

// assistantToolCallIDs extracts tool call IDs from an assistant tool_calls payload.
func assistantToolCallIDs(tc interface{}) []string {
	switch v := tc.(type) {
	case []interface{}:
		ids := make([]string, 0, len(v))
		for _, item := range v {
			if id, ok := toolCallID(item); ok {
				ids = append(ids, id)
			}
		}
		return ids
	case []map[string]interface{}:
		ids := make([]string, 0, len(v))
		for _, item := range v {
			if id, ok := item["id"].(string); ok && id != "" {
				ids = append(ids, id)
			}
		}
		return ids
	case []tools.OpenAIToolCall:
		ids := make([]string, 0, len(v))
		for _, item := range v {
			if item.ID != "" {
				ids = append(ids, item.ID)
			}
		}
		return ids
	default:
		return nil
	}
}

// toolCallID extracts an ID from one tool_calls element.
func toolCallID(item interface{}) (string, bool) {
	switch v := item.(type) {
	case map[string]interface{}:
		id, ok := v["id"].(string)
		return id, ok && id != ""
	default:
		return "", false
	}
}

// LastClean finds the most recently modified cleanly closed session in the current project.
// Used by -c to resume the last session closed via /exit.
//
// WHAT:  Searches project sessions/ for the newest session with closed_cleanly=true.
// WHY:   -c continues the last cleanly closed session per spec.
// HOW:   Resolves project sessions dir from workDir, lists folders, loads each, filters for closed_cleanly, picks newest.
// PARAMS: workDir — the current working directory used to locate the project folder.
// RETURNS: *Session — the last clean session; ErrNoCleanSession if none exists.
func LastClean(workDir string) (*Session, error) {
	dir, err := platform.EnsureProjectDir(workDir)
	if err != nil {
		return nil, err
	}
	return LastCleanInDir(dir)
}

// CreateInDir creates a new session in an explicit sessions directory.
// Test-friendly variant of Create.
//
// WHAT:  Same as Create but with an explicit sessions directory.
// WHY:   Enables testing with temp directories.
// PARAMS: dir — the sessions directory to create the session in.
// RETURNS: *Session — new session; error if folder creation fails.
func CreateInDir(dir string) (*Session, error) {
	name := randomName()
	folder := filepath.Join(dir, name)
	if err := os.MkdirAll(folder, 0755); err != nil {
		return nil, fmt.Errorf("cannot create session folder %s: %w", folder, err)
	}
	s := &Session{
		Messages:      []Message{},
		ClosedCleanly: false,
		Folder:        folder,
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return s, nil
}

// Last finds the most recently modified session in the current project regardless of closed_cleanly status.
// Used by -r to resume the last interrupted or cleanly closed session.
//
// WHAT:  Searches project sessions/ for the newest session by modification time.
// WHY:   -r resumes the most recent session whether it was cleanly closed or interrupted.
// HOW:   Resolves project sessions dir from workDir, delegates to LastInDir.
// PARAMS: workDir — the current working directory used to locate the project folder.
// RETURNS: *Session — the most recent session; ErrNoSessions if none exist.
func Last(workDir string) (*Session, error) {
	dir, err := platform.EnsureProjectDir(workDir)
	if err != nil {
		return nil, err
	}
	return LastInDir(dir)
}

// LastInDir finds the most recently modified session in an explicit directory.
// Test-friendly variant of Last. Does not filter by closed_cleanly.
//
// WHAT:  Same as Last but with an explicit sessions directory.
// WHY:   Enables testing with temp directories.
// PARAMS: dir — the sessions directory to search.
// RETURNS: *Session — most recent session; ErrNoSessions if none exist.
func LastInDir(dir string) (*Session, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoSessions
		}
		return nil, fmt.Errorf("cannot list sessions: %w", err)
	}

	type candidate struct {
		folder string
		mtime  time.Time
	}
	var candidates []candidate

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		folder := filepath.Join(dir, entry.Name())
		if _, err := Load(folder); err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{
			folder: folder,
			mtime:  info.ModTime(),
		})
	}

	if len(candidates) == 0 {
		return nil, ErrNoSessions
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].mtime.After(candidates[j].mtime)
	})

	return Load(candidates[0].folder)
}

// LastCleanInDir finds the last cleanly closed session in an explicit directory.
// Test-friendly variant of LastClean.
//
// WHAT:  Same as LastClean but with an explicit sessions directory.
// WHY:   Enables testing with temp directories.
// PARAMS: dir — the sessions directory to search.
// RETURNS: *Session — last clean session; ErrNoCleanSession if none exists.
func LastCleanInDir(dir string) (*Session, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoCleanSession
		}
		return nil, fmt.Errorf("cannot list sessions: %w", err)
	}

	type candidate struct {
		folder string
		mtime  time.Time
	}
	var candidates []candidate

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		folder := filepath.Join(dir, entry.Name())
		s, err := Load(folder)
		if err != nil {
			continue
		}
		if s.ClosedCleanly {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			candidates = append(candidates, candidate{
				folder: folder,
				mtime:  info.ModTime(),
			})
		}
	}

	if len(candidates) == 0 {
		return nil, ErrNoCleanSession
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].mtime.After(candidates[j].mtime)
	})

	return Load(candidates[0].folder)
}
