// usage.go — private per-session token and prompt-cache analytics.
// Stores aggregate provider usage by model for personal inspection only.
// Layer: session storage. Dependencies: encoding/json and standard file I/O.
package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const usageFileName = "token-usage.json"

// UsageSnapshot is the aggregate token and cache data for one model.
//
// WHAT: Captures cumulative usage counters for a model in one session.
// HOW:  Counts provider responses and sums reported token fields; cache counters count only explicit cache metadata.
type UsageSnapshot struct {
	Requests            int       `json:"requests"`
	PromptTokens        int       `json:"prompt_tokens"`
	CompletionTokens    int       `json:"completion_tokens"`
	TotalTokens         int       `json:"total_tokens"`
	CachedTokens        int       `json:"cached_tokens"`
	CacheWriteTokens    int       `json:"cache_write_tokens"`
	UncachedInputTokens int       `json:"uncached_input_tokens"`
	CacheHits           int       `json:"cache_hits"`
	CacheMisses         int       `json:"cache_misses"`
	UnknownCacheStatus  int       `json:"unknown_cache_status"`
	LastUpdated         time.Time `json:"last_updated"`
}

// UsageReport is the complete private analytics document for one session.
//
// WHAT: Groups token counters by provider/model identifier.
// HOW:  The file is rewritten atomically after each completed provider response.
type UsageReport struct {
	SessionFolder string                   `json:"session_folder"`
	UpdatedAt     time.Time                `json:"updated_at"`
	Models        map[string]UsageSnapshot `json:"models"`
}

var usageMu sync.Mutex

// RecordUsage adds one provider response to the session's private usage report.
//
// WHAT: Persists token, cache, and model counters after an LLM response.
// HOW:  Loads the existing aggregate, updates one model, and atomically replaces token-usage.json.
// PARAMS: folder — session directory; modelID — provider/model identifier; usage — provider usage data.
// RETURNS: error when the report cannot be read or written.
func RecordUsage(folder, modelID string, usage UsageData) error {
	if folder == "" || modelID == "" {
		return fmt.Errorf("usage folder and model ID are required")
	}
	usageMu.Lock()
	defer usageMu.Unlock()

	path := filepath.Join(folder, usageFileName)
	report := UsageReport{SessionFolder: folder, Models: make(map[string]UsageSnapshot)}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &report); err != nil {
			return fmt.Errorf("cannot parse token usage report: %w", err)
		}
		if report.Models == nil {
			report.Models = make(map[string]UsageSnapshot)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot read token usage report: %w", err)
	}

	snapshot := report.Models[modelID]
	snapshot.Requests++
	snapshot.PromptTokens += usage.PromptTokens
	snapshot.CompletionTokens += usage.CompletionTokens
	snapshot.TotalTokens += usage.TotalTokens
	snapshot.CachedTokens += usage.CachedTokens
	snapshot.CacheWriteTokens += usage.CacheWriteTokens
	switch usage.CacheStatus {
	case "hit", "miss":
		snapshot.UncachedInputTokens += usage.PromptTokens - usage.CachedTokens
		if usage.CacheStatus == "hit" {
			snapshot.CacheHits++
		} else {
			snapshot.CacheMisses++
		}
	default:
		snapshot.UnknownCacheStatus++
	}
	snapshot.LastUpdated = time.Now().UTC()
	report.Models[modelID] = snapshot
	report.UpdatedAt = snapshot.LastUpdated
	report.SessionFolder = folder

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot encode token usage report: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("cannot write token usage report: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("cannot replace token usage report: %w", err)
	}
	return nil
}

// UsageData is the provider-neutral usage input accepted by the session report.
//
// WHAT: Transfers token and cache counters without coupling session storage to a provider package.
// HOW:  CacheStatus is hit, miss, or unknown when the provider does not report cache details.
type UsageData struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CachedTokens     int
	CacheWriteTokens int
	CacheStatus      string
}

// CacheKeyForSession returns a stable, non-sensitive cache key for a session folder.
//
// WHAT: Produces the prompt-cache identity used for all model requests in this session.
// HOW:  Hashes the folder path so the provider never receives local filesystem details.
func CacheKeyForSession(folder string) string {
	sum := sha256.Sum256([]byte(folder))
	return hex.EncodeToString(sum[:])
}
