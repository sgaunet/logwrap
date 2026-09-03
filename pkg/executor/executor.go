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
	"slices"
	"strings"
	"sync"
	"sync/atomic"
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
	stdoutPipe  *os.File
	stderrPipe  *os.File
	stdoutWrite *os.File // child's write end, closed by the parent after Start
	stderrWrite *os.File // child's write end, closed by the parent after Start
	commandName string   // stored for error messages
	exitCode    int
	closeWrites sync.Once
	isStarted   atomic.Bool
	isFinished  atomic.Bool
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
		if cmd.Process != nil {
			return cmd.Process.Signal(syscall.SIGTERM)
		}
		return nil
	}
	cmd.WaitDelay = gracefulStopDelay
	cmd.Stdin = os.Stdin

	// Pipes are created with os.Pipe rather than cmd.StdoutPipe/StderrPipe on
	// purpose: Wait closes the pipes returned by cmd.StdoutPipe as soon as the
	// child exits, which truncates (or drops entirely) any output still buffered
	// in the pipe when the reader has not drained it yet. Because the caller
	// reads the streams concurrently with Wait, that race silently loses output
	// from short-lived commands. Files assigned to cmd.Stdout/cmd.Stderr are not
	// owned by os/exec, so they stay readable until the reader hits EOF.
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdout pipe for %q: %w", command[0], err)
	}

	stderrRead, stderrWrite, err := os.Pipe()
	if err != nil {
		_ = stdoutRead.Close()
		_ = stdoutWrite.Close()
		cancel()
		return nil, fmt.Errorf("failed to create stderr pipe for %q: %w", command[0], err)
	}

	cmd.Stdout = stdoutWrite
	cmd.Stderr = stderrWrite

	executor := &Executor{
		cmd:         cmd,
		cancel:      cancel,
		stdoutPipe:  stdoutRead,
		stderrPipe:  stderrRead,
		stdoutWrite: stdoutWrite,
		stderrWrite: stderrWrite,
		commandName: command[0],
		exitCode:    0,
	}

	return executor, nil
}

// Start begins execution of the command.
func (e *Executor) Start() error {
	if e.isStarted.Load() {
		return appErrors.ErrExecutorStarted
	}

	if err := e.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command %q: %w", e.commandName, err)
	}

	// The child holds its own descriptors now. Drop the parent's copies of the
	// write ends so readers observe EOF once the command (and anything else
	// inheriting its streams) exits.
	e.closeWriteEnds()

	e.isStarted.Store(true)
	return nil
}

// Wait waits for the command to complete and returns any error.
func (e *Executor) Wait() error {
	if !e.isStarted.Load() {
		return appErrors.ErrExecutorNotStarted
	}

	if e.isFinished.Load() {
		return nil
	}

	err := e.cmd.Wait()

	if err != nil {
		var exitError *exec.ExitError

		switch {
		// ErrWaitDelay means WaitDelay expired while shutting the command down
		// (e.g., it ignored the SIGTERM sent on context cancellation).
		// The process itself succeeded, so treat this as a normal exit.
		case errors.Is(err, exec.ErrWaitDelay):
			e.isFinished.Store(true)
			return nil

		case errors.As(err, &exitError):
			e.exitCode = resolveExitCode(exitError)

		// Context cancellation can race with the process exiting. If the
		// process already exited, extract its real exit code instead of
		// treating context.Canceled as a generic failure.
		case errors.Is(err, context.Canceled) && e.cmd.ProcessState != nil:
			e.exitCode = e.cmd.ProcessState.ExitCode()

		default:
			e.isFinished.Store(true)
			return fmt.Errorf("command %q execution failed: %w", e.commandName, err)
		}
	}

	e.isFinished.Store(true)

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
	return e.isFinished.Load()
}

// Stop gracefully terminates the command using SIGTERM.
// Context cancellation triggers the custom Cancel function (SIGTERM).
// If the process doesn't exit within WaitDelay, Go escalates to SIGKILL.
func (e *Executor) Stop() error {
	if !e.isStarted.Load() || e.isFinished.Load() {
		return nil
	}

	e.cancel()
	return nil
}

// Kill forcefully terminates the command with SIGKILL.
func (e *Executor) Kill() error {
	if !e.isStarted.Load() || e.isFinished.Load() {
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
	e.closeWriteEnds()
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

// closeWriteEnds releases the parent's copies of the pipe write ends.
// Safe to call more than once; subsequent closes are no-ops.
func (e *Executor) closeWriteEnds() {
	e.closeWrites.Do(func() {
		if e.stdoutWrite != nil {
			_ = e.stdoutWrite.Close()
		}
		if e.stderrWrite != nil {
			_ = e.stderrWrite.Close()
		}
	})
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
	// Check the raw path before filepath.Clean, which normalizes away ".."
	// in absolute paths (e.g., "/../etc/passwd" → "/etc/passwd").
	if slices.Contains(strings.Split(command, string(filepath.Separator)), "..") {
		return appErrors.ErrCommandPathTraversal
	}
	cleaned := filepath.Clean(command)
	if slices.Contains(strings.Split(cleaned, string(filepath.Separator)), "..") {
		return appErrors.ErrCommandPathTraversal
	}
	return nil
}
