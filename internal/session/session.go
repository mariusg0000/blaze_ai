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
//         ToolCalls — optional tool call array; ToolCallID — optional tool result reference ID;
//         Name — optional tool name for results.
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
//         Folder — absolute path to the session folder.
type Session struct {
	Messages      []Message `json:"messages"`
	ClosedCleanly bool      `json:"closed_cleanly"`
	Folder        string    `json:"-"`
}

// sessionsDir resolves the sessions directory under app home.
//
// WHAT:  Returns the absolute path to the sessions directory.
// WHY:   Session folders are created and found under this path.
// RETURNS: string — path to app_home/sessions/; error if app home cannot be resolved.
func sessionsDir() (string, error) {
	home, err := platform.AppHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "sessions"), nil
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
//
// WHAT:  Creates a new session on disk with an empty message array.
// WHY:   Every new conversation starts with a fresh session folder.
// HOW:   Generates a random name, creates the folder, writes initial session.json.
// RETURNS: *Session — ready-to-use session with empty messages; error if folder creation fails.
func Create() (*Session, error) {
	dir, err := sessionsDir()
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

// Close marks the session as cleanly closed and saves.
//
// WHAT:  Sets closed_cleanly to true and persists.
// WHY:   Only sessions closed via /exit are marked clean; -c resumes only clean sessions.
// RETURNS: error if saving fails.
func (s *Session) Close() error {
	s.ClosedCleanly = true
	return s.save()
}

// Sanitize removes incomplete tool call rounds from the session history.
// An incomplete round is an assistant message with tool_calls that lacks matching
// tool results before the next non-tool message. When found, the session is
// truncated from that assistant message onward.
//
// WHAT:  Strips incomplete assistant/tool-call rounds from the session.
// WHY:   Interrupted sessions can leave assistant messages with tool_calls that have
//        no corresponding tool results, and later user messages cannot repair that history.
// RETURNS: error if saving the sanitized session fails.
func (s *Session) Sanitize() error {
	for i := 0; i < len(s.Messages); i++ {
		if s.Messages[i].Role != "assistant" {
			continue
		}

		expectedResults := assistantToolCallCount(s.Messages[i].ToolCalls)
		if expectedResults == 0 {
			continue
		}

		actualResults := 0
		j := i + 1
		for j < len(s.Messages) && s.Messages[j].Role == "tool" {
			actualResults++
			j++
		}

		if actualResults < expectedResults {
			s.Messages = s.Messages[:i]
			return nil
		}

		i = j - 1
	}

	return nil
}

// assistantToolCallCount returns the number of tool calls in an assistant message.
func assistantToolCallCount(tc interface{}) int {
	if tc == nil {
		return 0
	}
	switch v := tc.(type) {
	case []interface{}:
		return len(v)
	}
	return 0
}

// LastClean finds the most recently modified cleanly closed session.
// Used by -c to resume the last session closed via /exit.
//
// WHAT:  Searches sessions/ for the newest session with closed_cleanly=true.
// WHY:   -c continues the last cleanly closed session per spec.
// HOW:   Lists session folders, loads each, filters for closed_cleanly, picks newest by mod time.
// RETURNS: *Session — the last clean session; ErrNoCleanSession if none exists.
func LastClean() (*Session, error) {
	dir, err := sessionsDir()
	if err != nil {
		return nil, err
	}
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

// Last finds the most recently modified session regardless of closed_cleanly status.
// Used by -r to resume the last interrupted or cleanly closed session.
//
// WHAT:  Searches sessions/ for the newest session by modification time.
// WHY:   -r resumes the most recent session whether it was cleanly closed or interrupted.
// HOW:   Lists session folders, sorts by mod time, returns the newest.
// RETURNS: *Session — the most recent session; ErrNoSessions if none exist.
func Last() (*Session, error) {
	dir, err := sessionsDir()
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
