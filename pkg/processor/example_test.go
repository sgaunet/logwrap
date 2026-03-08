package processor_test

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/sgaunet/logwrap/pkg/processor"
)

// stubFormatter is a simple formatter that prefixes lines with the stream type.
type stubFormatter struct{}

func (f *stubFormatter) FormatLine(line string, streamType processor.StreamType) string {
	return fmt.Sprintf("[%s] %s", streamType, line)
}

// ExampleNew demonstrates creating a processor and processing streams.
func ExampleNew() {
	var buf bytes.Buffer
	p := processor.New(&stubFormatter{}, &buf)

	stdout := strings.NewReader("hello world\n")
	stderr := strings.NewReader("")

	if err := p.ProcessStreams(context.Background(), stdout, stderr); err != nil {
		log.Fatal(err)
	}

	fmt.Print(buf.String())
	// Output: [stdout] hello world
}

// ExampleProcessor_ProcessStreams demonstrates processing a single stdout
// stream with a custom formatter.
func ExampleProcessor_ProcessStreams() {
	var buf bytes.Buffer
	p := processor.New(&stubFormatter{}, &buf)

	stdout := strings.NewReader("line 1\nline 2\n")
	stderr := strings.NewReader("")

	if err := p.ProcessStreams(context.Background(), stdout, stderr); err != nil {
		log.Fatal(err)
	}

	fmt.Print(buf.String())
	// Output:
	// [stdout] line 1
	// [stdout] line 2
}
