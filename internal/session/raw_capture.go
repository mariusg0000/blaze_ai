// raw_capture.go — private raw LLM response capture.
// Stores the latest exact JSON response events for provider debugging.
// Layer: session storage. Dependencies: standard file I/O and synchronization.
package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var rawCaptureMu sync.Mutex

// ResetRawJSON replaces a named private session capture file before a new response.
//
// WHAT: Starts a fresh capture for the next LLM response.
// HOW:  Truncates or creates the file with restrictive permissions.
func ResetRawJSON(folder, filename string) error {
	if folder == "" || filename == "" {
		return fmt.Errorf("raw capture folder and filename are required")
	}
	rawCaptureMu.Lock()
	defer rawCaptureMu.Unlock()
	file, err := os.OpenFile(filepath.Join(folder, filename), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("cannot reset raw capture: %w", err)
	}
	return file.Close()
}

// AppendRawJSON appends one exact JSON payload to a named private session capture file.
//
// WHAT: Persists one provider response event for protocol analysis.
// HOW:  Appends one payload per line without decoding or re-encoding it.
func AppendRawJSON(folder, filename string, payload []byte) error {
	if folder == "" || filename == "" {
		return fmt.Errorf("raw capture folder and filename are required")
	}
	rawCaptureMu.Lock()
	defer rawCaptureMu.Unlock()
	file, err := os.OpenFile(filepath.Join(folder, filename), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("cannot open raw capture: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("cannot write raw capture: %w", err)
	}
	return nil
}
