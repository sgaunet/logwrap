package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyTheme_AllThemes(t *testing.T) {
	t.Parallel()

	for _, name := range ThemeNames() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			colors := &ColorsConfig{Enabled: true}
			err := applyTheme(colors, name)
			require.NoError(t, err)
			assert.NotEmpty(t, colors.Info, "theme %s should set Info color", name)
			assert.NotEmpty(t, colors.Error, "theme %s should set Error color", name)
			assert.NotEmpty(t, colors.Timestamp, "theme %s should set Timestamp color", name)
		})
	}
}

func TestApplyTheme_UnknownTheme(t *testing.T) {
	t.Parallel()

	colors := &ColorsConfig{Enabled: true}
	err := applyTheme(colors, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown color theme")
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestApplyTheme_CaseInsensitive(t *testing.T) {
	t.Parallel()

	tests := []string{"Default", "DEFAULT", "Warm", "COOL"}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			colors := &ColorsConfig{Enabled: true}
			err := applyTheme(colors, name)
			assert.NoError(t, err)
		})
	}
}

func TestApplyTheme_OverridesExistingColors(t *testing.T) {
	t.Parallel()

	colors := &ColorsConfig{
		Enabled:   true,
		Info:      "cyan",
		Error:     "magenta",
		Timestamp: "yellow",
	}

	err := applyTheme(colors, "default")
	require.NoError(t, err)
	assert.Equal(t, "green", colors.Info, "theme should override existing Info")
	assert.Equal(t, "red", colors.Error, "theme should override existing Error")
	assert.Equal(t, "blue", colors.Timestamp, "theme should override existing Timestamp")
}

func TestThemeNames_Sorted(t *testing.T) {
	t.Parallel()

	names := ThemeNames()
	require.NotEmpty(t, names)

	for i := 1; i < len(names); i++ {
		assert.True(t, names[i-1] < names[i],
			"ThemeNames should be sorted, but %q >= %q", names[i-1], names[i])
	}
}

func TestThemeNames_ContainsDefault(t *testing.T) {
	t.Parallel()

	names := ThemeNames()
	assert.Contains(t, names, "default")
}

func TestLoadConfig_WithTheme(t *testing.T) {
	t.Parallel()

	yamlContent := `
prefix:
  template: "[{{.Level}}] "
  timestamp:
    format: "%H:%M:%S"
  colors:
    enabled: true
    theme: "warm"
  user:
    enabled: false
    format: "username"
  pid:
    enabled: false
    format: "decimal"
output:
  format: "text"
log_level:
  default_stdout: "INFO"
  default_stderr: "ERROR"
  detection:
    enabled: true
    keywords:
      error: ["ERROR"]
`
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"
	require.NoError(t, writeTestFile(configPath, yamlContent))

	cfg, err := LoadConfig(configPath, nil)
	require.NoError(t, err)

	// Warm theme: info=yellow, error=red, timestamp=magenta
	assert.Equal(t, "yellow", cfg.Prefix.Colors.Info)
	assert.Equal(t, "red", cfg.Prefix.Colors.Error)
	assert.Equal(t, "magenta", cfg.Prefix.Colors.Timestamp)
}

func TestLoadConfig_InvalidTheme(t *testing.T) {
	t.Parallel()

	yamlContent := `
prefix:
  template: "[{{.Level}}] "
  timestamp:
    format: "%H:%M:%S"
  colors:
    enabled: true
    theme: "nonexistent"
  user:
    enabled: false
    format: "username"
  pid:
    enabled: false
    format: "decimal"
output:
  format: "text"
log_level:
  default_stdout: "INFO"
  default_stderr: "ERROR"
  detection:
    enabled: true
    keywords:
      error: ["ERROR"]
`
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"
	require.NoError(t, writeTestFile(configPath, yamlContent))

	_, err := LoadConfig(configPath, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown color theme")
}

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}
