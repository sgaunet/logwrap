package config

import (
	"testing"

	"github.com/sgaunet/logwrap/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate_Success(t *testing.T) {
	t.Parallel()

	// Test with valid default configuration
	cfg := getDefaultConfig()
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_ValidatePrefix_EmptyTemplate(t *testing.T) {
	t.Parallel()

	cfg := getDefaultConfig()
	cfg.Prefix.Template = ""

	err := cfg.Validate()
	assert.Error(t, err)
	assert.ErrorIs(t, err, errors.ErrTemplateEmpty)
	assert.Contains(t, err.Error(), "prefix configuration error")
}

func TestConfig_ValidateTimestamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		format       string
		expectError  bool
		expectedErr  error
		errorMessage string
	}{
		{
			name:        "valid RFC3339-like strftime format",
			format:      "%Y-%m-%dT%H:%M:%S%z",
			expectError: false,
		},
		{
			name:        "valid custom strftime format",
			format:      "%Y-%m-%d %H:%M:%S",
			expectError: false,
		},
		{
			name:         "empty format",
			format:       "",
			expectError:  true,
			expectedErr:  errors.ErrTimestampFormatEmpty,
			errorMessage: "timestamp format cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := getDefaultConfig()
			cfg.Prefix.Timestamp.Format = tt.format

			err := cfg.Validate()

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.ErrorIs(t, err, tt.expectedErr)
				}
				assert.Contains(t, err.Error(), tt.errorMessage)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ValidateColors(t *testing.T) {
	t.Parallel()

	validColors := []string{"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white", "none", ""}
	invalidColors := []string{"purple", "orange", "invalid", "123"}

	tests := []struct {
		name        string
		colorField  string
		colors      []string
		expectError bool
	}{
		{
			name:       "valid info colors",
			colorField: "info",
			colors:     validColors,
		},
		{
			name:       "valid error colors",
			colorField: "error",
			colors:     validColors,
		},
		{
			name:       "valid timestamp colors",
			colorField: "timestamp",
			colors:     validColors,
		},
		{
			name:        "invalid info colors",
			colorField:  "info",
			colors:      invalidColors,
			expectError: true,
		},
		{
			name:        "invalid error colors",
			colorField:  "error",
			colors:      invalidColors,
			expectError: true,
		},
		{
			name:        "invalid timestamp colors",
			colorField:  "timestamp",
			colors:      invalidColors,
			expectError: true,
		},
	}

	for _, tt := range tests {
		for _, color := range tt.colors {
			testName := tt.name + "_" + color
			if color == "" {
				testName = tt.name + "_empty"
			}

			t.Run(testName, func(t *testing.T) {
				t.Parallel()

				cfg := getDefaultConfig()

				switch tt.colorField {
				case "info":
					cfg.Prefix.Colors.Info = color
				case "error":
					cfg.Prefix.Colors.Error = color
				case "timestamp":
					cfg.Prefix.Colors.Timestamp = color
				}

				err := cfg.Validate()

				if tt.expectError {
					assert.Error(t, err)
					assert.ErrorIs(t, err, errors.ErrInvalidColor)
					assert.Contains(t, err.Error(), "invalid color")
				} else {
					assert.NoError(t, err)
				}
			})
		}
	}
}

func TestConfig_ValidateUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		format      string
		expectError bool
	}{
		{
			name:   "valid username format",
			format: "username",
		},
		{
			name:   "valid uid format",
			format: "uid",
		},
		{
			name:   "valid full format",
			format: "full",
		},
		{
			name:        "invalid format",
			format:      "invalid",
			expectError: true,
		},
		{
			name:        "empty format",
			format:      "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := getDefaultConfig()
			cfg.Prefix.User.Format = tt.format

			err := cfg.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, errors.ErrInvalidUserFormat)
				assert.Contains(t, err.Error(), "invalid user format")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ValidatePID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		format      string
		expectError bool
	}{
		{
			name:   "valid decimal format",
			format: "decimal",
		},
		{
			name:   "valid hex format",
			format: "hex",
		},
		{
			name:        "invalid format",
			format:      "invalid",
			expectError: true,
		},
		{
			name:        "empty format",
			format:      "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := getDefaultConfig()
			cfg.Prefix.PID.Format = tt.format

			err := cfg.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, errors.ErrInvalidPIDFormat)
				assert.Contains(t, err.Error(), "invalid PID format")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ValidateOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		format      string
		buffer      string
		expectError bool
		expectedErr error
	}{
		{
			name:   "valid text format with line buffer",
			format: "text",
			buffer: "line",
		},
		{
			name:   "valid json format with none buffer",
			format: "json",
			buffer: "none",
		},
		{
			name:   "valid structured format with full buffer",
			format: "structured",
			buffer: "full",
		},
		{
			name:        "invalid format",
			format:      "invalid",
			buffer:      "line",
			expectError: true,
			expectedErr: errors.ErrInvalidOutputFormat,
		},
		{
			name:        "invalid buffer",
			format:      "text",
			buffer:      "invalid",
			expectError: true,
			expectedErr: errors.ErrInvalidBufferMode,
		},
		{
			name:        "both invalid",
			format:      "invalid",
			buffer:      "invalid",
			expectError: true,
			expectedErr: errors.ErrInvalidOutputFormat, // First error encountered
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := getDefaultConfig()
			cfg.Output.Format = tt.format
			cfg.Output.Buffer = tt.buffer

			err := cfg.Validate()

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.ErrorIs(t, err, tt.expectedErr)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ValidateLogLevel(t *testing.T) {
	t.Parallel()

	validLevels := []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL"}
	invalidLevels := []string{"INVALID", "VERBOSE", ""}

	tests := []struct {
		name           string
		stdoutLevel    string
		stderrLevel    string
		detectionLevel string
		keywords       []string
		expectError    bool
		expectedErr    error
	}{
		{
			name:        "valid levels",
			stdoutLevel: "INFO",
			stderrLevel: "ERROR",
		},
		{
			name:        "all valid levels",
			stdoutLevel: "DEBUG",
			stderrLevel: "WARN",
		},
		{
			name:        "invalid stdout level",
			stdoutLevel: "INVALID",
			stderrLevel: "ERROR",
			expectError: true,
			expectedErr: errors.ErrInvalidStdoutLogLevel,
		},
		{
			name:        "invalid stderr level",
			stdoutLevel: "INFO",
			stderrLevel: "INVALID",
			expectError: true,
			expectedErr: errors.ErrInvalidStderrLogLevel,
		},
		{
			name:           "invalid detection level",
			stdoutLevel:    "INFO",
			stderrLevel:    "ERROR",
			detectionLevel: "invalid",
			keywords:       []string{"ERROR"},
			expectError:    true,
			expectedErr:    errors.ErrInvalidLogLevel,
		},
		{
			name:           "empty keywords",
			stdoutLevel:    "INFO",
			stderrLevel:    "ERROR",
			detectionLevel: "error",
			keywords:       []string{},
			expectError:    true,
			expectedErr:    errors.ErrNoDetectionKeywords,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := getDefaultConfig()
			cfg.LogLevel.DefaultStdout = tt.stdoutLevel
			cfg.LogLevel.DefaultStderr = tt.stderrLevel

			if tt.detectionLevel != "" {
				cfg.LogLevel.Detection.Keywords = map[string][]string{
					tt.detectionLevel: tt.keywords,
				}
			}

			err := cfg.Validate()

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.ErrorIs(t, err, tt.expectedErr)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}

	// Test all valid levels work for stdout and stderr
	for _, level := range validLevels {
		t.Run("valid_level_"+level, func(t *testing.T) {
			t.Parallel()

			cfg := getDefaultConfig()
			cfg.LogLevel.DefaultStdout = level
			cfg.LogLevel.DefaultStderr = level

			err := cfg.Validate()
			assert.NoError(t, err)
		})
	}

	// Test all invalid levels fail for stdout and stderr
	for _, level := range invalidLevels {
		t.Run("invalid_stdout_level_"+level, func(t *testing.T) {
			t.Parallel()

			cfg := getDefaultConfig()
			cfg.LogLevel.DefaultStdout = level

			err := cfg.Validate()
			assert.Error(t, err)
			assert.ErrorIs(t, err, errors.ErrInvalidStdoutLogLevel)
		})

		t.Run("invalid_stderr_level_"+level, func(t *testing.T) {
			t.Parallel()

			cfg := getDefaultConfig()
			cfg.LogLevel.DefaultStderr = level

			err := cfg.Validate()
			assert.Error(t, err)
			assert.ErrorIs(t, err, errors.ErrInvalidStderrLogLevel)
		})
	}
}

func TestIsValidLogLevel(t *testing.T) {
	t.Parallel()

	validLevels := []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

	tests := []struct {
		name     string
		level    string
		expected bool
	}{
		// Valid levels (case insensitive)
		{"valid TRACE", "TRACE", true},
		{"valid trace", "trace", true},
		{"valid DEBUG", "DEBUG", true},
		{"valid debug", "debug", true},
		{"valid INFO", "INFO", true},
		{"valid info", "info", true},
		{"valid WARN", "WARN", true},
		{"valid warn", "warn", true},
		{"valid ERROR", "ERROR", true},
		{"valid error", "error", true},
		{"valid FATAL", "FATAL", true},
		{"valid fatal", "fatal", true},

		// Invalid levels
		{"invalid VERBOSE", "VERBOSE", false},
		{"invalid empty", "", false},
		{"invalid mixed case", "InFo", false},
		{"invalid number", "123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := isValidLogLevel(tt.level, validLevels)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetValidColorsString(t *testing.T) {
	t.Parallel()

	result := getValidColorsString()
	assert.NotEmpty(t, result)

	// Check that all expected colors are present
	expectedColors := []string{"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white", "none"}
	for _, color := range expectedColors {
		assert.Contains(t, result, color)
	}
}

func TestConfig_ValidateIntegration(t *testing.T) {
	t.Parallel()

	// Test a completely invalid configuration
	cfg := &Config{
		Prefix: PrefixConfig{
			Template: "", // Invalid: empty
			Timestamp: TimestampConfig{
				Format: "", // Invalid: empty
			},
			Colors: ColorsConfig{
				Info:  "purple", // Invalid: not a valid color
				Error: "orange", // Invalid: not a valid color
			},
			User: UserConfig{
				Format: "invalid", // Invalid: not username/uid/full
			},
			PID: PIDConfig{
				Format: "invalid", // Invalid: not decimal/hex
			},
		},
		Output: OutputConfig{
			Format: "invalid", // Invalid: not text/json/structured
			Buffer: "invalid", // Invalid: not line/none/full
		},
		LogLevel: LogLevelConfig{
			DefaultStdout: "INVALID", // Invalid: not a valid level
			DefaultStderr: "INVALID", // Invalid: not a valid level
			Detection: DetectionConfig{
				Keywords: map[string][]string{
					"invalid": {"ERROR"}, // Invalid: level name not valid
					"error":   {},         // Invalid: empty keywords
				},
			},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)

	// Should fail on the first error (template empty)
	assert.ErrorIs(t, err, errors.ErrTemplateEmpty)
}