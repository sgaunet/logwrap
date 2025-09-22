package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

type Executor struct {
	cmd         *exec.Cmd
	ctx         context.Context
	cancel      context.CancelFunc
	stdoutPipe  io.ReadCloser
	stderrPipe  io.ReadCloser
	signalChan  chan os.Signal
	exitCode    int
	isStarted   bool
	isFinished  bool
}

func New(command []string) (*Executor, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("command cannot be empty")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		stdoutPipe.Close()
		cancel()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	executor := &Executor{
		cmd:        cmd,
		ctx:        ctx,
		cancel:     cancel,
		stdoutPipe: stdoutPipe,
		stderrPipe: stderrPipe,
		signalChan: make(chan os.Signal, 1),
		exitCode:   0,
	}

	executor.setupSignalHandling()

	return executor, nil
}

func (e *Executor) setupSignalHandling() {
	signal.Notify(e.signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		for sig := range e.signalChan {
			if e.isStarted && !e.isFinished {
				if e.cmd.Process != nil {
					e.cmd.Process.Signal(sig)
				}
			}
		}
	}()
}

func (e *Executor) Start() error {
	if e.isStarted {
		return fmt.Errorf("executor already started")
	}

	if err := e.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	e.isStarted = true
	return nil
}

func (e *Executor) Wait() error {
	if !e.isStarted {
		return fmt.Errorf("executor not started")
	}

	if e.isFinished {
		return nil
	}

	err := e.cmd.Wait()
	e.isFinished = true

	signal.Stop(e.signalChan)
	close(e.signalChan)

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			e.exitCode = exitError.ExitCode()
		} else {
			return fmt.Errorf("command execution failed: %w", err)
		}
	}

	return nil
}

func (e *Executor) GetStreams() (stdout, stderr io.Reader) {
	return e.stdoutPipe, e.stderrPipe
}

func (e *Executor) GetExitCode() int {
	return e.exitCode
}

func (e *Executor) IsFinished() bool {
	return e.isFinished
}

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

func (e *Executor) Cleanup() {
	if e.stdoutPipe != nil {
		e.stdoutPipe.Close()
	}
	if e.stderrPipe != nil {
		e.stderrPipe.Close()
	}
	if e.cancel != nil {
		e.cancel()
	}
}