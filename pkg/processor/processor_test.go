package processor_test

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sgaunet/logwrap/internal/testutils"
	"github.com/sgaunet/logwrap/pkg/errors"
	"github.com/sgaunet/logwrap/pkg/processor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFormatter struct {
	formatFunc func(line string, streamType processor.StreamType) string
}

func (m *mockFormatter) FormatLine(line string, streamType processor.StreamType) string {
	if m.formatFunc != nil {
		return m.formatFunc(line, streamType)
	}
	return "[" + streamType.String() + "] " + line
}

func TestStreamType_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		streamType processor.StreamType
		expected   string
	}{
		{
			name:       "stdout stream",
			streamType: processor.StreamStdout,
			expected:   "stdout",
		},
		{
			name:       "stderr stream",
			streamType: processor.StreamStderr,
			expected:   "stderr",
		},
		{
			name:       "unknown stream",
			streamType: processor.StreamType(999),
			expected:   "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.streamType.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	output := &testutils.MockWriter{}
	formatter := &mockFormatter{}

	t.Run("basic creation", func(t *testing.T) {
		t.Parallel()

		p := processor.New(formatter, output)
		assert.NotNil(t, p)
	})

	t.Run("with context option", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		p := processor.New(formatter, output, processor.WithContext(ctx))
		assert.NotNil(t, p)
	})

	t.Run("with multiple options", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		p := processor.New(formatter, output, processor.WithContext(ctx))
		assert.NotNil(t, p)
	})
}

func TestProcessor_ProcessStreams_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		stdoutContent  string
		stderrContent  string
		expectedOutput []string
	}{
		{
			name:          "empty streams",
			stdoutContent: "",
			stderrContent: "",
			expectedOutput: []string{},
		},
		{
			name:          "stdout only",
			stdoutContent: "info message\nsecond info",
			stderrContent: "",
			expectedOutput: []string{
				"[stdout] info message",
				"[stdout] second info",
			},
		},
		{
			name:          "stderr only",
			stdoutContent: "",
			stderrContent: "error message\nsecond error",
			expectedOutput: []string{
				"[stderr] error message",
				"[stderr] second error",
			},
		},
		{
			name:          "both streams",
			stdoutContent: "info message",
			stderrContent: "error message",
			expectedOutput: []string{
				"[stdout] info message",
				"[stderr] error message",
			},
		},
		{
			name:          "multiline content",
			stdoutContent: "line1\nline2\nline3",
			stderrContent: "err1\nerr2",
			expectedOutput: []string{
				"[stdout] line1",
				"[stdout] line2",
				"[stdout] line3",
				"[stderr] err1",
				"[stderr] err2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			output := &testutils.MockWriter{}
			formatter := &mockFormatter{}
			p := processor.New(formatter, output)

			stdout := strings.NewReader(tt.stdoutContent)
			stderr := strings.NewReader(tt.stderrContent)

			ctx := context.Background()
			err := p.ProcessStreams(ctx, stdout, stderr)

			assert.NoError(t, err)

			writtenLines := output.GetLines()
			if len(tt.expectedOutput) == 0 {
				assert.Empty(t, writtenLines)
			} else {
				for _, expected := range tt.expectedOutput {
					assert.Contains(t, writtenLines, expected+"\n")
				}
			}
		})
	}
}

func TestProcessor_ProcessStreams_NilReaders(t *testing.T) {
	t.Parallel()

	output := &testutils.MockWriter{}
	formatter := &mockFormatter{}
	p := processor.New(formatter, output)
	ctx := context.Background()

	tests := []struct {
		name   string
		stdout io.Reader
		stderr io.Reader
	}{
		{
			name:   "nil stdout",
			stdout: nil,
			stderr: strings.NewReader("test"),
		},
		{
			name:   "nil stderr",
			stdout: strings.NewReader("test"),
			stderr: nil,
		},
		{
			name:   "both nil",
			stdout: nil,
			stderr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := p.ProcessStreams(ctx, tt.stdout, tt.stderr)
			assert.Error(t, err)
			assert.ErrorIs(t, err, errors.ErrReadersNil)
		})
	}
}

func TestProcessor_ProcessStreams_ContextCancellation(t *testing.T) {
	t.Parallel()

	output := &testutils.MockWriter{}
	formatter := &mockFormatter{}
	p := processor.New(formatter, output)

	// Create a slow reader that will be cancelled
	slowReader := &testutils.SlowReader{
		Content: "line1\nline2\nline3\nline4\nline5",
		Delay:   50 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := p.ProcessStreams(ctx, slowReader, strings.NewReader(""))

	// Should return an error due to context cancellation
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "processing errors occurred")
}

func TestProcessor_ProcessStreams_FormatterError(t *testing.T) {
	t.Parallel()

	output := &testutils.FailingWriter{FailAfter: 1}
	formatter := &mockFormatter{}
	p := processor.New(formatter, output)

	stdout := strings.NewReader("line1\nline2")
	stderr := strings.NewReader("")

	ctx := context.Background()
	err := p.ProcessStreams(ctx, stdout, stderr)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "processing errors occurred")
}

func TestProcessor_Stop(t *testing.T) {
	t.Parallel()

	output := &testutils.MockWriter{}
	formatter := &mockFormatter{}
	p := processor.New(formatter, output)

	// Stop should not panic even when called multiple times
	p.Stop()
	p.Stop()
}

func TestProcessor_Wait_Success(t *testing.T) {
	t.Parallel()

	output := &testutils.MockWriter{}
	formatter := &mockFormatter{}
	p := processor.New(formatter, output)

	// Start processing in background
	started := make(chan struct{})
	go func() {
		close(started) // Signal that goroutine has started
		ctx := context.Background()
		stdout := strings.NewReader("test line")
		stderr := strings.NewReader("")
		_ = p.ProcessStreams(ctx, stdout, stderr)
	}()

	// Wait for goroutine to start
	<-started

	// Give a small delay to ensure ProcessStreams has called wg.Add
	time.Sleep(10 * time.Millisecond)

	// Wait should complete successfully
	err := p.Wait(1 * time.Second)
	assert.NoError(t, err)
}

func TestProcessor_Wait_Timeout(t *testing.T) {
	// Do not run in parallel since we need predictable timing

	output := &testutils.MockWriter{}
	formatter := &mockFormatter{}
	p := processor.New(formatter, output)

	// Create a reader that will block for a long time
	slowReader := &testutils.SlowReader{
		Content: "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10",
		Delay:   50 * time.Millisecond, // This will make processing very slow
	}

	// Start processing in background
	processing := make(chan struct{})
	go func() {
		close(processing) // Signal that processing has started
		ctx := context.Background()
		_ = p.ProcessStreams(ctx, slowReader, strings.NewReader(""))
	}()

	// Wait for processing to start
	<-processing

	// Give it a moment to actually start processing
	time.Sleep(10 * time.Millisecond)

	// Wait with short timeout should timeout
	err := p.Wait(30 * time.Millisecond)
	assert.Error(t, err)
	assert.ErrorIs(t, err, errors.ErrProcessorTimeout)
	assert.Contains(t, err.Error(), "30ms")

	// Clean up - stop the processor to avoid goroutine leaks
	p.Stop()

	// Give time for cleanup
	time.Sleep(100 * time.Millisecond)
}

func TestProcessor_GetErrors(t *testing.T) {
	t.Parallel()

	output := &testutils.FailingWriter{FailAfter: 1}
	formatter := &mockFormatter{}
	p := processor.New(formatter, output)

	stdout := strings.NewReader("line1\nline2")
	stderr := strings.NewReader("error1")

	ctx := context.Background()
	err := p.ProcessStreams(ctx, stdout, stderr)

	assert.Error(t, err)

	// Get errors should return the processing errors
	processingErrors := p.GetErrors()
	assert.NotEmpty(t, processingErrors)

	// Should be able to call GetErrors multiple times
	processingErrors2 := p.GetErrors()
	assert.Equal(t, len(processingErrors), len(processingErrors2))
}

func TestProcessor_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	output := &testutils.MockWriter{}
	formatter := &mockFormatter{}
	p := processor.New(formatter, output)

	const numGoroutines = 10
	var wg sync.WaitGroup

	// Test concurrent access to GetErrors
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			_ = p.GetErrors()
		}()
	}

	// Start some processing
	go func() {
		ctx := context.Background()
		stdout := strings.NewReader("test line")
		stderr := strings.NewReader("")
		_ = p.ProcessStreams(ctx, stdout, stderr)
	}()

	wg.Wait()
}

func TestProcessor_LargeInput(t *testing.T) {
	t.Parallel()

	output := &testutils.MockWriter{}
	formatter := &mockFormatter{}
	p := processor.New(formatter, output)

	// Create large input
	const lineCount = 1000
	var lines []string
	for i := 0; i < lineCount; i++ {
		lines = append(lines, "line "+string(rune('0'+i%10)))
	}
	largeContent := strings.Join(lines, "\n")

	stdout := strings.NewReader(largeContent)
	stderr := strings.NewReader("")

	ctx := context.Background()
	err := p.ProcessStreams(ctx, stdout, stderr)

	assert.NoError(t, err)

	writtenLines := output.GetLines()
	assert.Len(t, writtenLines, lineCount)
}

func TestProcessor_CustomFormatter(t *testing.T) {
	t.Parallel()

	output := &testutils.MockWriter{}

	// Custom formatter that adds line numbers
	lineCounter := 0
	var mu sync.Mutex
	formatter := &mockFormatter{
		formatFunc: func(line string, streamType processor.StreamType) string {
			mu.Lock()
			lineCounter++
			counter := lineCounter
			mu.Unlock()
			return string(rune('0'+counter)) + ": [" + streamType.String() + "] " + line
		},
	}

	p := processor.New(formatter, output)

	stdout := strings.NewReader("first\nsecond")
	stderr := strings.NewReader("error")

	ctx := context.Background()
	err := p.ProcessStreams(ctx, stdout, stderr)

	assert.NoError(t, err)

	writtenLines := output.GetLines()
	require.GreaterOrEqual(t, len(writtenLines), 2)

	// Check that custom formatting was applied
	found := false
	for _, line := range writtenLines {
		if strings.Contains(line, ": [stdout]") || strings.Contains(line, ": [stderr]") {
			found = true
			break
		}
	}
	assert.True(t, found, "Custom formatter should have been applied")
}

func TestProcessor_EmptyLinesHandling(t *testing.T) {
	t.Parallel()

	output := &testutils.MockWriter{}
	formatter := &mockFormatter{}
	p := processor.New(formatter, output)

	// Content with empty lines
	stdout := strings.NewReader("line1\n\nline3\n\n")
	stderr := strings.NewReader("")

	ctx := context.Background()
	err := p.ProcessStreams(ctx, stdout, stderr)

	assert.NoError(t, err)

	writtenLines := output.GetLines()

	// Should process all lines including empty ones
	expectedLines := []string{
		"[stdout] line1\n",
		"[stdout] \n",
		"[stdout] line3\n",
		"[stdout] \n",
	}

	assert.Len(t, writtenLines, len(expectedLines))
	for _, expected := range expectedLines {
		assert.Contains(t, writtenLines, expected)
	}
}

func TestProcessor_Integration(t *testing.T) {
	t.Parallel()

	// Integration test with realistic scenarios
	output := &testutils.MockWriter{}
	formatter := &mockFormatter{
		formatFunc: func(line string, streamType processor.StreamType) string {
			timestamp := "2024-01-01T12:00:00Z"
			return "[" + timestamp + "] [" + streamType.String() + "] " + line
		},
	}

	p := processor.New(formatter, output)

	// Simulate typical application output
	stdoutContent := `Application starting...
Loading configuration from config.yaml
Server listening on port 8080
Processing request: GET /health
Request completed successfully`

	stderrContent := `WARN: Deprecated configuration option detected
ERROR: Failed to connect to database, retrying...
ERROR: Maximum retry attempts reached`

	stdout := strings.NewReader(stdoutContent)
	stderr := strings.NewReader(stderrContent)

	ctx := context.Background()
	err := p.ProcessStreams(ctx, stdout, stderr)

	assert.NoError(t, err)

	writtenLines := output.GetLines()
	assert.GreaterOrEqual(t, len(writtenLines), 8) // 5 stdout + 3 stderr lines

	// Verify timestamp formatting was applied
	timestampFound := false
	for _, line := range writtenLines {
		if strings.Contains(line, "2024-01-01T12:00:00Z") {
			timestampFound = true
			break
		}
	}
	assert.True(t, timestampFound, "Timestamp formatting should be applied")

	// Verify both stream types are present
	stdoutFound := false
	stderrFound := false
	for _, line := range writtenLines {
		if strings.Contains(line, "[stdout]") {
			stdoutFound = true
		}
		if strings.Contains(line, "[stderr]") {
			stderrFound = true
		}
	}
	assert.True(t, stdoutFound, "Stdout lines should be present")
	assert.True(t, stderrFound, "Stderr lines should be present")
}