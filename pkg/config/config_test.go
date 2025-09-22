package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sgaunet/logwrap/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_DefaultConfig(t *testing.T) {
	t.Parallel()

	// Test loading with no config file and no CLI args
	cfg, err := LoadConfig("", []string{})
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify default values
	assert.Equal(t, "[{{.Timestamp}}] [{{.Level}}] [{{.User}}:{{.PID}}] ", cfg.Prefix.Template)
	assert.Equal(t, time.RFC3339, cfg.Prefix.Timestamp.Format)
	assert.False(t, cfg.Prefix.Timestamp.UTC)
	assert.False(t, cfg.Prefix.Colors.Enabled)
	assert.Equal(t, "green", cfg.Prefix.Colors.Info)
	assert.Equal(t, "red", cfg.Prefix.Colors.Error)
	assert.Equal(t, "blue", cfg.Prefix.Colors.Timestamp)
	assert.True(t, cfg.Prefix.User.Enabled)
	assert.Equal(t, "username", cfg.Prefix.User.Format)
	assert.True(t, cfg.Prefix.PID.Enabled)
	assert.Equal(t, "decimal", cfg.Prefix.PID.Format)
	assert.Equal(t, "text", cfg.Output.Format)
	assert.Equal(t, "line", cfg.Output.Buffer)
	assert.Equal(t, "INFO", cfg.LogLevel.DefaultStdout)
	assert.Equal(t, "ERROR", cfg.LogLevel.DefaultStderr)
	assert.True(t, cfg.LogLevel.Detection.Enabled)
}

func TestLoadConfig_WithValidConfigFile(t *testing.T) {
	t.Parallel()

	configContent := testutils.ValidYAMLConfig()
	configFile := testutils.CreateTempConfigFile(t, configContent)

	cfg, err := LoadConfig(configFile, []string{})
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify loaded values
	assert.Equal(t, "[{{.Timestamp}}] [{{.Level}}] ", cfg.Prefix.Template)
	assert.Equal(t, "2006-01-02 15:04:05", cfg.Prefix.Timestamp.Format)
	assert.False(t, cfg.Prefix.Timestamp.UTC)
	assert.False(t, cfg.Prefix.Colors.Enabled)
	assert.True(t, cfg.Prefix.User.Enabled)
	assert.Equal(t, "username", cfg.Prefix.User.Format)
	assert.True(t, cfg.Prefix.PID.Enabled)
	assert.Equal(t, "decimal", cfg.Prefix.PID.Format)
	assert.Equal(t, "text", cfg.Output.Format)
	assert.Equal(t, "line", cfg.Output.Buffer)
	assert.Equal(t, "INFO", cfg.LogLevel.DefaultStdout)
	assert.Equal(t, "ERROR", cfg.LogLevel.DefaultStderr)
	assert.True(t, cfg.LogLevel.Detection.Enabled)
}

func TestLoadConfig_WithInvalidConfigFile(t *testing.T) {
	t.Parallel()

	invalidContent := testutils.InvalidYAMLConfig()
	configFile := testutils.CreateTempConfigFile(t, invalidContent)

	cfg, err := LoadConfig(configFile, []string{})
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "failed to parse YAML config")
}

func TestLoadConfig_ConfigFileNotFound(t *testing.T) {
	t.Parallel()

	nonExistentFile := "/nonexistent/config.yaml"

	cfg, err := LoadConfig(nonExistentFile, []string{})
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestLoadConfig_WithCLIOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		expected func(*testing.T, *Config)
	}{
		{
			name: "template override",
			args: []string{"-template", "[{{.Level}}] "},
			expected: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "[{{.Level}}] ", cfg.Prefix.Template)
			},
		},
		{
			name: "utc override",
			args: []string{"-utc"},
			expected: func(t *testing.T, cfg *Config) {
				assert.True(t, cfg.Prefix.Timestamp.UTC)
			},
		},
		{
			name: "colors override",
			args: []string{"-colors"},
			expected: func(t *testing.T, cfg *Config) {
				assert.True(t, cfg.Prefix.Colors.Enabled)
			},
		},
		{
			name: "format override",
			args: []string{"-format", "json"},
			expected: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "json", cfg.Output.Format)
			},
		},
		{
			name: "multiple overrides",
			args: []string{"-template", "[{{.Level}}] ", "-utc", "-format", "structured"},
			expected: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "[{{.Level}}] ", cfg.Prefix.Template)
				assert.True(t, cfg.Prefix.Timestamp.UTC)
				assert.Equal(t, "structured", cfg.Output.Format)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := LoadConfig("", tt.args)
			require.NoError(t, err)
			require.NotNil(t, cfg)

			tt.expected(t, cfg)
		})
	}
}

func TestLoadConfig_InvalidCLIFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		errorMsg string
	}{
		{
			name:     "invalid flag format",
			args:     []string{"-invalid-flag"},
			errorMsg: "failed to parse CLI flags",
		},
		{
			name:     "missing template value",
			args:     []string{"-template"},
			errorMsg: "failed to parse CLI flags",
		},
		{
			name:     "missing format value",
			args:     []string{"-format"},
			errorMsg: "failed to parse CLI flags",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := LoadConfig("", tt.args)
			assert.Error(t, err)
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tt.errorMsg)
		})
	}
}

func TestLoadConfig_ConfigValidationFailure(t *testing.T) {
	t.Parallel()

	// Create a config file with invalid values that will fail validation
	invalidConfig := `
prefix:
  template: ""  # Empty template should fail validation
output:
  format: "text"
log_level:
  default_stdout: "INFO"
  default_stderr: "ERROR"
`

	configFile := testutils.CreateTempConfigFile(t, invalidConfig)

	cfg, err := LoadConfig(configFile, []string{})
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "invalid configuration")
}

func TestFindConfigFile(t *testing.T) {
	t.Parallel()

	// Save original working directory
	originalWd, err := os.Getwd()
	require.NoError(t, err)

	// Create temporary directory and change to it
	tempDir := testutils.CreateTempDir(t)

	// Test case: no config files exist
	t.Run("no config files", func(t *testing.T) {
		err = os.Chdir(tempDir)
		require.NoError(t, err)

		result := FindConfigFile()
		assert.Equal(t, "", result)
	})

	// Test case: logwrap.yaml exists
	t.Run("logwrap.yaml exists", func(t *testing.T) {
		configPath := filepath.Join(tempDir, "logwrap.yaml")
		err = os.WriteFile(configPath, []byte("test"), 0644)
		require.NoError(t, err)

		err = os.Chdir(tempDir)
		require.NoError(t, err)

		result := FindConfigFile()
		assert.Equal(t, "logwrap.yaml", result)

		// Clean up
		os.Remove(configPath)
	})

	// Test case: logwrap.yml exists
	t.Run("logwrap.yml exists", func(t *testing.T) {
		configPath := filepath.Join(tempDir, "logwrap.yml")
		err = os.WriteFile(configPath, []byte("test"), 0644)
		require.NoError(t, err)

		err = os.Chdir(tempDir)
		require.NoError(t, err)

		result := FindConfigFile()
		assert.Equal(t, "logwrap.yml", result)

		// Clean up
		os.Remove(configPath)
	})

	// Restore original working directory
	err = os.Chdir(originalWd)
	require.NoError(t, err)
}

func TestParseCLIFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		expected func(*testing.T, *CLIFlags)
		wantErr  bool
	}{
		{
			name: "all flags",
			args: []string{
				"-config", "test.yaml",
				"-template", "[{{.Level}}] ",
				"-utc",
				"-colors",
				"-format", "json",
				"-help",
				"-version",
			},
			expected: func(t *testing.T, flags *CLIFlags) {
				assert.Equal(t, "test.yaml", *flags.ConfigFile)
				assert.Equal(t, "[{{.Level}}] ", *flags.Template)
				assert.True(t, *flags.TimestampUTC)
				assert.True(t, *flags.ColorsEnabled)
				assert.Equal(t, "json", *flags.OutputFormat)
				assert.True(t, *flags.Help)
				assert.True(t, *flags.Version)
			},
			wantErr: false,
		},
		{
			name: "no flags",
			args: []string{},
			expected: func(t *testing.T, flags *CLIFlags) {
				assert.Equal(t, "", *flags.ConfigFile)
				assert.Equal(t, "", *flags.Template)
				assert.False(t, *flags.TimestampUTC)
				assert.False(t, *flags.ColorsEnabled)
				assert.Equal(t, "", *flags.OutputFormat)
				assert.False(t, *flags.Help)
				assert.False(t, *flags.Version)
			},
			wantErr: false,
		},
		{
			name:    "invalid flag",
			args:    []string{"-invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			flags, err := parseCLIFlags(tt.args)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, flags)
			} else {
				require.NoError(t, err)
				require.NotNil(t, flags)
				tt.expected(t, flags)
			}
		})
	}
}

func TestValidateConfigPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		wantErr  bool
		errorMsg string
	}{
		{
			name:    "valid yaml file",
			path:    "config.yaml",
			wantErr: false,
		},
		{
			name:    "valid yml file",
			path:    "config.yml",
			wantErr: false,
		},
		{
			name:    "valid nested yaml file",
			path:    "configs/app.yaml",
			wantErr: false,
		},
		{
			name:     "path traversal attack",
			path:     "../../../etc/passwd",
			wantErr:  true,
			errorMsg: "path traversal not allowed",
		},
		{
			name:     "invalid file extension",
			path:     "config.txt",
			wantErr:  true,
			errorMsg: "only .yaml and .yml files are allowed",
		},
		{
			name:     "no extension",
			path:     "config",
			wantErr:  true,
			errorMsg: "only .yaml and .yml files are allowed",
		},
		{
			name:     "complex path traversal",
			path:     "configs/../../../etc/passwd.yaml",
			wantErr:  true,
			errorMsg: "path traversal not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateConfigPath(tt.path)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := getDefaultConfig()
	require.NotNil(t, cfg)

	// Test all default values are set correctly
	assert.Equal(t, "[{{.Timestamp}}] [{{.Level}}] [{{.User}}:{{.PID}}] ", cfg.Prefix.Template)
	assert.Equal(t, time.RFC3339, cfg.Prefix.Timestamp.Format)
	assert.False(t, cfg.Prefix.Timestamp.UTC)
	assert.False(t, cfg.Prefix.Colors.Enabled)
	assert.Equal(t, "green", cfg.Prefix.Colors.Info)
	assert.Equal(t, "red", cfg.Prefix.Colors.Error)
	assert.Equal(t, "blue", cfg.Prefix.Colors.Timestamp)
	assert.True(t, cfg.Prefix.User.Enabled)
	assert.Equal(t, "username", cfg.Prefix.User.Format)
	assert.True(t, cfg.Prefix.PID.Enabled)
	assert.Equal(t, "decimal", cfg.Prefix.PID.Format)
	assert.Equal(t, "text", cfg.Output.Format)
	assert.Equal(t, "line", cfg.Output.Buffer)
	assert.Equal(t, "INFO", cfg.LogLevel.DefaultStdout)
	assert.Equal(t, "ERROR", cfg.LogLevel.DefaultStderr)
	assert.True(t, cfg.LogLevel.Detection.Enabled)

	// Test default keywords are present
	assert.Contains(t, cfg.LogLevel.Detection.Keywords["error"], "ERROR")
	assert.Contains(t, cfg.LogLevel.Detection.Keywords["error"], "FATAL")
	assert.Contains(t, cfg.LogLevel.Detection.Keywords["error"], "PANIC")
	assert.Contains(t, cfg.LogLevel.Detection.Keywords["warn"], "WARN")
	assert.Contains(t, cfg.LogLevel.Detection.Keywords["warn"], "WARNING")
	assert.Contains(t, cfg.LogLevel.Detection.Keywords["debug"], "DEBUG")
	assert.Contains(t, cfg.LogLevel.Detection.Keywords["debug"], "TRACE")
	assert.Contains(t, cfg.LogLevel.Detection.Keywords["info"], "INFO")
}

func TestApplyCLIOverrides(t *testing.T) {
	t.Parallel()

	// Start with default config
	cfg := getDefaultConfig()
	originalTemplate := cfg.Prefix.Template

	// Create flags with overrides
	template := "[{{.Level}}] "
	utc := true
	colors := true
	userEnabled := false
	pidEnabled := false
	format := "json"

	flags := &CLIFlags{
		Template:      &template,
		TimestampUTC:  &utc,
		ColorsEnabled: &colors,
		UserEnabled:   &userEnabled,
		PIDEnabled:    &pidEnabled,
		OutputFormat:  &format,
	}

	// Apply overrides
	applyCLIOverrides(cfg, flags)

	// Verify overrides were applied
	assert.Equal(t, template, cfg.Prefix.Template)
	assert.True(t, cfg.Prefix.Timestamp.UTC)
	assert.True(t, cfg.Prefix.Colors.Enabled)
	assert.False(t, cfg.Prefix.User.Enabled)
	assert.False(t, cfg.Prefix.PID.Enabled)
	assert.Equal(t, format, cfg.Output.Format)

	// Test that nil values don't override
	cfg2 := getDefaultConfig()
	emptyFlags := &CLIFlags{}

	applyCLIOverrides(cfg2, emptyFlags)

	// Config should remain unchanged
	assert.Equal(t, originalTemplate, cfg2.Prefix.Template)
	assert.False(t, cfg2.Prefix.Timestamp.UTC)
	assert.False(t, cfg2.Prefix.Colors.Enabled)

	// Test empty string values don't override
	cfg3 := getDefaultConfig()
	emptyString := ""
	emptyStringFlags := &CLIFlags{
		Template:     &emptyString,
		OutputFormat: &emptyString,
	}

	applyCLIOverrides(cfg3, emptyStringFlags)

	// Config should remain unchanged for empty strings
	assert.Equal(t, originalTemplate, cfg3.Prefix.Template)
	assert.Equal(t, "text", cfg3.Output.Format) // Should keep default
}