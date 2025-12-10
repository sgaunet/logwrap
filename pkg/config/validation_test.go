package config

import (
	"testing"

	"github.com/sgaunet/logwrap/pkg/apperrors"
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
	assert.ErrorIs(t, err, apperrors.ErrTemplateEmpty)
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
			expectedErr:  apperrors.ErrTimestampFormatEmpty,
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
					assert.ErrorIs(t, err, apperrors.ErrInvalidColor)
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
				assert.ErrorIs(t, err, apperrors.ErrInvalidUserFormat)
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
				assert.ErrorIs(t, err, apperrors.ErrInvalidPIDFormat)
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
		expectError bool
		expectedErr error
	}{
		{
			name:   "valid text format",
			format: "text",
		},
		{
			name:   "valid json format",
			format: "json",
		},
		{
			name:   "valid structured format",
			format: "structured",
		},
		{
			name:        "invalid format",
			format:      "invalid",
			expectError: true,
			expectedErr: apperrors.ErrInvalidOutputFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := getDefaultConfig()
			cfg.Output.Format = tt.format

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
			expectedErr: apperrors.ErrInvalidStdoutLogLevel,
		},
		{
			name:        "invalid stderr level",
			stdoutLevel: "INFO",
			stderrLevel: "INVALID",
			expectError: true,
			expectedErr: apperrors.ErrInvalidStderrLogLevel,
		},
		{
			name:           "invalid detection level",
			stdoutLevel:    "INFO",
			stderrLevel:    "ERROR",
			detectionLevel: "invalid",
			keywords:       []string{"ERROR"},
			expectError:    true,
			expectedErr:    apperrors.ErrInvalidLogLevel,
		},
		{
			name:           "empty keywords",
			stdoutLevel:    "INFO",
			stderrLevel:    "ERROR",
			detectionLevel: "error",
			keywords:       []string{},
			expectError:    true,
			expectedErr:    apperrors.ErrNoDetectionKeywords,
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
			assert.ErrorIs(t, err, apperrors.ErrInvalidStdoutLogLevel)
		})

		t.Run("invalid_stderr_level_"+level, func(t *testing.T) {
			t.Parallel()

			cfg := getDefaultConfig()
			cfg.LogLevel.DefaultStderr = level

			err := cfg.Validate()
			assert.Error(t, err)
			assert.ErrorIs(t, err, apperrors.ErrInvalidStderrLogLevel)
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
	assert.ErrorIs(t, err, apperrors.ErrTemplateEmpty)
}

// TestConfig_ValidateTimestamp_InvalidStrftimeFormats tests various invalid strftime format directives.
func TestConfig_ValidateTimestamp_InvalidStrftimeFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		format       string
		expectError  bool
		errorMessage string
	}{
		// Valid formats
		{
			name:        "valid simple date",
			format:      "%Y-%m-%d",
			expectError: false,
		},
		{
			name:        "valid date and time",
			format:      "%Y-%m-%d %H:%M:%S",
			expectError: false,
		},
		{
			name:        "valid RFC3339-like",
			format:      "%Y-%m-%dT%H:%M:%S%z",
			expectError: false,
		},
		{
			name:        "valid with microseconds",
			format:      "%Y-%m-%d %H:%M:%S.%f",
			expectError: false,
		},
		{
			name:        "valid with weekday and month names",
			format:      "%a %b %d %Y %H:%M:%S",
			expectError: false,
		},
		{
			name:        "valid with timezone",
			format:      "%Y-%m-%d %H:%M:%S %Z",
			expectError: false,
		},
		{
			name:        "valid with escaped percent",
			format:      "%%Y-%m-%d",
			expectError: false,
		},
		// Invalid formats - these may or may not fail depending on timefmt-go implementation
		// Testing edge cases that should be caught
		{
			name:         "invalid directive %Q",
			format:       "%Q",
			expectError:  true,
			errorMessage: "invalid strftime format",
		},
		{
			name:         "invalid directive %K",
			format:       "%K",
			expectError:  true,
			errorMessage: "invalid strftime format",
		},
		{
			name:         "unclosed percent at end",
			format:       "%Y-%m-%d %",
			expectError:  true,
			errorMessage: "invalid strftime format",
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
				if tt.errorMessage != "" {
					assert.Contains(t, err.Error(), tt.errorMessage)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestConfig_ValidateLogLevel_EmptyKeywordsArray tests empty keyword arrays.
func TestConfig_ValidateLogLevel_EmptyKeywordsArray(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		keywords    map[string][]string
		expectError bool
		expectedErr error
	}{
		{
			name: "valid keywords with entries",
			keywords: map[string][]string{
				"error": {"ERROR", "FATAL"},
				"warn":  {"WARN"},
			},
			expectError: false,
		},
		{
			name: "empty keywords array for error",
			keywords: map[string][]string{
				"error": {},
			},
			expectError: true,
			expectedErr: apperrors.ErrNoDetectionKeywords,
		},
		{
			name: "empty keywords array for multiple levels",
			keywords: map[string][]string{
				"error": {},
				"warn":  {},
			},
			expectError: true,
			expectedErr: apperrors.ErrNoDetectionKeywords,
		},
		{
			name: "mixed empty and valid keywords",
			keywords: map[string][]string{
				"error": {"ERROR"},
				"warn":  {}, // Empty
			},
			expectError: true,
			expectedErr: apperrors.ErrNoDetectionKeywords,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := getDefaultConfig()
			cfg.LogLevel.Detection.Keywords = tt.keywords

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

// TestConfig_ValidateLogLevel_EmptyStringsInKeywords tests keywords containing empty strings.
func TestConfig_ValidateLogLevel_EmptyStringsInKeywords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		keywords    map[string][]string
		expectError bool
		expectedErr error
	}{
		{
			name: "valid keywords no empty strings",
			keywords: map[string][]string{
				"error": {"ERROR", "FATAL"},
			},
			expectError: false,
		},
		{
			name: "keywords with one empty string",
			keywords: map[string][]string{
				"error": {"ERROR", ""},
			},
			expectError: true,
			expectedErr: apperrors.ErrEmptyKeyword,
		},
		{
			name: "keywords with only empty string",
			keywords: map[string][]string{
				"error": {""},
			},
			expectError: true,
			expectedErr: apperrors.ErrEmptyKeyword,
		},
		{
			name: "keywords with multiple empty strings",
			keywords: map[string][]string{
				"error": {"", "", "ERROR"},
			},
			expectError: true,
			expectedErr: apperrors.ErrEmptyKeyword,
		},
		{
			name: "multiple levels with empty strings",
			keywords: map[string][]string{
				"error": {"ERROR", ""},
				"warn":  {"WARN"},
			},
			expectError: true,
			expectedErr: apperrors.ErrEmptyKeyword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := getDefaultConfig()
			cfg.LogLevel.Detection.Keywords = tt.keywords

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

// TestConfig_ValidateLogLevel_CaseSensitivity tests log level case sensitivity.
func TestConfig_ValidateLogLevel_CaseSensitivity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		level       string
		expectError bool
	}{
		// Valid uppercase
		{name: "valid TRACE", level: "TRACE", expectError: false},
		{name: "valid DEBUG", level: "DEBUG", expectError: false},
		{name: "valid INFO", level: "INFO", expectError: false},
		{name: "valid WARN", level: "WARN", expectError: false},
		{name: "valid ERROR", level: "ERROR", expectError: false},
		{name: "valid FATAL", level: "FATAL", expectError: false},
		// Valid lowercase
		{name: "valid trace", level: "trace", expectError: false},
		{name: "valid debug", level: "debug", expectError: false},
		{name: "valid info", level: "info", expectError: false},
		{name: "valid warn", level: "warn", expectError: false},
		{name: "valid error", level: "error", expectError: false},
		{name: "valid fatal", level: "fatal", expectError: false},
		// Invalid mixed case
		{name: "invalid Info", level: "Info", expectError: true},
		{name: "invalid Error", level: "Error", expectError: true},
		{name: "invalid WaRn", level: "WaRn", expectError: true},
		{name: "invalid DeBuG", level: "DeBuG", expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_stdout", func(t *testing.T) {
			t.Parallel()

			cfg := getDefaultConfig()
			cfg.LogLevel.DefaultStdout = tt.level

			err := cfg.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, apperrors.ErrInvalidStdoutLogLevel)
			} else {
				assert.NoError(t, err)
			}
		})

		t.Run(tt.name+"_stderr", func(t *testing.T) {
			t.Parallel()

			cfg := getDefaultConfig()
			cfg.LogLevel.DefaultStderr = tt.level

			err := cfg.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, apperrors.ErrInvalidStderrLogLevel)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestConfig_ValidateLogLevel_ConflictingConfiguration tests detection disabled but keywords provided.
func TestConfig_ValidateLogLevel_ConflictingConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		enabled     bool
		keywords    map[string][]string
		expectError bool
		expectedErr error
	}{
		{
			name:    "detection enabled with keywords - valid",
			enabled: true,
			keywords: map[string][]string{
				"error": {"ERROR"},
			},
			expectError: false,
		},
		{
			name:        "detection disabled with no keywords - valid",
			enabled:     false,
			keywords:    map[string][]string{},
			expectError: false,
		},
		{
			name:        "detection disabled with nil keywords - valid",
			enabled:     false,
			keywords:    nil,
			expectError: false,
		},
		{
			name:    "detection disabled but keywords provided - invalid",
			enabled: false,
			keywords: map[string][]string{
				"error": {"ERROR"},
			},
			expectError: true,
			expectedErr: apperrors.ErrDetectionDisabledWithKeywords,
		},
		{
			name:    "detection disabled but multiple keywords provided - invalid",
			enabled: false,
			keywords: map[string][]string{
				"error": {"ERROR"},
				"warn":  {"WARN"},
			},
			expectError: true,
			expectedErr: apperrors.ErrDetectionDisabledWithKeywords,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := getDefaultConfig()
			cfg.LogLevel.Detection.Enabled = tt.enabled
			cfg.LogLevel.Detection.Keywords = tt.keywords

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

// TestConfig_ValidateColors_CaseInsensitivity tests that color validation is case-insensitive.
func TestConfig_ValidateColors_CaseInsensitivity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		color       string
		expectError bool
	}{
		// Lowercase (should be valid)
		{name: "lowercase red", color: "red", expectError: false},
		{name: "lowercase blue", color: "blue", expectError: false},
		{name: "lowercase green", color: "green", expectError: false},
		// Uppercase (should be valid due to case-insensitive check)
		{name: "uppercase RED", color: "RED", expectError: false},
		{name: "uppercase BLUE", color: "BLUE", expectError: false},
		{name: "uppercase GREEN", color: "GREEN", expectError: false},
		// Mixed case (should be valid)
		{name: "mixed Red", color: "Red", expectError: false},
		{name: "mixed BlUe", color: "BlUe", expectError: false},
		// Invalid colors
		{name: "invalid purple", color: "purple", expectError: true},
		{name: "invalid ORANGE", color: "ORANGE", expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := getDefaultConfig()
			cfg.Prefix.Colors.Info = tt.color

			err := cfg.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, apperrors.ErrInvalidColor)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestConfig_ValidateOutputFormat_AllFormats tests all valid and invalid output formats.
func TestConfig_ValidateOutputFormat_AllFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		format      string
		expectError bool
	}{
		// Valid formats
		{name: "valid text", format: "text", expectError: false},
		{name: "valid json", format: "json", expectError: false},
		{name: "valid structured", format: "structured", expectError: false},
		// Invalid formats
		{name: "invalid xml", format: "xml", expectError: true},
		{name: "invalid yaml", format: "yaml", expectError: true},
		{name: "invalid empty", format: "", expectError: true},
		{name: "invalid mixed case", format: "Text", expectError: true},
		{name: "invalid uppercase", format: "JSON", expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := getDefaultConfig()
			cfg.Output.Format = tt.format

			err := cfg.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, apperrors.ErrInvalidOutputFormat)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}