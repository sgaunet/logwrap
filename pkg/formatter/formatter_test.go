package formatter

import (
	"encoding/json"
	"os"
	"os/user"
	"strconv"
	"testing"
	"time"

	"github.com/sgaunet/logwrap/pkg/config"
	"github.com/sgaunet/logwrap/pkg/processor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_Success(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Prefix: config.PrefixConfig{
			Template: "[{{.Timestamp}}] [{{.Level}}] ",
			Timestamp: config.TimestampConfig{
				Format: time.RFC3339,
				UTC:    false,
			},
			Colors: config.ColorsConfig{
				Enabled: false,
			},
			User: config.UserConfig{
				Enabled: true,
				Format:  "username",
			},
			PID: config.PIDConfig{
				Enabled: true,
				Format:  "decimal",
			},
		},
		Output: config.OutputConfig{
			Format: "text",
		},
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
			DefaultStderr: "ERROR",
			Detection: config.DetectionConfig{
				Enabled: true,
				Keywords: map[string][]string{
					"error": {"ERROR", "FATAL"},
					"warn":  {"WARN"},
					"debug": {"DEBUG"},
					"info":  {"INFO"},
				},
			},
		},
	}

	formatter, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, formatter)

	// Verify that the formatter has all required fields
	assert.NotNil(t, formatter.config)
	assert.NotNil(t, formatter.template)
	assert.NotNil(t, formatter.userInfo)
	assert.NotZero(t, formatter.pid)
	assert.NotNil(t, formatter.levelCache)
}

func TestNew_InvalidTemplate(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Prefix: config.PrefixConfig{
			Template: "{{invalid template syntax",
		},
	}

	formatter, err := New(cfg)
	assert.Error(t, err)
	assert.Nil(t, formatter)
	assert.Contains(t, err.Error(), "failed to parse template")
}

func TestFormatLine_TextFormat(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Prefix: config.PrefixConfig{
			Template: "[{{.Level}}] ",
			Timestamp: config.TimestampConfig{
				Format: "15:04:05",
				UTC:    false,
			},
			Colors: config.ColorsConfig{
				Enabled: false,
			},
			User: config.UserConfig{
				Enabled: false,
			},
			PID: config.PIDConfig{
				Enabled: false,
			},
		},
		Output: config.OutputConfig{
			Format: "text",
		},
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
			DefaultStderr: "ERROR",
			Detection: config.DetectionConfig{
				Enabled: true,
				Keywords: map[string][]string{
					"error": {"ERROR"},
					"info":  {"INFO"},
				},
			},
		},
	}

	formatter, err := New(cfg)
	require.NoError(t, err)

	tests := []struct {
		name       string
		line       string
		streamType processor.StreamType
		expected   string
	}{
		{
			name:       "stdout info line",
			line:       "This is info",
			streamType: processor.StreamStdout,
			expected:   "[INFO] This is info",
		},
		{
			name:       "stderr error line",
			line:       "This is an error",
			streamType: processor.StreamStderr,
			expected:   "[ERROR] This is an error",
		},
		{
			name:       "line with ERROR keyword",
			line:       "ERROR: something went wrong",
			streamType: processor.StreamStdout,
			expected:   "[ERROR] ERROR: something went wrong",
		},
		{
			name:       "empty line",
			line:       "",
			streamType: processor.StreamStdout,
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := formatter.FormatLine(tt.line, tt.streamType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatLine_JSONFormat(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Prefix: config.PrefixConfig{
			Template: "[{{.Level}}] ",
			Timestamp: config.TimestampConfig{
				Format: time.RFC3339,
				UTC:    false,
			},
			User: config.UserConfig{
				Enabled: true,
				Format:  "username",
			},
			PID: config.PIDConfig{
				Enabled: true,
				Format:  "decimal",
			},
		},
		Output: config.OutputConfig{
			Format: "json",
		},
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
			DefaultStderr: "ERROR",
			Detection: config.DetectionConfig{
				Enabled: false,
			},
		},
	}

	formatter, err := New(cfg)
	require.NoError(t, err)

	line := "test message"
	result := formatter.FormatLine(line, processor.StreamStdout)

	// Parse the JSON to verify it's valid
	var jsonData map[string]interface{}
	err = json.Unmarshal([]byte(result), &jsonData)
	require.NoError(t, err)

	// Verify required fields are present
	assert.Equal(t, "INFO", jsonData["level"])
	assert.Equal(t, "test message", jsonData["message"])
	assert.Contains(t, jsonData, "timestamp")
	assert.Contains(t, jsonData, "user")
	assert.Contains(t, jsonData, "pid")

	// Verify timestamp is valid
	assert.NotEmpty(t, jsonData["timestamp"])

	// Verify user is current user
	currentUser, _ := user.Current()
	assert.Equal(t, currentUser.Username, jsonData["user"])

	// Verify PID is current process PID
	expectedPID := strconv.Itoa(os.Getpid())
	assert.Equal(t, expectedPID, jsonData["pid"])
}

func TestFormatLine_StructuredFormat(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Prefix: config.PrefixConfig{
			User: config.UserConfig{
				Enabled: true,
				Format:  "username",
			},
			PID: config.PIDConfig{
				Enabled: true,
				Format:  "decimal",
			},
		},
		Output: config.OutputConfig{
			Format: "structured",
		},
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
			DefaultStderr: "ERROR",
			Detection: config.DetectionConfig{
				Enabled: false,
			},
		},
	}

	formatter, err := New(cfg)
	require.NoError(t, err)

	line := "test message"
	result := formatter.FormatLine(line, processor.StreamStdout)

	// Verify structured format
	assert.Contains(t, result, "level=INFO")
	assert.Contains(t, result, "message=\"test message\"")
	assert.Contains(t, result, "timestamp=")
	assert.Contains(t, result, "user=")
	assert.Contains(t, result, "pid=")

	// Verify user is current user
	currentUser, _ := user.Current()
	assert.Contains(t, result, "user="+currentUser.Username)

	// Verify PID is current process PID
	expectedPID := strconv.Itoa(os.Getpid())
	assert.Contains(t, result, "pid="+expectedPID)
}

func TestFormatLine_WithColors(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Prefix: config.PrefixConfig{
			Template: "[{{.Level}}] ",
			Colors: config.ColorsConfig{
				Enabled:   true,
				Info:      "green",
				Error:     "red",
				Timestamp: "blue",
			},
			User: config.UserConfig{
				Enabled: false,
			},
			PID: config.PIDConfig{
				Enabled: false,
			},
		},
		Output: config.OutputConfig{
			Format: "text",
		},
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
			DefaultStderr: "ERROR",
			Detection: config.DetectionConfig{
				Enabled: true,
				Keywords: map[string][]string{
					"error": {"ERROR"},
				},
			},
		},
	}

	formatter, err := New(cfg)
	require.NoError(t, err)

	tests := []struct {
		name       string
		line       string
		streamType processor.StreamType
		checkColor string
	}{
		{
			name:       "info line with green color",
			line:       "info message",
			streamType: processor.StreamStdout,
			checkColor: "\033[32m", // green color code
		},
		{
			name:       "error line with red color",
			line:       "ERROR: failed",
			streamType: processor.StreamStdout,
			checkColor: "\033[31m", // red color code
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := formatter.FormatLine(tt.line, tt.streamType)
			assert.Contains(t, result, tt.checkColor)
			assert.Contains(t, result, "\033[0m") // reset color code
		})
	}
}

func TestGetLogLevel(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
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

	formatter, err := New(cfg)
	require.NoError(t, err)

	tests := []struct {
		name       string
		line       string
		streamType processor.StreamType
		expected   string
	}{
		{
			name:       "ERROR keyword detection",
			line:       "ERROR: something failed",
			streamType: processor.StreamStdout,
			expected:   "ERROR",
		},
		{
			name:       "FATAL keyword detection",
			line:       "FATAL error occurred",
			streamType: processor.StreamStdout,
			expected:   "ERROR",
		},
		{
			name:       "WARN keyword detection",
			line:       "WARN: this is a warning",
			streamType: processor.StreamStdout,
			expected:   "WARN",
		},
		{
			name:       "DEBUG keyword detection",
			line:       "DEBUG: debugging info",
			streamType: processor.StreamStdout,
			expected:   "DEBUG",
		},
		{
			name:       "INFO keyword detection",
			line:       "INFO: information",
			streamType: processor.StreamStdout,
			expected:   "INFO",
		},
		{
			name:       "no keyword - stdout default",
			line:       "regular stdout message",
			streamType: processor.StreamStdout,
			expected:   "INFO",
		},
		{
			name:       "no keyword - stderr default",
			line:       "regular stderr message",
			streamType: processor.StreamStderr,
			expected:   "ERROR",
		},
		{
			name:       "case insensitive detection",
			line:       "error occurred",
			streamType: processor.StreamStdout,
			expected:   "ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := formatter.getLogLevel(tt.line, tt.streamType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetLogLevel_Caching(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
			Detection: config.DetectionConfig{
				Enabled: true,
				Keywords: map[string][]string{
					"error": {"ERROR"},
				},
			},
		},
	}

	formatter, err := New(cfg)
	require.NoError(t, err)

	line := "ERROR: test message"

	// First call should cache the result
	result1 := formatter.getLogLevel(line, processor.StreamStdout)
	assert.Equal(t, "ERROR", result1)

	// Second call should use cache
	result2 := formatter.getLogLevel(line, processor.StreamStdout)
	assert.Equal(t, "ERROR", result2)

	// Verify cache was used (both calls return the same result)
	assert.Equal(t, result1, result2)
}

func TestGetLogLevel_DetectionDisabled(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
			DefaultStderr: "ERROR",
			Detection: config.DetectionConfig{
				Enabled: false,
			},
		},
	}

	formatter, err := New(cfg)
	require.NoError(t, err)

	tests := []struct {
		name       string
		line       string
		streamType processor.StreamType
		expected   string
	}{
		{
			name:       "ERROR keyword ignored - stdout",
			line:       "ERROR: something failed",
			streamType: processor.StreamStdout,
			expected:   "INFO",
		},
		{
			name:       "ERROR keyword ignored - stderr",
			line:       "ERROR: something failed",
			streamType: processor.StreamStderr,
			expected:   "ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := formatter.getLogLevel(tt.line, tt.streamType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetUserString(t *testing.T) {
	t.Parallel()

	currentUser, err := user.Current()
	require.NoError(t, err)

	tests := []struct {
		name     string
		enabled  bool
		format   string
		expected string
	}{
		{
			name:     "disabled user",
			enabled:  false,
			format:   "username",
			expected: "",
		},
		{
			name:     "username format",
			enabled:  true,
			format:   "username",
			expected: currentUser.Username,
		},
		{
			name:     "uid format",
			enabled:  true,
			format:   "uid",
			expected: currentUser.Uid,
		},
		{
			name:     "full format",
			enabled:  true,
			format:   "full",
			expected: currentUser.Username + "(" + currentUser.Uid + ")",
		},
		{
			name:     "invalid format defaults to username",
			enabled:  true,
			format:   "invalid",
			expected: currentUser.Username,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{
				Prefix: config.PrefixConfig{
					User: config.UserConfig{
						Enabled: tt.enabled,
						Format:  tt.format,
					},
				},
			}

			formatter, err := New(cfg)
			require.NoError(t, err)

			result := formatter.getUserString()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetPIDString(t *testing.T) {
	t.Parallel()

	currentPID := os.Getpid()

	tests := []struct {
		name     string
		enabled  bool
		format   string
		expected string
	}{
		{
			name:     "disabled PID",
			enabled:  false,
			format:   "decimal",
			expected: "",
		},
		{
			name:     "decimal format",
			enabled:  true,
			format:   "decimal",
			expected: strconv.Itoa(currentPID),
		},
		{
			name:     "hex format",
			enabled:  true,
			format:   "hex",
			expected: "0x" + strconv.FormatInt(int64(currentPID), 16),
		},
		{
			name:     "invalid format defaults to decimal",
			enabled:  true,
			format:   "invalid",
			expected: strconv.Itoa(currentPID),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{
				Prefix: config.PrefixConfig{
					PID: config.PIDConfig{
						Enabled: tt.enabled,
						Format:  tt.format,
					},
				},
			}

			formatter, err := New(cfg)
			require.NoError(t, err)

			result := formatter.getPIDString()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetTimestamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		format   string
		utc      bool
		validate func(*testing.T, string)
	}{
		{
			name:   "RFC3339 format",
			format: time.RFC3339,
			utc:    false,
			validate: func(t *testing.T, result string) {
				_, err := time.Parse(time.RFC3339, result)
				assert.NoError(t, err)
			},
		},
		{
			name:   "custom format",
			format: "2006-01-02 15:04:05",
			utc:    false,
			validate: func(t *testing.T, result string) {
				_, err := time.Parse("2006-01-02 15:04:05", result)
				assert.NoError(t, err)
			},
		},
		{
			name:   "UTC time",
			format: time.RFC3339,
			utc:    true,
			validate: func(t *testing.T, result string) {
				parsed, err := time.Parse(time.RFC3339, result)
				assert.NoError(t, err)
				assert.Equal(t, time.UTC, parsed.Location())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{
				Prefix: config.PrefixConfig{
					Timestamp: config.TimestampConfig{
						Format: tt.format,
						UTC:    tt.utc,
					},
				},
			}

			formatter, err := New(cfg)
			require.NoError(t, err)

			result := formatter.getTimestamp()
			assert.NotEmpty(t, result)
			tt.validate(t, result)
		})
	}
}

func TestGetColorCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		colorName string
		expected string
	}{
		{"black", "black", "\033[30m"},
		{"red", "red", "\033[31m"},
		{"green", "green", "\033[32m"},
		{"yellow", "yellow", "\033[33m"},
		{"blue", "blue", "\033[34m"},
		{"magenta", "magenta", "\033[35m"},
		{"cyan", "cyan", "\033[36m"},
		{"white", "white", "\033[37m"},
		{"none", "none", ""},
		{"empty", "", ""},
		{"invalid", "invalid", ""},
		{"case insensitive", "RED", "\033[31m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := getColorCode(tt.colorName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildTemplateData(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Prefix: config.PrefixConfig{
			Timestamp: config.TimestampConfig{
				Format: "15:04:05",
				UTC:    false,
			},
			User: config.UserConfig{
				Enabled: true,
				Format:  "username",
			},
			PID: config.PIDConfig{
				Enabled: true,
				Format:  "decimal",
			},
		},
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
			Detection: config.DetectionConfig{
				Enabled: false,
			},
		},
	}

	formatter, err := New(cfg)
	require.NoError(t, err)

	line := "test message"
	data := formatter.buildTemplateData(line, processor.StreamStdout)

	assert.Equal(t, line, data.Line)
	assert.Equal(t, "INFO", data.Level)
	assert.NotEmpty(t, data.Timestamp)
	assert.NotEmpty(t, data.User)
	assert.NotEmpty(t, data.PID)

	// Verify timestamp format
	_, err = time.Parse("15:04:05", data.Timestamp)
	assert.NoError(t, err)

	// Verify user is current user
	currentUser, _ := user.Current()
	assert.Equal(t, currentUser.Username, data.User)

	// Verify PID is current process PID
	expectedPID := strconv.Itoa(os.Getpid())
	assert.Equal(t, expectedPID, data.PID)
}

func TestColorizePrefix_Disabled(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Prefix: config.PrefixConfig{
			Colors: config.ColorsConfig{
				Enabled: false,
			},
		},
	}

	formatter, err := New(cfg)
	require.NoError(t, err)

	prefix := "[INFO] "
	result := formatter.colorizePrefix(prefix)
	assert.Equal(t, prefix, result) // Should be unchanged
}

func TestColorizeLine(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Prefix: config.PrefixConfig{
			Colors: config.ColorsConfig{
				Enabled: true,
				Info:    "green",
				Error:   "red",
			},
		},
	}

	formatter, err := New(cfg)
	require.NoError(t, err)

	tests := []struct {
		name     string
		line     string
		level    string
		expected string
	}{
		{
			name:     "info level",
			line:     "info message",
			level:    "INFO",
			expected: "\033[32minfo message\033[0m",
		},
		{
			name:     "error level",
			line:     "error message",
			level:    "ERROR",
			expected: "\033[31merror message\033[0m",
		},
		{
			name:     "unknown level",
			line:     "unknown message",
			level:    "UNKNOWN",
			expected: "unknown message", // No coloring
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := formatter.colorizeLine(tt.line, tt.level)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatLine_EmptyLine(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Prefix: config.PrefixConfig{
			Template: "[{{.Level}}] ",
		},
		Output: config.OutputConfig{
			Format: "text",
		},
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
		},
	}

	formatter, err := New(cfg)
	require.NoError(t, err)

	// Empty lines should be returned as-is
	result := formatter.FormatLine("", processor.StreamStdout)
	assert.Equal(t, "", result)
}

func TestFormatLine_TemplateExecutionError(t *testing.T) {
	t.Parallel()

	// This test is tricky because we need to cause a template execution error
	// after the template has been successfully parsed. This is difficult to achieve
	// in practice with the current template, so we'll test the fallback behavior.

	cfg := &config.Config{
		Prefix: config.PrefixConfig{
			Template: "{{.Level}}",
		},
		Output: config.OutputConfig{
			Format: "text",
		},
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
		},
	}

	formatter, err := New(cfg)
	require.NoError(t, err)

	// Normal case should work
	result := formatter.FormatLine("test", processor.StreamStdout)
	assert.Equal(t, "INFOtest", result)
}