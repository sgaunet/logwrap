// Package processor provides real-time stream processing for command output.
//
// The processor captures stdout and stderr from executed commands and processes
// them line-by-line in real-time using concurrent goroutines. Each line is
// passed through a [Formatter] before being written to the output.
//
// # Architecture
//
// The processor uses a pipeline pattern:
//  1. Accept stdout/stderr readers from the executor
//  2. Launch one goroutine per stream for concurrent processing
//  3. Use [bufio.Scanner] for efficient line-by-line reading
//  4. Pass each line to the formatter with its stream type
//  5. Write formatted output immediately (no buffering)
//
// # Concurrency Model
//
// Two goroutines run concurrently, one per stream (stdout and stderr).
// A [sync.WaitGroup] coordinates completion. Errors from each goroutine
// are collected in a mutex-protected slice. Context cancellation is
// checked between lines for responsive shutdown.
//
// # Buffer Management
//
// Scanner buffer sizes:
//   - Initial: 64KB (balances memory usage vs syscall overhead)
//   - Maximum: 1MB (prevents memory exhaustion on very long lines)
//
// Lines exceeding 1MB will cause a scanner error for that stream.
//
// # Error Handling
//
// EOF and closed-pipe errors are expected during normal shutdown and
// handled gracefully. Scanner errors are collected and returned as a
// combined error after both streams complete.
package processor

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	pkgerrors "github.com/sgaunet/logwrap/pkg/apperrors"
)

// StreamType represents the type of stream (stdout or stderr).
type StreamType int

const (
	// StreamStdout represents standard output stream.
	StreamStdout StreamType = iota
	// StreamStderr represents standard error stream.
	StreamStderr
)

func (s StreamType) String() string {
	switch s {
	case StreamStdout:
		return "stdout"
	case StreamStderr:
		return "stderr"
	default:
		return "unknown"
	}
}

// Formatter defines the interface for formatting log lines.
type Formatter interface {
	FormatLine(line string, streamType StreamType) string
}

// Processor handles real-time processing of command output streams.
type Processor struct {
	formatter Formatter
	output    io.Writer
	wg        sync.WaitGroup
	errors    []error
	mutex     sync.Mutex
	cancel    context.CancelFunc
	stopOnce  sync.Once
}

// Option defines a function that configures a Processor.
type Option func(*Processor)

// WithContext sets a cancellable context for the processor.
// The cancel function is stored and called when Stop() is invoked.
func WithContext(ctx context.Context) Option {
	return func(p *Processor) {
		_, p.cancel = context.WithCancel(ctx) //nolint:gosec // G118 - cancel is called via Stop()
	}
}

// New creates a new Processor with the given formatter and output writer.
func New(formatter Formatter, output io.Writer, opts ...Option) *Processor {
	p := &Processor{
		formatter: formatter,
		output:    output,
		cancel:    func() {},
		errors:    make([]error, 0),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// ProcessStreams processes both stdout and stderr streams concurrently.
func (p *Processor) ProcessStreams(ctx context.Context, stdout, stderr io.Reader) error {
	if stdout == nil || stderr == nil {
		return pkgerrors.ErrReadersNil
	}

	const streamCount = 2
	p.wg.Add(streamCount)

	go func() {
		defer p.wg.Done()
		if err := p.processStream(ctx, stdout, StreamStdout); err != nil {
			p.addError(fmt.Errorf("stdout processing error: %w", err))
		}
	}()

	go func() {
		defer p.wg.Done()
		if err := p.processStream(ctx, stderr, StreamStderr); err != nil {
			p.addError(fmt.Errorf("stderr processing error: %w", err))
		}
	}()

	p.wg.Wait()

	if len(p.errors) > 0 {
		return fmt.Errorf("%w: %v", pkgerrors.ErrProcessingErrors, p.errors)
	}

	return nil
}

// Stop cancels the processor context to stop stream processing.
// Safe to call multiple times - subsequent calls are no-ops.
func (p *Processor) Stop() {
	p.stopOnce.Do(func() {
		if p.cancel != nil {
			p.cancel()
		}
	})
}

// Wait waits for stream processing to complete with a timeout.
func (p *Processor) Wait(timeout time.Duration) error {
	done := make(chan struct{})

	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		p.Stop()
		return fmt.Errorf("%w after %v", pkgerrors.ErrProcessorTimeout, timeout)
	}
}

// GetErrors returns a copy of all processing errors that occurred.
func (p *Processor) GetErrors() []error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	errors := make([]error, len(p.errors))
	copy(errors, p.errors)
	return errors
}

func (p *Processor) processStream(ctx context.Context, stream io.Reader, streamType StreamType) error {
	scanner := bufio.NewScanner(stream)

	const (
		bufferSize     = 64 * 1024
		maxScannerSize = 1024 * 1024
	)

	buf := make([]byte, 0, bufferSize)
	scanner.Buffer(buf, maxScannerSize)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		line := scanner.Text()
		formattedLine := p.formatter.FormatLine(line, streamType)

		if _, err := p.output.Write([]byte(formattedLine + "\n")); err != nil {
			return fmt.Errorf("failed to write to output: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		// Handle expected errors during stream closure
		if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
			return nil
		}
		// Check for closed pipe error (PathError wrapping)
		var pathErr *os.PathError
		if errors.As(err, &pathErr) && errors.Is(pathErr.Err, os.ErrClosed) {
			return nil
		}
		return fmt.Errorf("scanner error for %s: %w", streamType.String(), err)
	}

	return nil
}

func (p *Processor) addError(err error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.errors = append(p.errors, err)
}