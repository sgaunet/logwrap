package executor_test

import (
	"bufio"
	"fmt"
	"io"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/sgaunet/logwrap/pkg/apperrors"
	"github.com/sgaunet/logwrap/pkg/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command []string
	}{
		{
			name:    "simple command",
			command: []string{"echo", "hello"},
		},
		{
			name:    "command with multiple args",
			command: []string{"echo", "hello", "world"},
		},
		{
			name:    "command with flags",
			command: []string{"ls", "-la"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			exec, err := executor.New(tt.command)
			require.NoError(t, err)
			require.NotNil(t, exec)

			t.Cleanup(func() {
				exec.Cleanup()
			})

			// Verify streams are available
			stdout, stderr := exec.GetStreams()
			assert.NotNil(t, stdout)
			assert.NotNil(t, stderr)

			// Verify initial state
			assert.False(t, exec.IsFinished())
			assert.Equal(t, 0, exec.GetExitCode())
		})
	}
}

func TestNew_EmptyCommand(t *testing.T) {
	t.Parallel()

	exec, err := executor.New([]string{})
	assert.Error(t, err)
	assert.Nil(t, exec)
	assert.ErrorIs(t, err, apperrors.ErrCommandEmpty)
}

func TestNew_PathTraversal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command []string
	}{
		{
			name:    "simple path traversal",
			command: []string{"../../../etc/passwd"},
		},
		{
			name:    "path traversal with args",
			command: []string{"../../../bin/sh", "-c", "echo test"},
		},
		{
			name:    "relative path with traversal",
			command: []string{"./../../secret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			exec, err := executor.New(tt.command)
			assert.Error(t, err)
			assert.Nil(t, exec)
			assert.ErrorIs(t, err, apperrors.ErrCommandPathTraversal)
		})
	}
}

func TestExecutor_StartAndWait_Success(t *testing.T) {
	t.Parallel()

	exec, err := executor.New([]string{"echo", "hello world"})
	require.NoError(t, err)
	require.NotNil(t, exec)

	t.Cleanup(func() {
		exec.Cleanup()
	})

	// Start the command
	err = exec.Start()
	assert.NoError(t, err)

	// Read output
	stdout, _ := exec.GetStreams()
	output, err := io.ReadAll(stdout)
	assert.NoError(t, err)
	assert.Contains(t, string(output), "hello world")

	// Wait for completion
	err = exec.Wait()
	assert.NoError(t, err)

	// Verify final state
	assert.True(t, exec.IsFinished())
	assert.Equal(t, 0, exec.GetExitCode())
}

func TestExecutor_StartTwice(t *testing.T) {
	t.Parallel()

	exec, err := executor.New([]string{"echo", "test"})
	require.NoError(t, err)
	require.NotNil(t, exec)

	t.Cleanup(func() {
		exec.Cleanup()
	})

	// Start once - should succeed
	err = exec.Start()
	assert.NoError(t, err)

	// Start again - should fail
	err = exec.Start()
	assert.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrExecutorStarted)

	// Clean up
	_ = exec.Wait()
}

func TestExecutor_WaitWithoutStart(t *testing.T) {
	t.Parallel()

	exec, err := executor.New([]string{"echo", "test"})
	require.NoError(t, err)
	require.NotNil(t, exec)

	t.Cleanup(func() {
		exec.Cleanup()
	})

	// Wait without starting should fail
	err = exec.Wait()
	assert.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrExecutorNotStarted)
}

func TestExecutor_NonZeroExitCode(t *testing.T) {
	t.Parallel()

	// Use a command that will fail
	var command []string
	if runtime.GOOS == "windows" {
		command = []string{"cmd", "/c", "exit 42"}
	} else {
		command = []string{"sh", "-c", "exit 42"}
	}

	exec, err := executor.New(command)
	require.NoError(t, err)
	require.NotNil(t, exec)

	t.Cleanup(func() {
		exec.Cleanup()
	})

	err = exec.Start()
	require.NoError(t, err)

	err = exec.Wait()
	assert.NoError(t, err) // Wait should not return error for non-zero exit

	assert.True(t, exec.IsFinished())
	assert.Equal(t, 42, exec.GetExitCode())
}

func TestExecutor_InvalidCommand(t *testing.T) {
	t.Parallel()

	exec, err := executor.New([]string{"nonexistent-command-12345"})
	require.NoError(t, err)
	require.NotNil(t, exec)

	t.Cleanup(func() {
		exec.Cleanup()
	})

	// Start should fail for invalid command
	err = exec.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start command")
}

func TestExecutor_StderrOutput(t *testing.T) {
	t.Parallel()

	var command []string
	if runtime.GOOS == "windows" {
		command = []string{"cmd", "/c", "echo error message 1>&2"}
	} else {
		command = []string{"sh", "-c", "echo 'error message' >&2"}
	}

	exec, err := executor.New(command)
	require.NoError(t, err)
	require.NotNil(t, exec)

	t.Cleanup(func() {
		exec.Cleanup()
	})

	err = exec.Start()
	require.NoError(t, err)

	// Read stderr
	_, stderr := exec.GetStreams()
	errorOutput, err := io.ReadAll(stderr)
	assert.NoError(t, err)
	assert.Contains(t, string(errorOutput), "error message")

	err = exec.Wait()
	assert.NoError(t, err)
}

func TestExecutor_BothStreams(t *testing.T) {
	t.Parallel()

	var command []string
	if runtime.GOOS == "windows" {
		command = []string{"cmd", "/c", "echo stdout message && echo stderr message 1>&2"}
	} else {
		command = []string{"sh", "-c", "echo 'stdout message'; echo 'stderr message' >&2"}
	}

	exec, err := executor.New(command)
	require.NoError(t, err)
	require.NotNil(t, exec)

	t.Cleanup(func() {
		exec.Cleanup()
	})

	err = exec.Start()
	require.NoError(t, err)

	stdout, stderr := exec.GetStreams()

	// Read both streams concurrently
	stdoutChan := make(chan string, 1)
	stderrChan := make(chan string, 1)

	go func() {
		output, _ := io.ReadAll(stdout)
		stdoutChan <- string(output)
	}()

	go func() {
		output, _ := io.ReadAll(stderr)
		stderrChan <- string(output)
	}()

	err = exec.Wait()
	assert.NoError(t, err)

	// Check outputs
	stdoutContent := <-stdoutChan
	stderrContent := <-stderrChan

	assert.Contains(t, stdoutContent, "stdout message")
	assert.Contains(t, stderrContent, "stderr message")
}

func TestExecutor_Stop(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Signal handling tests not reliable on Windows")
	}

	t.Parallel()

	// Command that runs for a while
	exec, err := executor.New([]string{"sleep", "10"})
	require.NoError(t, err)
	require.NotNil(t, exec)

	t.Cleanup(func() {
		exec.Cleanup()
	})

	err = exec.Start()
	require.NoError(t, err)

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Stop the command
	err = exec.Stop()
	assert.NoError(t, err)

	// Wait should complete quickly
	start := time.Now()
	_ = exec.Wait()
	duration := time.Since(start)

	assert.True(t, exec.IsFinished())
	assert.Less(t, duration, 2*time.Second, "Command should have been terminated quickly")
}

func TestExecutor_Kill(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Signal handling tests not reliable on Windows")
	}

	t.Parallel()

	// Command that runs for a while
	exec, err := executor.New([]string{"sleep", "10"})
	require.NoError(t, err)
	require.NotNil(t, exec)

	t.Cleanup(func() {
		exec.Cleanup()
	})

	err = exec.Start()
	require.NoError(t, err)

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Kill the command
	err = exec.Kill()
	assert.NoError(t, err)

	// Wait should complete quickly
	start := time.Now()
	_ = exec.Wait()
	duration := time.Since(start)

	assert.True(t, exec.IsFinished())
	assert.Less(t, duration, 2*time.Second, "Command should have been killed quickly")
}

func TestExecutor_StopAlreadyFinished(t *testing.T) {
	t.Parallel()

	exec, err := executor.New([]string{"echo", "test"})
	require.NoError(t, err)
	require.NotNil(t, exec)

	t.Cleanup(func() {
		exec.Cleanup()
	})

	err = exec.Start()
	require.NoError(t, err)

	err = exec.Wait()
	require.NoError(t, err)

	// Stop after completion should not error
	err = exec.Stop()
	assert.NoError(t, err)

	// Kill after completion should not error
	err = exec.Kill()
	assert.NoError(t, err)
}

func TestExecutor_StopNotStarted(t *testing.T) {
	t.Parallel()

	exec, err := executor.New([]string{"echo", "test"})
	require.NoError(t, err)
	require.NotNil(t, exec)

	t.Cleanup(func() {
		exec.Cleanup()
	})

	// Stop without starting should not error
	err = exec.Stop()
	assert.NoError(t, err)

	// Kill without starting should not error
	err = exec.Kill()
	assert.NoError(t, err)
}

func TestExecutor_LargeOutput(t *testing.T) {
	t.Parallel()

	// Command that produces large output
	var command []string
	if runtime.GOOS == "windows" {
		// Windows doesn't have seq, use a simple loop
		command = []string{"cmd", "/c", "for /l %i in (1,1,1000) do @echo Line %i"}
	} else {
		command = []string{"seq", "1", "1000"}
	}

	exec, err := executor.New(command)
	require.NoError(t, err)
	require.NotNil(t, exec)

	t.Cleanup(func() {
		exec.Cleanup()
	})

	err = exec.Start()
	require.NoError(t, err)

	stdout, _ := exec.GetStreams()
	scanner := bufio.NewScanner(stdout)

	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}

	err = exec.Wait()
	assert.NoError(t, err)

	// Should have read many lines
	assert.GreaterOrEqual(t, lineCount, 100, "Should have read substantial output")
}

func TestExecutor_Cleanup(t *testing.T) {
	t.Parallel()

	exec, err := executor.New([]string{"echo", "test"})
	require.NoError(t, err)
	require.NotNil(t, exec)

	// Cleanup should not panic
	exec.Cleanup()

	// Multiple cleanups should not panic
	exec.Cleanup()
	exec.Cleanup()
}

func TestExecutor_MultipleWaits(t *testing.T) {
	t.Parallel()

	exec, err := executor.New([]string{"echo", "test"})
	require.NoError(t, err)
	require.NotNil(t, exec)

	t.Cleanup(func() {
		exec.Cleanup()
	})

	err = exec.Start()
	require.NoError(t, err)

	// First wait
	err = exec.Wait()
	assert.NoError(t, err)
	assert.True(t, exec.IsFinished())

	// Second wait should also succeed
	err = exec.Wait()
	assert.NoError(t, err)
	assert.True(t, exec.IsFinished())
}

func TestExecutor_Integration(t *testing.T) {
	t.Parallel()

	// Integration test with a realistic command
	var command []string
	if runtime.GOOS == "windows" {
		command = []string{"cmd", "/c", "echo Starting && timeout 1 && echo Finished"}
	} else {
		command = []string{"sh", "-c", "echo 'Starting'; sleep 0.1; echo 'Finished'"}
	}

	exec, err := executor.New(command)
	require.NoError(t, err)
	require.NotNil(t, exec)

	t.Cleanup(func() {
		exec.Cleanup()
	})

	// Start and capture all output
	err = exec.Start()
	require.NoError(t, err)

	stdout, stderr := exec.GetStreams()

	var stdoutContent, stderrContent string
	var wg sync.WaitGroup

	// Read stdout
	wg.Add(1)
	go func() {
		defer wg.Done()
		output, _ := io.ReadAll(stdout)
		stdoutContent = string(output)
	}()

	// Read stderr
	wg.Add(1)
	go func() {
		defer wg.Done()
		output, _ := io.ReadAll(stderr)
		stderrContent = string(output)
	}()

	// Wait for completion
	err = exec.Wait()
	assert.NoError(t, err)

	// Wait for all readers to finish
	wg.Wait()

	// Verify results
	assert.True(t, exec.IsFinished())
	assert.Equal(t, 0, exec.GetExitCode())
	assert.Contains(t, stdoutContent, "Starting")
	assert.Contains(t, stdoutContent, "Finished")
	assert.Empty(t, stderrContent)
}

func TestExecutor_ContextIntegration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Context cancellation tests not reliable on Windows")
	}

	t.Parallel()

	// Test that we can create and manage executors concurrently
	const numExecutors = 5
	executors := make([]*executor.Executor, numExecutors)

	// Create multiple executors
	for i := 0; i < numExecutors; i++ {
		exec, err := executor.New([]string{"echo", fmt.Sprintf("executor-%d", i)})
		require.NoError(t, err)
		executors[i] = exec
	}

	// Start all executors
	for i, exec := range executors {
		err := exec.Start()
		require.NoError(t, err, "Failed to start executor %d", i)
	}

	// Wait for all to complete
	for i, exec := range executors {
		err := exec.Wait()
		assert.NoError(t, err, "Executor %d failed", i)
		assert.True(t, exec.IsFinished(), "Executor %d not finished", i)
		assert.Equal(t, 0, exec.GetExitCode(), "Executor %d bad exit code", i)
	}

	// Cleanup all
	for _, exec := range executors {
		exec.Cleanup()
	}
}

func TestExecutor_StateTransitions(t *testing.T) {
	t.Parallel()

	exec, err := executor.New([]string{"echo", "state-test"})
	require.NoError(t, err)
	require.NotNil(t, exec)

	t.Cleanup(func() {
		exec.Cleanup()
	})

	// Initial state
	assert.False(t, exec.IsFinished())
	assert.Equal(t, 0, exec.GetExitCode())

	// After start
	err = exec.Start()
	require.NoError(t, err)
	assert.False(t, exec.IsFinished()) // May or may not be finished yet

	// After wait
	err = exec.Wait()
	assert.NoError(t, err)
	assert.True(t, exec.IsFinished())
	assert.Equal(t, 0, exec.GetExitCode())

	// State should remain consistent
	assert.True(t, exec.IsFinished())
	assert.Equal(t, 0, exec.GetExitCode())
}