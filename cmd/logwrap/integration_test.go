package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sgaunet/logwrap/internal/testutils"
	"github.com/sgaunet/logwrap/pkg/config"
	"github.com/sgaunet/logwrap/pkg/executor"
	"github.com/sgaunet/logwrap/pkg/formatter"
	"github.com/sgaunet/logwrap/pkg/processor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testBinaryPath holds the path to the compiled logwrap binary for subprocess tests.
var testBinaryPath string

// Timestamp regex patterns for validation (never assert exact timestamps).
var (
	// Matches default strftime format "%Y-%m-%dT%H:%M:%S%z" (e.g., 2024-01-15T14:30:45-0700).
	defaultTimestampPattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{4}`)
	// Matches simple "%Y-%m-%d %H:%M:%S" format (e.g., 2024-01-15 14:30:45).
	simpleTimestampPattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`)
)

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "logwrap-integration-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}

	testBinaryPath = filepath.Join(tmpDir, "logwrap")
	cmd := exec.Command("go", "build", "-o", testBinaryPath, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to build test binary: %v\n", err)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	exitCode := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(exitCode)
}

// runPipeline constructs the full logwrap pipeline with a thread-safe captured
// writer instead of os.Stdout, and returns the formatted output and exit code.
// Uses testutils.MockWriter because the processor writes from two concurrent
// goroutines (stdout + stderr), making bytes.Buffer unsafe.
func runPipeline(t *testing.T, cfg *config.Config, command []string) (string, int) {
	t.Helper()

	e, err := executor.New(command)
	require.NoError(t, err)
	t.Cleanup(func() { e.Cleanup() })

	form, err := formatter.New(cfg)
	require.NoError(t, err)

	output := &testutils.MockWriter{}
	proc := processor.New(form, output)

	err = e.Start()
	require.NoError(t, err)

	stdout, stderr := e.GetStreams()

	ctx := testutils.TestContext(t)
	processingDone := make(chan error, 1)
	go func() {
		processingDone <- proc.ProcessStreams(ctx, stdout, stderr)
	}()

	// Wait for stream processing to complete first. The processor finishes
	// when the pipes close (process exit closes the write end). We must do
	// this BEFORE exec.Wait() because cmd.Wait() closes the read end of the
	// pipes, which would cause the scanner to lose buffered data.
	select {
	case procErr := <-processingDone:
		if procErr != nil {
			t.Logf("Stream processing completed with error: %v", procErr)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Stream processing timed out")
	}

	// Now collect exit code (process has already exited at this point).
	_ = e.Wait()

	return strings.Join(output.GetLines(), ""), e.GetExitCode()
}

// defaultTestConfig returns a validated default configuration.
func defaultTestConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.LoadConfig("", nil)
	require.NoError(t, err)
	return cfg
}

func TestIntegration_BasicPipeline(t *testing.T) {
	t.Parallel()

	cfg := defaultTestConfig(t)
	output, exitCode := runPipeline(t, cfg, []string{"echo", "test message"})

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "test message")
	assert.Regexp(t, defaultTimestampPattern, output)
	assert.Contains(t, output, "INFO")

	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.NotEmpty(t, lines)
	assert.True(t, strings.HasPrefix(lines[0], "["), "output should start with prefix bracket, got: %s", lines[0])
}

func TestIntegration_MixedStreams(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("shell redirection test not supported on Windows")
	}

	cfg := defaultTestConfig(t)
	output, exitCode := runPipeline(t, cfg, []string{
		"sh", "-c", "echo 'stdout line'; echo 'stderr line' >&2",
	})

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "stdout line")
	assert.Contains(t, output, "stderr line")

	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.GreaterOrEqual(t, len(lines), 2)

	var stdoutLine, stderrLine string
	for _, line := range lines {
		if strings.Contains(line, "stdout line") {
			stdoutLine = line
		}
		if strings.Contains(line, "stderr line") {
			stderrLine = line
		}
	}

	require.NotEmpty(t, stdoutLine, "stdout line should be in output")
	require.NotEmpty(t, stderrLine, "stderr line should be in output")
	assert.Contains(t, stdoutLine, "[INFO]")
	assert.Contains(t, stderrLine, "[ERROR]")
}

func TestIntegration_ConfigFileLoading(t *testing.T) {
	t.Parallel()

	yamlContent := `
prefix:
  template: "{{.Level}}> "
  timestamp:
    format: "%Y-%m-%d %H:%M:%S"
    utc: true
  colors:
    enabled: false
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
`
	configFile := testutils.CreateTempConfigFile(t, yamlContent)
	cfg, err := config.LoadConfig(configFile, nil)
	require.NoError(t, err)

	output, exitCode := runPipeline(t, cfg, []string{"echo", "configured output"})

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "INFO> configured output")

	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.NotEmpty(t, lines)
	assert.True(t, strings.HasPrefix(lines[0], "INFO> "),
		"line should start with simple level prefix, got: %s", lines[0])
}

func TestIntegration_ExitCodePreservation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess integration test in short mode")
	}
	if runtime.GOOS == "windows" {
		t.Skip("shell exit codes test not supported on Windows")
	}
	t.Parallel()

	tests := []struct {
		name         string
		command      []string
		expectedCode int
	}{
		{"exit 0", []string{"sh", "-c", "exit 0"}, 0},
		{"exit 1", []string{"sh", "-c", "exit 1"}, 1},
		{"exit 42", []string{"sh", "-c", "exit 42"}, 42},
		{"exit 127", []string{"sh", "-c", "exit 127"}, 127},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			args := append([]string{"--"}, tt.command...)
			cmd := exec.Command(testBinaryPath, args...)
			err := cmd.Run()

			if tt.expectedCode == 0 {
				assert.NoError(t, err)
			} else {
				var exitErr *exec.ExitError
				require.ErrorAs(t, err, &exitErr)
				assert.Equal(t, tt.expectedCode, exitErr.ExitCode())
			}
		})
	}
}

func TestIntegration_ExitCodePreservation_Pipeline(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("shell exit codes test not supported on Windows")
	}

	tests := []struct {
		name         string
		command      []string
		expectedCode int
	}{
		{"success", []string{"true"}, 0},
		{"failure", []string{"false"}, 1},
		{"exit 42", []string{"sh", "-c", "exit 42"}, 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := defaultTestConfig(t)
			_, exitCode := runPipeline(t, cfg, tt.command)
			assert.Equal(t, tt.expectedCode, exitCode)
		})
	}
}

func TestIntegration_RealTimeProcessing(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("shell sleep test not supported on Windows")
	}

	cfg := defaultTestConfig(t)

	e, err := executor.New([]string{
		"sh", "-c", "echo 'line1'; sleep 0.1; echo 'line2'; sleep 0.1; echo 'line3'",
	})
	require.NoError(t, err)
	t.Cleanup(func() { e.Cleanup() })

	form, err := formatter.New(cfg)
	require.NoError(t, err)

	// Use thread-safe MockWriter for concurrent reads during processing.
	output := &testutils.MockWriter{}
	proc := processor.New(form, output)

	err = e.Start()
	require.NoError(t, err)

	stdout, stderr := e.GetStreams()

	ctx := testutils.TestContext(t)
	processingDone := make(chan error, 1)
	go func() {
		processingDone <- proc.ProcessStreams(ctx, stdout, stderr)
	}()

	// Verify line1 appears before the command finishes (~300ms total).
	require.Eventually(t, func() bool {
		for _, line := range output.GetLines() {
			if strings.Contains(line, "line1") {
				return true
			}
		}
		return false
	}, 2*time.Second, 10*time.Millisecond,
		"line1 should appear before command completes")

	// Wait for processing to complete before exec.Wait() to avoid pipe closure race.
	select {
	case <-processingDone:
	case <-time.After(10 * time.Second):
		t.Fatal("processing timed out")
	}

	_ = e.Wait()

	allOutput := strings.Join(output.GetLines(), "")
	assert.Contains(t, allOutput, "line1")
	assert.Contains(t, allOutput, "line2")
	assert.Contains(t, allOutput, "line3")
}

func TestIntegration_TemplateRendering(t *testing.T) {
	t.Parallel()

	currentUser, err := user.Current()
	require.NoError(t, err)

	tests := []struct {
		name     string
		config   func(t *testing.T) *config.Config
		validate func(t *testing.T, output string)
	}{
		{
			name: "timestamp only",
			config: func(t *testing.T) *config.Config {
				t.Helper()
				cfg := defaultTestConfig(t)
				cfg.Prefix.Template = "[{{.Timestamp}}] "
				cfg.Prefix.Timestamp.Format = "%Y-%m-%d %H:%M:%S"
				return cfg
			},
			validate: func(t *testing.T, output string) {
				t.Helper()
				assert.Regexp(t, simpleTimestampPattern, output)
			},
		},
		{
			name: "user in template",
			config: func(t *testing.T) *config.Config {
				t.Helper()
				cfg := defaultTestConfig(t)
				cfg.Prefix.Template = "[{{.User}}] "
				cfg.Prefix.User.Enabled = true
				cfg.Prefix.User.Format = "username"
				return cfg
			},
			validate: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "["+currentUser.Username+"]")
			},
		},
		{
			name: "PID in template",
			config: func(t *testing.T) *config.Config {
				t.Helper()
				cfg := defaultTestConfig(t)
				cfg.Prefix.Template = "[{{.PID}}] "
				cfg.Prefix.PID.Enabled = true
				cfg.Prefix.PID.Format = "decimal"
				return cfg
			},
			validate: func(t *testing.T, output string) {
				t.Helper()
				assert.Regexp(t, `\[\d+\]`, output)
			},
		},
		{
			name: "full default template",
			config: func(t *testing.T) *config.Config {
				t.Helper()
				return defaultTestConfig(t)
			},
			validate: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, currentUser.Username)
				assert.Regexp(t, defaultTimestampPattern, output)
				assert.Contains(t, output, "[INFO]")
				// PID appears in "[user:PID]" format; use regex to avoid
				// false positives from timestamp digit collisions.
				assert.Regexp(t, `:\d+\]`, output)
			},
		},
		{
			name: "UTC timestamp",
			config: func(t *testing.T) *config.Config {
				t.Helper()
				cfg := defaultTestConfig(t)
				cfg.Prefix.Template = "[{{.Timestamp}}] "
				cfg.Prefix.Timestamp.UTC = true
				cfg.Prefix.Timestamp.Format = "%Y-%m-%dT%H:%M:%S%z"
				return cfg
			},
			validate: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "+0000")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := tt.config(t)
			output, exitCode := runPipeline(t, cfg, []string{"echo", "template test"})

			assert.Equal(t, 0, exitCode)
			assert.Contains(t, output, "template test")
			tt.validate(t, output)
		})
	}
}

func TestIntegration_LogLevelDetection(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("shell echo test not supported on Windows")
	}

	cfg := defaultTestConfig(t)
	cfg.Prefix.Template = "[{{.Level}}] "

	output, exitCode := runPipeline(t, cfg, []string{
		"sh", "-c",
		`echo "INFO: starting up"
echo "DEBUG: verbose message"
echo "WARN: something might be wrong"
echo "ERROR: something failed" >&2
echo "regular message"`,
	})

	assert.Equal(t, 0, exitCode)

	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.GreaterOrEqual(t, len(lines), 5)

	// Build a map of message content -> detected level.
	levelMap := make(map[string]string)
	for _, line := range lines {
		if strings.HasPrefix(line, "[") {
			closeBracket := strings.Index(line, "] ")
			if closeBracket > 0 {
				level := line[1:closeBracket]
				message := line[closeBracket+2:]
				levelMap[message] = level
			}
		}
	}

	assert.Equal(t, "INFO", levelMap["INFO: starting up"])
	assert.Equal(t, "DEBUG", levelMap["DEBUG: verbose message"])
	assert.Equal(t, "WARN", levelMap["WARN: something might be wrong"])
	assert.Equal(t, "ERROR", levelMap["ERROR: something failed"])
	assert.Equal(t, "INFO", levelMap["regular message"]) // default for stdout
}

func TestIntegration_JSONOutput(t *testing.T) {
	t.Parallel()

	cfg := defaultTestConfig(t)
	cfg.Output.Format = "json"

	output, exitCode := runPipeline(t, cfg, []string{"echo", "json test"})

	assert.Equal(t, 0, exitCode)

	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	require.NotEmpty(t, lines)

	for _, line := range lines {
		assert.True(t, json.Valid([]byte(line)), "output line should be valid JSON: %s", line)
	}

	var jsonData map[string]any
	err := json.Unmarshal([]byte(lines[0]), &jsonData)
	require.NoError(t, err)

	assert.Equal(t, "json test", jsonData["message"])
	assert.Equal(t, "INFO", jsonData["level"])
	assert.Contains(t, jsonData, "timestamp")
	assert.Contains(t, jsonData, "user")
	assert.Contains(t, jsonData, "pid")
}

func TestIntegration_MultilineOutput(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("shell echo test not supported on Windows")
	}

	cfg := defaultTestConfig(t)
	cfg.Prefix.Template = "[{{.Level}}] "

	output, exitCode := runPipeline(t, cfg, []string{
		"sh", "-c", "echo 'line one'; echo 'line two'; echo 'line three'",
	})

	assert.Equal(t, 0, exitCode)

	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Len(t, lines, 3)

	for _, line := range lines {
		assert.True(t, strings.HasPrefix(line, "[INFO] "),
			"each line should have its own prefix, got: %s", line)
	}
}

func TestIntegration_EmptyOutput(t *testing.T) {
	t.Parallel()

	cfg := defaultTestConfig(t)
	output, exitCode := runPipeline(t, cfg, []string{"true"})

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, strings.TrimSpace(output))
}

