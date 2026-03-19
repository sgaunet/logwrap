package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// configFilePermissions matches the permission used in testutils.
const testConfigFilePermissions = 0600

// Note: FindConfigFile tests cannot use t.Parallel() because they call os.Chdir,
// which modifies global process state.

func TestFindConfigFile_YamlOverYml(t *testing.T) {
	tmpDir := t.TempDir()

	yamlContent := validConfigWithFormat("json")
	ymlContent := validConfigWithFormat("structured")

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "logwrap.yaml"), []byte(yamlContent), testConfigFilePermissions))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "logwrap.yml"), []byte(ymlContent), testConfigFilePermissions))

	// chdir to the temp directory so FindConfigFile finds the files.
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	result := FindConfigFile()
	assert.Equal(t, "logwrap.yaml", result, ".yaml should have priority over .yml")
}

func TestFindConfigFile_LogwrapOverDotLogwrap(t *testing.T) {
	tmpDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "logwrap.yaml"), []byte(validConfigWithFormat("json")), testConfigFilePermissions))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".logwrap.yaml"), []byte(validConfigWithFormat("structured")), testConfigFilePermissions))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	result := FindConfigFile()
	assert.Equal(t, "logwrap.yaml", result, "logwrap.yaml should have priority over .logwrap.yaml")
}

func TestFindConfigFile_FallbackToYml(t *testing.T) {
	tmpDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "logwrap.yml"), []byte(validConfigWithFormat("json")), testConfigFilePermissions))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	result := FindConfigFile()
	assert.Equal(t, "logwrap.yml", result, "should find .yml when .yaml does not exist")
}

func TestFindConfigFile_FallbackToDotLogwrap(t *testing.T) {
	tmpDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".logwrap.yaml"), []byte(validConfigWithFormat("json")), testConfigFilePermissions))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	result := FindConfigFile()
	assert.Equal(t, ".logwrap.yaml", result, "should find .logwrap.yaml when logwrap.yaml/yml don't exist")
}

func TestFindConfigFile_NoneFound(t *testing.T) {
	tmpDir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	result := FindConfigFile()
	// May return empty or a home-dir config if one exists on the test machine.
	// We can only assert it doesn't return a cwd-relative path.
	if result != "" {
		assert.True(t, filepath.IsAbs(result),
			"when no cwd config exists, result should be empty or an absolute home path, got: %s", result)
	}
}

func TestFindConfigFile_CompleteOrder(t *testing.T) {
	tmpDir := t.TempDir()

	// Create all four cwd candidates.
	files := []string{"logwrap.yaml", "logwrap.yml", ".logwrap.yaml", ".logwrap.yml"}
	for _, f := range files {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, f), []byte(validConfigWithFormat("text")), testConfigFilePermissions))
	}

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Remove files one by one and verify fallback order.
	expected := []string{"logwrap.yaml", "logwrap.yml", ".logwrap.yaml", ".logwrap.yml"}
	for i, exp := range expected {
		result := FindConfigFile()
		assert.Equal(t, exp, result, "step %d: expected %s", i, exp)
		require.NoError(t, os.Remove(filepath.Join(tmpDir, exp)))
	}
}

func TestLoadConfig_ExplicitPathNotFound(t *testing.T) {
	t.Parallel()

	_, err := LoadConfig("/nonexistent/path/config.yaml", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load config file")
}

func TestLoadConfig_ExplicitPathLoaded(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "custom.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(validConfigWithFormat("json")), testConfigFilePermissions))

	cfg, err := LoadConfig(configPath, nil)
	require.NoError(t, err)
	assert.Equal(t, "json", cfg.Output.Format, "explicit config should be loaded")
}

func TestLoadConfig_EmptyPathUsesDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := LoadConfig("", nil)
	require.NoError(t, err)
	assert.Equal(t, "text", cfg.Output.Format, "empty path should use defaults")
}

func TestLoadConfig_CLIOverridesConfigFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(validConfigWithFormat("json")), testConfigFilePermissions))

	cfg, err := LoadConfig(configPath, []string{"-format", "structured"})
	require.NoError(t, err)
	assert.Equal(t, "structured", cfg.Output.Format, "CLI flags should override config file")
}

// validConfigWithFormat returns a complete valid YAML config with the given output format.
func validConfigWithFormat(format string) string {
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
  format: "` + format + `"
log_level:
  default_stdout: "INFO"
  default_stderr: "ERROR"
  detection:
    enabled: true
    keywords:
      error: ["ERROR"]
      warn: ["WARN"]
      info: ["INFO"]
`
}
