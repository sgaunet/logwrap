package formatter

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/sgaunet/logwrap/pkg/config"
	"github.com/sgaunet/logwrap/pkg/processor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestFormatter creates a formatter with a simple config for edge case testing.
func newTestFormatter(t *testing.T, outputFormat string) *DefaultFormatter {
	t.Helper()
	cfg := &config.Config{
		Prefix: config.PrefixConfig{
			Template: "[{{.Level}}] ",
			Timestamp: config.TimestampConfig{
				Format: "%H:%M:%S",
				UTC:    true,
			},
			Colors: config.ColorsConfig{Enabled: false},
			User:   config.UserConfig{Enabled: false},
			PID:    config.PIDConfig{Enabled: false},
		},
		Output: config.OutputConfig{Format: outputFormat},
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
			DefaultStderr: "ERROR",
			Detection: config.DetectionConfig{
				Enabled: true,
				Keywords: map[string][]string{
					"error": {"ERROR", "FATAL", "PANIC"},
					"warn":  {"WARN", "WARNING"},
					"debug": {"DEBUG", "TRACE"},
					"info":  {"INFO"},
				},
			},
		},
	}
	f, err := New(cfg)
	require.NoError(t, err)
	return f
}

func TestFormatLine_VeryLongLines(t *testing.T) {
	t.Parallel()

	f := newTestFormatter(t, "text")

	tests := []struct {
		name     string
		lineSize int
	}{
		{"100 bytes", 100},
		{"1KB", 1024},
		{"100KB", 100 * 1024},
		{"1MB", 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			line := strings.Repeat("x", tt.lineSize)
			result := f.FormatLine(line, processor.StreamStdout)

			assert.Contains(t, result, "[INFO] ")
			assert.True(t, len(result) > tt.lineSize,
				"formatted output should be longer than input")
		})
	}
}

func TestFormatLine_VeryLongLines_JSON(t *testing.T) {
	t.Parallel()

	f := newTestFormatter(t, "json")

	line := strings.Repeat("a", 100*1024) // 100KB
	result := f.FormatLine(line, processor.StreamStdout)

	assert.True(t, json.Valid([]byte(result)), "long line JSON should be valid")

	var data map[string]any
	err := json.Unmarshal([]byte(result), &data)
	require.NoError(t, err)
	assert.Len(t, data["message"], 100*1024)
}

func TestFormatLine_SpecialCharacters_Text(t *testing.T) {
	t.Parallel()

	f := newTestFormatter(t, "text")

	tests := []struct {
		name  string
		input string
	}{
		{"null bytes", "line with \x00 null byte"},
		{"ANSI escape codes", "line with \x1b[31m red \x1b[0m codes"},
		{"unicode emoji", "line with emoji 🚀🎉 and 中文"},
		{"tabs", "col1\tcol2\tcol3"},
		{"bell character", "line with \x07 bell"},
		{"backspace", "line with \x08 backspace"},
		{"control chars", "line with \x01\x02\x03 control"},
		{"mixed unicode", "café résumé naïve"},
		{"zero-width chars", "zero\u200Bwidth\u200Bjoiner"},
		{"right-to-left", "text \u200Frtl\u200F mark"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := f.FormatLine(tt.input, processor.StreamStdout)

			// Must not panic and must contain the prefix.
			assert.True(t, strings.HasPrefix(result, "[INFO] ") ||
				strings.HasPrefix(result, "[ERROR] ") ||
				strings.HasPrefix(result, "[WARN] ") ||
				strings.HasPrefix(result, "[DEBUG] "),
				"output should start with level prefix, got: %q", result)
			// Original content should be preserved.
			assert.Contains(t, result, tt.input)
		})
	}
}

func TestFormatLine_SpecialCharacters_JSON(t *testing.T) {
	t.Parallel()

	f := newTestFormatter(t, "json")

	tests := []struct {
		name  string
		input string
	}{
		{"null bytes", "line with \x00 null byte"},
		{"ANSI escape codes", "line with \x1b[31m red \x1b[0m codes"},
		{"bell character", "line with \x07 bell"},
		{"backspace", "line with \x08 backspace"},
		{"control chars", "line with \x01\x02\x03 control"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := f.FormatLine(tt.input, processor.StreamStdout)
			assert.True(t, json.Valid([]byte(result)),
				"JSON output with special chars must be valid, got: %s", result)

			var data map[string]any
			err := json.Unmarshal([]byte(result), &data)
			require.NoError(t, err)
			assert.Equal(t, tt.input, data["message"])
		})
	}
}

func TestFormatLine_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	f := newTestFormatter(t, "text")

	const goroutines = 100
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				line := fmt.Sprintf("ERROR: message %d-%d", id, j)
				result := f.FormatLine(line, processor.StreamStdout)
				if !strings.HasPrefix(result, "[ERROR] ") {
					t.Errorf("unexpected prefix in concurrent result: %q", result)
					return
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestFormatLine_ConcurrentAccess_JSON(t *testing.T) {
	t.Parallel()

	f := newTestFormatter(t, "json")

	const goroutines = 50
	const iterations = 500

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				line := fmt.Sprintf("WARN: message %d-%d", id, j)
				result := f.FormatLine(line, processor.StreamStdout)
				if !json.Valid([]byte(result)) {
					t.Errorf("invalid JSON in concurrent result: %s", result)
					return
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestGetLogLevel_EdgeCases(t *testing.T) {
	t.Parallel()

	f := newTestFormatter(t, "text")

	tests := []struct {
		name       string
		line       string
		streamType processor.StreamType
		expected   string
	}{
		{
			name:       "multiple keywords - highest priority wins",
			line:       "ERROR: this is also a WARN message",
			streamType: processor.StreamStdout,
			expected:   "ERROR",
		},
		{
			name:       "keyword at end of line",
			line:       "Message ends with ERROR",
			streamType: processor.StreamStdout,
			expected:   "ERROR",
		},
		{
			name:       "keyword in middle of line",
			line:       "Some ERROR occurred here",
			streamType: processor.StreamStdout,
			expected:   "ERROR",
		},
		{
			name:       "only whitespace",
			line:       "   \t   ",
			streamType: processor.StreamStdout,
			expected:   "INFO",
		},
		{
			name:       "very long line with keyword at end",
			line:       strings.Repeat("x", 10000) + "ERROR",
			streamType: processor.StreamStdout,
			expected:   "ERROR",
		},
		{
			name:       "keyword substring in word",
			line:       "INFORMATION about the system",
			streamType: processor.StreamStdout,
			expected:   "INFO",
		},
		{
			name:       "keyword in mixed case",
			line:       "an error message lowercase",
			streamType: processor.StreamStdout,
			expected:   "ERROR",
		},
		{
			name:       "WARN and INFO both present",
			line:       "INFO: received WARNING signal",
			streamType: processor.StreamStdout,
			expected:   "WARN",
		},
		{
			name:       "all levels present - error wins",
			line:       "DEBUG INFO WARN ERROR",
			streamType: processor.StreamStdout,
			expected:   "ERROR",
		},
		{
			name:       "stderr with no keyword uses default",
			line:       "just a regular message",
			streamType: processor.StreamStderr,
			expected:   "ERROR",
		},
		{
			name:       "line with only newlines and spaces",
			line:       "\n  \n  \n",
			streamType: processor.StreamStdout,
			expected:   "INFO",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := f.getLogLevel(tt.line, tt.streamType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatLine_VeryLongLines_Structured(t *testing.T) {
	t.Parallel()

	f := newTestFormatter(t, "structured")

	line := strings.Repeat("b", 100*1024) // 100KB
	result := f.FormatLine(line, processor.StreamStdout)

	assert.Contains(t, result, "level=INFO")
	assert.Contains(t, result, "message=")
	assert.True(t, len(result) > 100*1024)
}

func TestFormatLine_EmptyAndWhitespace(t *testing.T) {
	t.Parallel()

	f := newTestFormatter(t, "text")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", "[INFO] "},
		{"single space", " ", "[INFO]  "},
		{"only tabs", "\t\t", "[INFO] \t\t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := f.FormatLine(tt.input, processor.StreamStdout)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Fuzz tests for robustness - ensures no panics on arbitrary input.

func FuzzFormatLine_Text(f *testing.F) {
	cfg := &config.Config{
		Prefix: config.PrefixConfig{
			Template: "[{{.Level}}] ",
			Timestamp: config.TimestampConfig{
				Format: "%H:%M:%S",
				UTC:    true,
			},
			User: config.UserConfig{Enabled: false},
			PID:  config.PIDConfig{Enabled: false},
		},
		Output: config.OutputConfig{Format: "text"},
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
			DefaultStderr: "ERROR",
			Detection: config.DetectionConfig{
				Enabled: true,
				Keywords: map[string][]string{
					"error": {"ERROR"},
					"warn":  {"WARN"},
					"info":  {"INFO"},
				},
			},
		},
	}

	formatter, err := New(cfg)
	if err != nil {
		f.Fatalf("failed to create formatter: %v", err)
	}

	f.Add("ERROR: test message")
	f.Add("normal line")
	f.Add("")
	f.Add("line with \x00 null byte")
	f.Add("emoji 🚀 and unicode ñ")

	f.Fuzz(func(t *testing.T, line string) {
		// Must never panic.
		result := formatter.FormatLine(line, processor.StreamStdout)
		if result == "" {
			t.Error("FormatLine should never return empty string")
		}
	})
}

func FuzzFormatLine_JSON(f *testing.F) {
	cfg := &config.Config{
		Prefix: config.PrefixConfig{
			Template: "[{{.Level}}] ",
			Timestamp: config.TimestampConfig{
				Format: "%H:%M:%S",
				UTC:    true,
			},
			User: config.UserConfig{Enabled: true, Format: "username"},
			PID:  config.PIDConfig{Enabled: true, Format: "decimal"},
		},
		Output: config.OutputConfig{Format: "json"},
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
			DefaultStderr: "ERROR",
			Detection:     config.DetectionConfig{Enabled: false},
		},
	}

	formatter, err := New(cfg)
	if err != nil {
		f.Fatalf("failed to create formatter: %v", err)
	}

	f.Add("ERROR: test")
	f.Add("")
	f.Add("line with \"quotes\" and \\backslash")
	f.Add("\x00\x01\x02\x03")

	f.Fuzz(func(t *testing.T, line string) {
		result := formatter.FormatLine(line, processor.StreamStdout)
		if !json.Valid([]byte(result)) {
			t.Errorf("FormatLine JSON output must always be valid JSON, got: %s", result)
		}
	})
}
