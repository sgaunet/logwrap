// Package executor provides secure command execution with stream capture.
//
// The executor spawns processes and captures their stdout/stderr streams
// for real-time processing by the processor package. It handles process
// lifecycle management, signal forwarding, and exit code preservation.
//
// # Security Model
//
// The executor provides minimal security validation. See [validateCommand]
// for details on what is and is not validated. Users must validate commands
// before passing them to logwrap.
//
// # Process Lifecycle
//
//  1. Validate command path (path traversal check)
//  2. Create [exec.Cmd] with context for cancellation
//  3. Set up stdout/stderr pipes
//  4. Start process via [Executor.Start]
//  5. Caller reads pipes via [Executor.GetStreams]
//  6. Wait for completion via [Executor.Wait]
//  7. Release resources via [Executor.Cleanup]
//
// # Signal Handling
//
// When the executor's context is cancelled (via [Executor.Stop]),
// the child process receives SIGTERM. If it doesn't exit within
// [gracefulStopDelay], Go's stdlib escalates to SIGKILL.
//
// # Exit Code Preservation
//
// The executor preserves the exact exit code from the wrapped command:
//   - Success (0) → returns 0
//   - Failure (N) → returns N
//   - Signal termination → returns 128 + signal number
//
// Non-exit errors (e.g., command not found) are returned as Go errors.
package executor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	appErrors "github.com/sgaunet/logwrap/pkg/apperrors"
)

const (
	// gracefulStopDelay is the time to wait after sending SIGTERM (via context
	// cancellation) before the Go runtime escalates to SIGKILL.
	gracefulStopDelay = 5 * time.Second

	// signalExitCodeBase is the UNIX convention base for signal exit codes (128 + signal number).
	signalExitCodeBase = 128
)

// Executor manages command execution with stream capture and signal handling.
type Executor struct {
	cmd         *exec.Cmd
	cancel      context.CancelFunc
	stdoutPipe  io.ReadCloser
	stderrPipe  io.ReadCloser
	commandName string // stored for error messages
	exitCode    int
	isStarted   bool
	isFinished  bool
}

// New creates a new Executor instance for the given command.
func New(command []string) (*Executor, error) {
	if len(command) == 0 {
		return nil, appErrors.ErrCommandEmpty
	}

	if err := validateCommand(command[0]); err != nil {
		return nil, fmt.Errorf("invalid command %q: %w", command[0], err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, command[0], command[1:]...) // #nosec G204 - command is validated above

	// Send SIGTERM (not SIGKILL) when the context is cancelled.
	// If the process doesn't exit within WaitDelay, Go escalates to SIGKILL.
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = gracefulStopDelay
	cmd.Stdin = os.Stdin

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdout pipe for %q: %w", command[0], err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = stdoutPipe.Close()
		cancel()
		return nil, fmt.Errorf("failed to create stderr pipe for %q: %w", command[0], err)
	}

	executor := &Executor{
		cmd:         cmd,
		cancel:      cancel,
		stdoutPipe:  stdoutPipe,
		stderrPipe:  stderrPipe,
		commandName: command[0],
		exitCode:    0,
	}

	return executor, nil
}

// Start begins execution of the command.
func (e *Executor) Start() error {
	if e.isStarted {
		return appErrors.ErrExecutorStarted
	}

	if err := e.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command %q: %w", e.commandName, err)
	}

	e.isStarted = true
	return nil
}

// Wait waits for the command to complete and returns any error.
func (e *Executor) Wait() error {
	if !e.isStarted {
		return appErrors.ErrExecutorNotStarted
	}

	if e.isFinished {
		return nil
	}

	err := e.cmd.Wait()
	e.isFinished = true

	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			e.exitCode = resolveExitCode(exitError)
		} else {
			return fmt.Errorf("command %q execution failed: %w", e.commandName, err)
		}
	}

	return nil
}

// resolveExitCode extracts the exit code from an ExitError.
// When the process was killed by a signal, ExitCode() returns -1;
// in that case, compute 128 + signal number per UNIX convention.
func resolveExitCode(exitError *exec.ExitError) int {
	code := exitError.ExitCode()
	if code != -1 {
		return code
	}
	if status, ok := exitError.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return signalExitCodeBase + int(status.Signal())
	}
	return code
}

// GetStreams returns the stdout and stderr readers for the command.
func (e *Executor) GetStreams() (io.Reader, io.Reader) {
	return e.stdoutPipe, e.stderrPipe
}

// GetExitCode returns the exit code of the finished command.
func (e *Executor) GetExitCode() int {
	return e.exitCode
}

// IsFinished returns true if the command has finished execution.
func (e *Executor) IsFinished() bool {
	return e.isFinished
}

// Stop gracefully terminates the command using SIGTERM.
// Context cancellation triggers the custom Cancel function (SIGTERM).
// If the process doesn't exit within WaitDelay, Go escalates to SIGKILL.
func (e *Executor) Stop() error {
	if !e.isStarted || e.isFinished {
		return nil
	}

	e.cancel()
	return nil
}

// Kill forcefully terminates the command with SIGKILL.
func (e *Executor) Kill() error {
	if !e.isStarted || e.isFinished {
		return nil
	}

	if e.cmd.Process != nil {
		if err := e.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process %q: %w", e.commandName, err)
		}
	}

	e.cancel()
	return nil
}

// Cleanup closes pipes and cancels context to release resources.
func (e *Executor) Cleanup() {
	if e.stdoutPipe != nil {
		_ = e.stdoutPipe.Close()
	}
	if e.stderrPipe != nil {
		_ = e.stderrPipe.Close()
	}
	if e.cancel != nil {
		e.cancel()
	}
}

// validateCommand performs minimal security validation on the command path.
//
// Security Model:
//   - Prevents path traversal attacks using ".." in command paths
//   - Does NOT prevent command injection via arguments
//   - Does NOT restrict access to system binaries
//   - Does NOT filter shell metacharacters
//
// Commands run with the current user's privileges. Callers are responsible
// for validating commands before passing them to logwrap.
func validateCommand(command string) error {
	cleaned := filepath.Clean(command)
	if strings.Contains(cleaned, "..") {
		return appErrors.ErrCommandPathTraversal
	}

	return nil
}