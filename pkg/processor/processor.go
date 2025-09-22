package processor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

type StreamType int

const (
	StreamStdout StreamType = iota
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

type Formatter interface {
	FormatLine(line string, streamType StreamType) string
}

type Processor struct {
	formatter Formatter
	output    io.Writer
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	errors    []error
	mutex     sync.Mutex
}

type ProcessorOption func(*Processor)

func WithContext(ctx context.Context) ProcessorOption {
	return func(p *Processor) {
		p.ctx, p.cancel = context.WithCancel(ctx)
	}
}

func New(formatter Formatter, output io.Writer, opts ...ProcessorOption) *Processor {
	ctx, cancel := context.WithCancel(context.Background())

	p := &Processor{
		formatter: formatter,
		output:    output,
		ctx:       ctx,
		cancel:    cancel,
		errors:    make([]error, 0),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

func (p *Processor) ProcessStreams(stdout, stderr io.Reader) error {
	if stdout == nil || stderr == nil {
		return fmt.Errorf("stdout and stderr readers cannot be nil")
	}

	p.wg.Add(2)

	go func() {
		defer p.wg.Done()
		if err := p.processStream(stdout, StreamStdout); err != nil {
			p.addError(fmt.Errorf("stdout processing error: %w", err))
		}
	}()

	go func() {
		defer p.wg.Done()
		if err := p.processStream(stderr, StreamStderr); err != nil {
			p.addError(fmt.Errorf("stderr processing error: %w", err))
		}
	}()

	p.wg.Wait()

	if len(p.errors) > 0 {
		return fmt.Errorf("processing errors occurred: %v", p.errors)
	}

	return nil
}

func (p *Processor) processStream(stream io.Reader, streamType StreamType) error {
	scanner := bufio.NewScanner(stream)

	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		select {
		case <-p.ctx.Done():
			return p.ctx.Err()
		default:
		}

		line := scanner.Text()
		formattedLine := p.formatter.FormatLine(line, streamType)

		if _, err := p.output.Write([]byte(formattedLine + "\n")); err != nil {
			return fmt.Errorf("failed to write to output: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		if err.Error() == "read |0: file already closed" {
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

func (p *Processor) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

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
		return fmt.Errorf("processor wait timeout after %v", timeout)
	}
}

func (p *Processor) GetErrors() []error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	errors := make([]error, len(p.errors))
	copy(errors, p.errors)
	return errors
}