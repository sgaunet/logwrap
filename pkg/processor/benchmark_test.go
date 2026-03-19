package processor_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/sgaunet/logwrap/internal/testutils"
	"github.com/sgaunet/logwrap/pkg/processor"
)

// benchFormatter is a minimal formatter for benchmarks to isolate scanner/buffer performance.
type benchFormatter struct{}

func (f *benchFormatter) FormatLine(line string, streamType processor.StreamType) string {
	return "[" + streamType.String() + "] " + line
}

// BenchmarkProcessStream_LineVolume measures throughput at different line volumes.
func BenchmarkProcessStream_LineVolume(b *testing.B) {
	volumes := []int{10, 100, 1000, 10000}

	for _, n := range volumes {
		b.Run(fmt.Sprintf("%d_lines", n), func(b *testing.B) {
			// Build content once
			lines := make([]string, n)
			for i := range lines {
				lines[i] = "INFO: benchmark log line for volume test"
			}
			content := strings.Join(lines, "\n") + "\n"
			contentBytes := int64(len(content))

			b.ReportAllocs()
			b.SetBytes(contentBytes)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				output := &testutils.MockWriter{}
				p := processor.New(&benchFormatter{}, output)
				ctx := context.Background()
				_ = p.ProcessStreams(ctx, strings.NewReader(content), strings.NewReader(""))
			}
		})
	}
}

// BenchmarkProcessStream_LineSize measures throughput for different line sizes.
func BenchmarkProcessStream_LineSize(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"100B", 100},
		{"1KB", 1024},
		{"10KB", 10 * 1024},
		{"100KB", 100 * 1024},
	}

	for _, s := range sizes {
		b.Run(s.name, func(b *testing.B) {
			line := strings.Repeat("x", s.size)
			// Use 100 lines to amortize setup cost
			content := strings.Repeat(line+"\n", 100)
			contentBytes := int64(len(content))

			b.ReportAllocs()
			b.SetBytes(contentBytes)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				output := &testutils.MockWriter{}
				p := processor.New(&benchFormatter{}, output)
				ctx := context.Background()
				_ = p.ProcessStreams(ctx, strings.NewReader(content), strings.NewReader(""))
			}
		})
	}
}

// BenchmarkProcessStream_Concurrent measures throughput with both streams active.
func BenchmarkProcessStream_Concurrent(b *testing.B) {
	const lineCount = 1000
	lines := make([]string, lineCount)
	for i := range lines {
		lines[i] = "INFO: concurrent benchmark log line"
	}
	content := strings.Join(lines, "\n") + "\n"
	contentBytes := int64(len(content) * 2) // both streams

	b.ReportAllocs()
	b.SetBytes(contentBytes)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		output := &testutils.MockWriter{}
		p := processor.New(&benchFormatter{}, output)
		ctx := context.Background()
		_ = p.ProcessStreams(ctx, strings.NewReader(content), strings.NewReader(content))
	}
}
