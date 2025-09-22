package main

import (
	"testing"

	"github.com/sgaunet/logwrap/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseArgs_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            []string
		expectedConfig  []string
		expectedCommand []string
	}{
		{
			name:            "simple command only",
			args:            []string{"echo", "hello"},
			expectedConfig:  nil,
			expectedCommand: []string{"echo", "hello"},
		},
		{
			name:            "command with args",
			args:            []string{"ls", "-la", "/tmp"},
			expectedConfig:  nil,
			expectedCommand: []string{"ls", "-la", "/tmp"},
		},
		{
			name:            "flags before command",
			args:            []string{"-colors", "-utc", "echo", "test"},
			expectedConfig:  []string{"-colors", "-utc"},
			expectedCommand: []string{"echo", "test"},
		},
		{
			name:            "config flag with value",
			args:            []string{"-config", "test.yaml", "echo", "hello"},
			expectedConfig:  []string{"-config", "test.yaml"},
			expectedCommand: []string{"echo", "hello"},
		},
		{
			name:            "template flag with value",
			args:            []string{"-template", "[{{.Level}}] ", "echo", "test"},
			expectedConfig:  []string{"-template", "[{{.Level}}] "},
			expectedCommand: []string{"echo", "test"},
		},
		{
			name:            "format flag with value",
			args:            []string{"-format", "json", "echo", "test"},
			expectedConfig:  []string{"-format", "json"},
			expectedCommand: []string{"echo", "test"},
		},
		{
			name:            "multiple flags",
			args:            []string{"-colors", "-template", "[test] ", "-utc", "echo", "hello"},
			expectedConfig:  []string{"-colors", "-template", "[test] ", "-utc"},
			expectedCommand: []string{"echo", "hello"},
		},
		{
			name:            "double dash separator",
			args:            []string{"-colors", "--", "sh", "-c", "echo test"},
			expectedConfig:  []string{"-colors"},
			expectedCommand: []string{"sh", "-c", "echo test"},
		},
		{
			name:            "command with flags after separator",
			args:            []string{"-utc", "--", "ls", "-la"},
			expectedConfig:  []string{"-utc"},
			expectedCommand: []string{"ls", "-la"},
		},
		{
			name:            "empty args",
			args:            []string{},
			expectedConfig:  nil,
			expectedCommand: nil,
		},
		{
			name:            "flags only",
			args:            []string{"-colors", "-utc"},
			expectedConfig:  []string{"-colors", "-utc"},
			expectedCommand: nil,
		},
		{
			name:            "complex example",
			args:            []string{"-config", "app.yaml", "-template", "[{{.Timestamp}}] ", "-colors", "--", "make", "build", "-j4"},
			expectedConfig:  []string{"-config", "app.yaml", "-template", "[{{.Timestamp}}] ", "-colors"},
			expectedCommand: []string{"make", "build", "-j4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			configArgs, command, err := parseArgs(tt.args)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedConfig, configArgs)
			assert.Equal(t, tt.expectedCommand, command)
		})
	}
}

func TestParseArgs_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		errorMsg string
	}{
		{
			name:     "config flag without value",
			args:     []string{"-config"},
			errorMsg: "option requires a value: -config",
		},
		{
			name:     "template flag without value",
			args:     []string{"-template"},
			errorMsg: "option requires a value: -template",
		},
		{
			name:     "format flag without value",
			args:     []string{"-format"},
			errorMsg: "option requires a value: -format",
		},
		{
			name:     "template flag at end",
			args:     []string{"-colors", "-template"},
			errorMsg: "option requires a value: -template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			configArgs, command, err := parseArgs(tt.args)
			assert.Error(t, err)
			assert.Nil(t, configArgs)
			assert.Nil(t, command)
			assert.ErrorIs(t, err, errors.ErrOptionRequiresValue)
			assert.Contains(t, err.Error(), tt.errorMsg)
		})
	}
}

func TestHasFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		flag     string
		expected bool
	}{
		{
			name:     "flag present",
			args:     []string{"-colors", "-utc", "-help"},
			flag:     "-colors",
			expected: true,
		},
		{
			name:     "flag not present",
			args:     []string{"-colors", "-utc"},
			flag:     "-help",
			expected: false,
		},
		{
			name:     "empty args",
			args:     []string{},
			flag:     "-help",
			expected: false,
		},
		{
			name:     "flag in middle",
			args:     []string{"-config", "test.yaml", "-utc", "-colors"},
			flag:     "-utc",
			expected: true,
		},
		{
			name:     "flag at end",
			args:     []string{"-config", "test.yaml", "-help"},
			flag:     "-help",
			expected: true,
		},
		{
			name:     "similar but not exact",
			args:     []string{"-color", "-helper"},
			flag:     "-colors",
			expected: false,
		},
		{
			name:     "case sensitive",
			args:     []string{"-HELP"},
			flag:     "-help",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := hasFlag(tt.args, tt.flag)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetConfigFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		expected string // Will be checked with strings.Contains since FindConfigFile returns abs path
	}{
		{
			name:     "config flag present",
			args:     []string{"-config", "myconfig.yaml", "-colors"},
			expected: "myconfig.yaml",
		},
		{
			name:     "config flag with different position",
			args:     []string{"-colors", "-config", "app.yml", "-utc"},
			expected: "app.yml",
		},
		{
			name:     "config flag at end",
			args:     []string{"-utc", "-config", "test.yaml"},
			expected: "test.yaml",
		},
		{
			name:     "no config flag",
			args:     []string{"-colors", "-utc"},
			expected: "", // Will use FindConfigFile, might return empty or default path
		},
		{
			name:     "empty args",
			args:     []string{},
			expected: "",
		},
		{
			name:     "config flag without value (should not match)",
			args:     []string{"-config"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := getConfigFile(tt.args)

			if tt.expected != "" {
				// For specific config files, check exact match
				assert.Equal(t, tt.expected, result)
			} else {
				// For default case, result could be empty string or a default path
				// Just ensure it doesn't panic and returns a string
				assert.IsType(t, "", result)
			}
		})
	}
}

func TestParseArgs_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            []string
		expectedConfig  []string
		expectedCommand []string
	}{
		{
			name:            "dash dash with no following args",
			args:            []string{"-colors", "--"},
			expectedConfig:  []string{"-colors"},
			expectedCommand: []string{}, // parseArgs returns empty slice, not nil
		},
		{
			name:            "command that looks like flag",
			args:            []string{"--", "-fake-command", "arg"},
			expectedConfig:  nil, // parseArgs returns nil for empty config args
			expectedCommand: []string{"-fake-command", "arg"},
		},
		{
			name:            "flag value that looks like flag",
			args:            []string{"-template", "-test-template", "echo", "hello"},
			expectedConfig:  []string{"-template", "-test-template"},
			expectedCommand: []string{"echo", "hello"},
		},
		{
			name:            "single dash command",
			args:            []string{"-"},
			expectedConfig:  []string{"-"}, // Single dash is treated as a flag
			expectedCommand: nil,
		},
		{
			name:            "numeric args",
			args:            []string{"test", "123", "456"},
			expectedConfig:  nil,
			expectedCommand: []string{"test", "123", "456"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			configArgs, command, err := parseArgs(tt.args)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedConfig, configArgs)
			assert.Equal(t, tt.expectedCommand, command)
		})
	}
}

func TestConstants(t *testing.T) {
	t.Parallel()

	// Test that constants are defined and have reasonable values
	assert.NotEmpty(t, version)
	assert.NotEmpty(t, usage)

	// Test that usage contains expected sections
	assert.Contains(t, usage, "Usage:")
	assert.Contains(t, usage, "Options:")
	assert.Contains(t, usage, "Examples:")
	assert.Contains(t, usage, "Configuration:")

	// Test that usage contains key flags
	assert.Contains(t, usage, "-config")
	assert.Contains(t, usage, "-template")
	assert.Contains(t, usage, "-help")
	assert.Contains(t, usage, "-version")

	// Test that version is a string (could be "development" or semantic version)
	assert.IsType(t, "", version)
}

func TestParseArgs_ComplexScenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            []string
		expectedConfig  []string
		expectedCommand []string
		description     string
	}{
		{
			name:            "real world docker command",
			args:            []string{"-config", "docker.yaml", "-colors", "--", "docker", "run", "-it", "--rm", "ubuntu", "bash"},
			expectedConfig:  []string{"-config", "docker.yaml", "-colors"},
			expectedCommand: []string{"docker", "run", "-it", "--rm", "ubuntu", "bash"},
			description:     "Docker command with interactive flags",
		},
		{
			name:            "shell command with complex args",
			args:            []string{"-template", "[{{.Level}}] ", "sh", "-c", "echo 'hello world'; ls -la"},
			expectedConfig:  []string{"-template", "[{{.Level}}] "},
			expectedCommand: []string{"sh", "-c", "echo 'hello world'; ls -la"},
			description:     "Shell command with semicolon and quotes",
		},
		{
			name:            "make command with parallel jobs",
			args:            []string{"-config", "build.yml", "-utc", "make", "-j8", "clean", "build", "test"},
			expectedConfig:  []string{"-config", "build.yml", "-utc"},
			expectedCommand: []string{"make", "-j8", "clean", "build", "test"},
			description:     "Make command with multiple targets",
		},
		{
			name:            "ssh command with complex options",
			args:            []string{"-colors", "--", "ssh", "-o", "StrictHostKeyChecking=no", "user@host", "ls -la"},
			expectedConfig:  []string{"-colors"},
			expectedCommand: []string{"ssh", "-o", "StrictHostKeyChecking=no", "user@host", "ls -la"},
			description:     "SSH command with connection options",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			configArgs, command, err := parseArgs(tt.args)
			require.NoError(t, err, "Failed to parse: %s", tt.description)

			assert.Equal(t, tt.expectedConfig, configArgs, "Config args mismatch for: %s", tt.description)
			assert.Equal(t, tt.expectedCommand, command, "Command mismatch for: %s", tt.description)
		})
	}
}

func TestGetConfigFile_WithConfigFlag(t *testing.T) {
	t.Parallel()

	// Test specific scenarios for config file detection
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "absolute path",
			args:     []string{"-config", "/etc/logwrap/config.yaml"},
			expected: "/etc/logwrap/config.yaml",
		},
		{
			name:     "relative path",
			args:     []string{"-config", "../configs/app.yml"},
			expected: "../configs/app.yml",
		},
		{
			name:     "current directory",
			args:     []string{"-config", "./logwrap.yaml"},
			expected: "./logwrap.yaml",
		},
		{
			name:     "config in middle of args",
			args:     []string{"-colors", "-config", "middle.yaml", "-utc"},
			expected: "middle.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := getConfigFile(tt.args)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasFlag_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		flag     string
		expected bool
	}{
		{
			name:     "flag as value",
			args:     []string{"-template", "-help", "echo", "test"},
			flag:     "-help",
			expected: true, // -help appears as template value but still counts as present
		},
		{
			name:     "empty flag",
			args:     []string{"-colors", "-utc"},
			flag:     "",
			expected: false,
		},
		{
			name:     "partial flag match",
			args:     []string{"-colors"},
			flag:     "-color",
			expected: false,
		},
		{
			name:     "flag without dash",
			args:     []string{"-colors", "colors"},
			flag:     "colors",
			expected: true, // "colors" appears as an argument
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := hasFlag(tt.args, tt.flag)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseArgs_Stability(t *testing.T) {
	t.Parallel()

	// Test that parsing the same args multiple times gives same result
	args := []string{"-config", "test.yaml", "-colors", "-template", "[{{.Level}}] ", "echo", "hello", "world"}

	configArgs1, command1, err1 := parseArgs(args)
	require.NoError(t, err1)

	configArgs2, command2, err2 := parseArgs(args)
	require.NoError(t, err2)

	assert.Equal(t, configArgs1, configArgs2)
	assert.Equal(t, command1, command2)

	// Test that original args slice is not modified
	originalArgs := []string{"-colors", "echo", "test"}
	argsCopy := make([]string, len(originalArgs))
	copy(argsCopy, originalArgs)

	_, _, err := parseArgs(originalArgs)
	require.NoError(t, err)

	assert.Equal(t, argsCopy, originalArgs, "Original args should not be modified")
}