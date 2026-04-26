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
// combined error after both streams complete. Lines exceeding the
// maximum buffer size (1MB) cause [bufio.ErrTooLong], which is
// returned with a descriptive message including the byte limit.
//
// # Performance Characteristics
//
// Approximate throughput (Apple M2 Max, benchFormatter):
//   - 1000 lines of typical log output: ~325 MB/s
//   - Short lines (100B): ~335 MB/s
//   - Medium lines (1KB): ~1.1 GB/s
//   - Long lines (10KB+): ~2-4 GB/s (I/O bound)
//
// Run BenchmarkProcessStream_* in benchmark_test.go to reproduce.
//
// Bottlenecks:
//   - Small buffers (<32KB) increase syscall overhead
//   - Lines >1MB cause scanner failure (bufio.ErrTooLong)
//   - Formatter overhead per line depends on template complexity
//
// For high-volume scenarios (>100k lines/sec), use simpler templates
// and disable colors if not needed.
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

// LineFilter is an optional filter that decides whether a raw line should be
// processed. If ShouldInclude returns false, the line is silently dropped
// before formatting.
type LineFilter interface {
	ShouldInclude(line string) bool
}

// Processor handles real-time processing of command output streams.
type Processor struct {
	formatter  Formatter
	filter     LineFilter
	output     io.Writer
	wg         sync.WaitGroup
	errors     []error
	mutex      sync.Mutex
	parentDone <-chan struct{} // closed when parent context is cancelled; nil if no WithContext
	stopCh     chan struct{}
	readers    []io.Reader // stored so Stop() can close them to unblock scanners
	stopOnce   sync.Once
}

// Option defines a function that configures a Processor.
type Option func(*Processor)

// WithContext enables context-based cancellation for the processor.
// When Stop() is called or the parent context is cancelled, it signals
// ProcessStreams to cancel processing. No goroutines are created until
// ProcessStreams is called, so it is safe to create a processor with
// WithContext and never call ProcessStreams.
func WithContext(ctx context.Context) Option {
	return func(p *Processor) {
		p.parentDone = ctx.Done()
		p.stopCh = make(chan struct{})
	}
}

// WithFilter sets a line filter that is checked before formatting.
// Lines rejected by the filter are silently dropped.
func WithFilter(f LineFilter) Option {
	return func(p *Processor) {
		p.filter = f
	}
}

// New creates a new Processor with the given formatter and output writer.
func New(formatter Formatter, output io.Writer, opts ...Option) *Processor {
	p := &Processor{
		formatter: formatter,
		output:    output,
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

	p.mutex.Lock()
	p.readers = []io.Reader{stdout, stderr}
	p.mutex.Unlock()

	if p.stopCh != nil {
		var cancel context.CancelFunc
		ctx, cancel = p.setupCancellation(ctx)
		defer cancel()
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

	// Clear reader references so Stop() won't close them — the executor
	// owns these pipes and will close them via Cleanup().
	p.mutex.Lock()
	p.readers = nil
	p.mutex.Unlock()

	p.Stop() // Clean up cancellation goroutines

	if errs := p.GetErrors(); len(errs) > 0 {
		return fmt.Errorf("%w: %v", pkgerrors.ErrProcessingErrors, errs)
	}

	return nil
}

// Stop signals the processor to stop stream processing.
// Safe to call multiple times - subsequent calls are no-ops.
// If the readers implement io.Closer, they are closed to unblock
// any in-progress scanner.Scan() calls.
func (p *Processor) Stop() {
	p.stopOnce.Do(func() {
		if p.stopCh != nil {
			close(p.stopCh)
		}

		p.mutex.Lock()
		readers := p.readers
		p.mutex.Unlock()

		for _, r := range readers {
			if c, ok := r.(io.Closer); ok {
				_ = c.Close()
			}
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
		<-done
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

// setupCancellation wires stopCh and parent context into the given ctx.
// Returns the enhanced ctx and a cleanup function that must be deferred.
func (p *Processor) setupCancellation(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, mergedCancel := context.WithCancel(ctx) //nolint:gosec // G118 - cancel is returned to caller

	go func() {
		select {
		case <-p.stopCh:
			mergedCancel()
		case <-ctx.Done():
		}
	}()

	if p.parentDone != nil {
		go func() {
			select {
			case <-p.parentDone:
				p.Stop()
			case <-p.stopCh:
			}
		}()
	}

	return ctx, mergedCancel
}

// processStream reads lines from a single stream using [bufio.Scanner].
//
// Scanner buffer configuration:
//   - Initial buffer: 64KB, allocated up front via scanner.Buffer
//   - Maximum buffer: 1MB, the largest single line the scanner will accept
//
// If a line exceeds 1MB, the scanner returns [bufio.ErrTooLong] which is
// wrapped with the byte limit for diagnostics. EOF and closed-pipe errors
// are expected during normal process shutdown and return nil.
// Context cancellation is checked between lines for responsive shutdown.
func (p *Processor) processStream(ctx context.Context, stream io.Reader, streamType StreamType) error {
	scanner := bufio.NewScanner(stream)

	const (
		// bufferSize is the initial scanner buffer allocation (64KB).
		//
		// Most log lines are well under 1KB, so 64KB handles many lines per read.
		// Benchmarks show diminishing throughput returns above 64KB:
		//   32KB  → ~300 MB/s
		//   64KB  → ~325 MB/s (chosen)
		//   128KB → ~330 MB/s
		//
		// See BenchmarkProcessStream_LineVolume in benchmark_test.go.
		bufferSize = 64 * 1024

		// maxScannerSize is the maximum line size the scanner will accept (1MB).
		//
		// This prevents memory exhaustion from pathological input (e.g. a single
		// multi-megabyte line). Lines exceeding this limit cause bufio.ErrTooLong.
		//
		// 1MB is a reasonable upper bound for text-based log output. Lines this
		// large are rare in practice (binary dumps or very deep stack traces).
		// If exceeded, consider pre-processing with split(1) or similar tools.
		//
		// Buffer sizes are currently hardcoded. If your use case requires
		// different limits, file an issue.
		maxScannerSize = 1024 * 1024
	)

	buf := make([]byte, 0, bufferSize)
	scanner.Buffer(buf, maxScannerSize)

	for scanner.Scan() {
		line := scanner.Text()

		if p.filter != nil && !p.filter.ShouldInclude(line) {
			continue
		}

		formattedLine := p.formatter.FormatLine(line, streamType)

		if _, err := p.output.Write([]byte(formattedLine + "\n")); err != nil {
			return fmt.Errorf("failed to write to output: %w", err)
		}

		// Check for context cancellation after writing the line, not before,
		// so that already-scanned lines are never silently dropped.
		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}

	if err := scanner.Err(); err != nil {
		if isExpectedStreamError(err) {
			return nil
		}
		// Handle oversized lines explicitly with actionable diagnostics
		if errors.Is(err, bufio.ErrTooLong) {
			return fmt.Errorf("line exceeds maximum buffer size (%d bytes) for %s: %w",
				maxScannerSize, streamType.String(), err)
		}
		return fmt.Errorf("scanner error for %s: %w", streamType.String(), err)
	}

	return nil
}

// isExpectedStreamError returns true for errors that occur during normal
// process shutdown: closed file descriptors and closed pipes.
// Note: bufio.Scanner.Err() never returns io.EOF (it returns nil at EOF),
// and errors.Is already unwraps *os.PathError chains, so only os.ErrClosed
// needs to be checked.
func isExpectedStreamError(err error) bool {
	return errors.Is(err, os.ErrClosed)
}

func (p *Processor) addError(err error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.errors = append(p.errors, err)
}