// raw_capture.go — stream-scoped raw LLM response capture.
// Layer: session storage. Dependencies: bufio and standard file I/O.
package session

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
)

// RawCapture owns one buffered private capture file for a provider stream.
type RawCapture struct {
	file   *os.File
	writer *bufio.Writer
}

// NewRawCapture creates or truncates a private capture file with mode 0600.
func NewRawCapture(folder, filename string) (*RawCapture, error) {
	if folder == "" || filename == "" {
		return nil, fmt.Errorf("raw capture folder and filename are required")
	}
	file, err := os.OpenFile(filepath.Join(folder, filename), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("cannot create raw capture: %w", err)
	}
	return &RawCapture{file: file, writer: bufio.NewWriter(file)}, nil
}

// Append writes the exact payload followed by one newline.
func (c *RawCapture) Append(payload []byte) error {
	if c == nil || c.writer == nil {
		return fmt.Errorf("raw capture is closed")
	}
	if _, err := c.writer.Write(payload); err != nil {
		return fmt.Errorf("cannot write raw capture: %w", err)
	}
	if err := c.writer.WriteByte('\n'); err != nil {
		return fmt.Errorf("cannot write raw capture newline: %w", err)
	}
	return nil
}

// Close flushes buffered data and closes the capture file.
func (c *RawCapture) Close() error {
	if c == nil || c.file == nil {
		return nil
	}
	flushErr := c.writer.Flush()
	closeErr := c.file.Close()
	c.file = nil
	if flushErr != nil {
		return flushErr
	}
	return closeErr
}
