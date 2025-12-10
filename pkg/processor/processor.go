// Package processor provides real-time stream processing functionality.
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

// WithContext sets a custom context for the processor.
func WithContext(ctx context.Context) Option {
	return func(p *Processor) {
		_, p.cancel = context.WithCancel(ctx)
	}
}

// New creates a new Processor with the given formatter and output writer.
func New(formatter Formatter, output io.Writer, opts ...Option) *Processor {
	_, cancel := context.WithCancel(context.Background())

	p := &Processor{
		formatter: formatter,
		output:    output,
		cancel:    cancel,
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