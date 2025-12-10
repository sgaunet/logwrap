// Package executor provides command execution functionality with stream capture.
package executor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	appErrors "github.com/sgaunet/logwrap/pkg/apperrors"
)

// Executor manages command execution with stream capture and signal handling.
type Executor struct {
	cmd         *exec.Cmd
	cancel      context.CancelFunc
	stdoutPipe  io.ReadCloser
	stderrPipe  io.ReadCloser
	signalChan  chan os.Signal
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
		return nil, fmt.Errorf("invalid command: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, command[0], command[1:]...) // #nosec G204 - command is validated above

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = stdoutPipe.Close()
		cancel()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	executor := &Executor{
		cmd:        cmd,
		cancel:     cancel,
		stdoutPipe: stdoutPipe,
		stderrPipe: stderrPipe,
		signalChan: make(chan os.Signal, 1),
		exitCode:   0,
	}

	executor.setupSignalHandling()

	return executor, nil
}

// Start begins execution of the command.
func (e *Executor) Start() error {
	if e.isStarted {
		return appErrors.ErrExecutorStarted
	}

	if err := e.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
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

	signal.Stop(e.signalChan)
	close(e.signalChan)

	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			e.exitCode = exitError.ExitCode()
		} else {
			return fmt.Errorf("command execution failed: %w", err)
		}
	}

	return nil
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
func (e *Executor) Stop() error {
	if !e.isStarted || e.isFinished {
		return nil
	}

	e.cancel()

	if e.cmd.Process != nil {
		if err := e.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			return fmt.Errorf("failed to send SIGTERM: %w", err)
		}
	}

	return nil
}

// Kill forcefully terminates the command.
func (e *Executor) Kill() error {
	if !e.isStarted || e.isFinished {
		return nil
	}

	e.cancel()

	if e.cmd.Process != nil {
		if err := e.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}

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

func (e *Executor) setupSignalHandling() {
	signal.Notify(e.signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		for sig := range e.signalChan {
			if e.isStarted && !e.isFinished {
				if e.cmd.Process != nil {
					_ = e.cmd.Process.Signal(sig)
				}
			}
		}
	}()
}

func validateCommand(command string) error {
	// Prevent path traversal
	cleaned := filepath.Clean(command)
	if strings.Contains(cleaned, "..") {
		return appErrors.ErrCommandPathTraversal
	}

	// For security, we could add more validations here
	// such as checking against an allowlist of commands
	// For now, we just prevent obvious path traversal

	return nil
}