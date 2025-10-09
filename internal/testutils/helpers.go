// Package testutils provides testing utilities and helpers for logwrap tests.
package testutils

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	// Test timeout duration.
	testTimeout = 30 * time.Second
	// Polling interval for condition checks.
	pollingInterval = 10 * time.Millisecond
	// File permissions for test config files.
	configFilePermissions = 0600
)

// Predefined errors for test scenarios.
var (
	ErrMockWriteFailure = errors.New("mock write failure")
)

// CreateTempConfigFile creates a temporary YAML config file with the given content.
func CreateTempConfigFile(t *testing.T, content string) string {
	t.Helper()

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.yaml")

	err := os.WriteFile(configFile, []byte(content), configFilePermissions)
	require.NoError(t, err)

	return configFile
}

// CreateTempDir creates a temporary directory and returns its path.
func CreateTempDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// CaptureOutput captures output written to the provided writer.
type CaptureOutput struct {
	buffer *bytes.Buffer
}

// NewCaptureOutput creates a new output capturer.
func NewCaptureOutput() *CaptureOutput {
	return &CaptureOutput{
		buffer: &bytes.Buffer{},
	}
}

// Writer returns the io.Writer interface for capturing output.
func (c *CaptureOutput) Writer() io.Writer {
	return c.buffer
}

// String returns the captured output as a string.
func (c *CaptureOutput) String() string {
	return c.buffer.String()
}

// Reset clears the captured output.
func (c *CaptureOutput) Reset() {
	c.buffer.Reset()
}

// MockReader provides a mock io.Reader for testing.
type MockReader struct {
	data   []byte
	pos    int
	closed bool
}

// NewMockReader creates a new mock reader with the given data.
func NewMockReader(data string) *MockReader {
	return &MockReader{
		data: []byte(data),
		pos:  0,
	}
}

// Read implements io.Reader interface.
func (r *MockReader) Read(p []byte) (int, error) {
	if r.closed {
		return 0, io.EOF
	}

	if r.pos >= len(r.data) {
		return 0, io.EOF
	}

	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// Close simulates closing the reader.
func (r *MockReader) Close() error {
	r.closed = true
	return nil
}

// MultiLineReader creates a reader that provides multiple lines of text.
func MultiLineReader(lines []string) io.Reader {
	data := strings.Join(lines, "\n")
	if len(lines) > 0 {
		data += "\n" // Add final newline
	}
	return strings.NewReader(data)
}

// TestContext creates a test context with timeout.
func TestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	t.Cleanup(cancel)
	return ctx
}

// AssertEventuallyTrue polls a condition until it becomes true or times out.
func AssertEventuallyTrue(t *testing.T, condition func() bool, timeout time.Duration, msg string) {
	t.Helper()

	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			t.Fatalf("Timeout waiting for condition: %s", msg)
		case <-ticker.C:
			if condition() {
				return
			}
		}
	}
}

// TempFile creates a temporary file with the given content and returns its path.
func TempFile(t *testing.T, content string) string {
	t.Helper()

	tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.tmp")
	require.NoError(t, err)

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)

	err = tmpFile.Close()
	require.NoError(t, err)

	return tmpFile.Name()
}

// InvalidPath returns a path that would cause path traversal issues.
func InvalidPath() string {
	return "../../../etc/passwd"
}

// ValidYAMLConfig returns a valid YAML configuration for testing.
func ValidYAMLConfig() string {
	return `
prefix:
  template: "[{{.Timestamp}}] [{{.Level}}] "
  timestamp:
    format: "%Y-%m-%d %H:%M:%S"
    utc: false
  colors:
    enabled: false
  user:
    enabled: true
    format: "username"
  pid:
    enabled: true
    format: "decimal"

output:
  format: "text"
  buffer: "line"

log_level:
  default_stdout: "INFO"
  default_stderr: "ERROR"
  detection:
    enabled: true
    keywords:
      error: ["ERROR", "FATAL"]
      warn: ["WARN", "WARNING"]
      debug: ["DEBUG"]
      info: ["INFO"]
`
}

// InvalidYAMLConfig returns invalid YAML for testing error cases.
func InvalidYAMLConfig() string {
	return `
prefix:
  template: "[{{.Timestamp}}] [{{.Level}}] "
  timestamp:
    format: "%Y-%m-%d %H:%M:%S"
    utc: invalid_boolean
`
}

// MinimalYAMLConfig returns a minimal YAML configuration.
func MinimalYAMLConfig() string {
	return `
prefix:
  template: "{{.Timestamp}} "
output:
  format: "text"
log_level:
  default_stdout: "INFO"
  default_stderr: "ERROR"
`
}

// MockWriter provides a mock io.Writer for testing.
type MockWriter struct {
	mutex sync.Mutex
	lines []string
}

// Write implements io.Writer interface.
func (w *MockWriter) Write(p []byte) (int, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	data := string(p)
	w.lines = append(w.lines, data)
	return len(p), nil
}

// GetLines returns all written lines.
func (w *MockWriter) GetLines() []string {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	result := make([]string, len(w.lines))
	copy(result, w.lines)
	return result
}

// Reset clears all written lines.
func (w *MockWriter) Reset() {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	w.lines = nil
}

// FailingWriter provides a writer that fails after a certain number of writes.
type FailingWriter struct {
	FailAfter int
	writeCount int
	mutex     sync.Mutex
}

// Write implements io.Writer interface and fails after FailAfter writes.
func (w *FailingWriter) Write(p []byte) (int, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.writeCount++
	if w.writeCount > w.FailAfter {
		return 0, ErrMockWriteFailure
	}
	return len(p), nil
}

// SlowReader provides a reader that introduces delays between reads.
type SlowReader struct {
	Content string
	Delay   time.Duration
	pos     int
}

// Read implements io.Reader interface with artificial delays.
func (r *SlowReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.Content) {
		return 0, io.EOF
	}

	// Introduce delay
	if r.Delay > 0 {
		time.Sleep(r.Delay)
	}

	// Read one byte at a time to make it really slow
	if len(p) > 0 {
		p[0] = r.Content[r.pos]
		r.pos++
		return 1, nil
	}

	return 0, nil
}