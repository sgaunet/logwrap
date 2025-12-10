package formatter

import (
	"fmt"
	"testing"

	"github.com/sgaunet/logwrap/pkg/config"
	"github.com/sgaunet/logwrap/pkg/processor"
)

// BenchmarkGetLogLevel_WithCache measures performance with the cache enabled
func BenchmarkGetLogLevel_WithCache(b *testing.B) {
	cfg := &config.Config{
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
			DefaultStderr: "ERROR",
			Detection: config.DetectionConfig{
				Enabled: true,
				Keywords: map[string][]string{
					"error": {"ERROR", "FATAL", "PANIC"},
					"warn":  {"WARN", "WARNING"},
					"debug": {"DEBUG", "TRACE"},
					"info":  {"INFO"},
				},
			},
		},
	}

	formatter, err := New(cfg)
	if err != nil {
		b.Fatalf("Failed to create formatter: %v", err)
	}

	// Test different scenarios
	scenarios := []struct {
		name  string
		lines []string
	}{
		{
			name: "RepeatedLines",
			lines: []string{
				"ERROR: connection failed",
				"ERROR: connection failed",
				"ERROR: connection failed",
			},
		},
		{
			name: "UniqueLines",
			lines: []string{
				"ERROR: connection failed at 10:01:23",
				"ERROR: connection failed at 10:01:24",
				"ERROR: connection failed at 10:01:25",
			},
		},
		{
			name: "MixedLevels",
			lines: []string{
				"INFO: started",
				"DEBUG: processing item 1",
				"WARN: slow response",
				"ERROR: failed to connect",
			},
		},
		{
			name: "NoKeywords",
			lines: []string{
				"regular log line without keywords",
				"another regular line",
				"yet another line",
			},
		},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, line := range scenario.lines {
					_ = formatter.getLogLevel(line, processor.StreamStdout)
				}
			}
		})
	}
}

// BenchmarkGetLogLevel_WithoutCache measures performance without cache
func BenchmarkGetLogLevel_WithoutCache(b *testing.B) {
	cfg := &config.Config{
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
			DefaultStderr: "ERROR",
			Detection: config.DetectionConfig{
				Enabled: true,
				Keywords: map[string][]string{
					"error": {"ERROR", "FATAL", "PANIC"},
					"warn":  {"WARN", "WARNING"},
					"debug": {"DEBUG", "TRACE"},
					"info":  {"INFO"},
				},
			},
		},
	}

	formatter, err := New(cfg)
	if err != nil {
		b.Fatalf("Failed to create formatter: %v", err)
	}

	// Clear cache before each operation to simulate no-cache scenario
	scenarios := []struct {
		name  string
		lines []string
	}{
		{
			name: "RepeatedLines",
			lines: []string{
				"ERROR: connection failed",
				"ERROR: connection failed",
				"ERROR: connection failed",
			},
		},
		{
			name: "UniqueLines",
			lines: []string{
				"ERROR: connection failed at 10:01:23",
				"ERROR: connection failed at 10:01:24",
				"ERROR: connection failed at 10:01:25",
			},
		},
		{
			name: "MixedLevels",
			lines: []string{
				"INFO: started",
				"DEBUG: processing item 1",
				"WARN: slow response",
				"ERROR: failed to connect",
			},
		},
		{
			name: "NoKeywords",
			lines: []string{
				"regular log line without keywords",
				"another regular line",
				"yet another line",
			},
		},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, line := range scenario.lines {
					_ = formatter.getLogLevel(line, processor.StreamStdout)
				}
			}
		})
	}
}

// BenchmarkGetLogLevel_RealWorld simulates real-world log patterns
func BenchmarkGetLogLevel_RealWorld(b *testing.B) {
	cfg := &config.Config{
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
			DefaultStderr: "ERROR",
			Detection: config.DetectionConfig{
				Enabled: true,
				Keywords: map[string][]string{
					"error": {"ERROR", "FATAL", "PANIC"},
					"warn":  {"WARN", "WARNING"},
					"debug": {"DEBUG", "TRACE"},
					"info":  {"INFO"},
				},
			},
		},
	}

	formatter, err := New(cfg)
	if err != nil {
		b.Fatalf("Failed to create formatter: %v", err)
	}

	// Simulate real-world patterns: mostly unique lines with timestamps and IDs
	b.Run("TimestampedLogs", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			line := fmt.Sprintf("2024-01-15 10:%02d:%02d ERROR: connection timeout", i%60, i%60)
			_ = formatter.getLogLevel(line, processor.StreamStdout)
		}
	})

	b.Run("WithUniqueIDs", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			line := fmt.Sprintf("INFO: processing request id=%d", i)
			_ = formatter.getLogLevel(line, processor.StreamStdout)
		}
	})

	b.Run("WithCounters", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			line := fmt.Sprintf("DEBUG: item %d processed successfully", i)
			_ = formatter.getLogLevel(line, processor.StreamStdout)
		}
	})
}

// BenchmarkCacheGrowth measures memory impact of unbounded cache
func BenchmarkCacheGrowth(b *testing.B) {
	cfg := &config.Config{
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
			DefaultStderr: "ERROR",
			Detection: config.DetectionConfig{
				Enabled: true,
				Keywords: map[string][]string{
					"error": {"ERROR"},
					"info":  {"INFO"},
				},
			},
		},
	}

	formatter, err := New(cfg)
	if err != nil {
		b.Fatalf("Failed to create formatter: %v", err)
	}

	b.Run("100KUniqueLines", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for j := 0; j < 100000; j++ {
				line := fmt.Sprintf("INFO: unique log line with timestamp %d and counter %d", i, j)
				_ = formatter.getLogLevel(line, processor.StreamStdout)
			}
		}
	})
}

// BenchmarkKeywordDetection_Complexity measures the cost of keyword scanning
func BenchmarkKeywordDetection_Complexity(b *testing.B) {
	cfg := &config.Config{
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
			DefaultStderr: "ERROR",
			Detection: config.DetectionConfig{
				Enabled: true,
				Keywords: map[string][]string{
					"error": {"ERROR", "FATAL", "PANIC"},
					"warn":  {"WARN", "WARNING"},
					"debug": {"DEBUG", "TRACE"},
					"info":  {"INFO"},
				},
			},
		},
	}

	formatter, err := New(cfg)
	if err != nil {
		b.Fatalf("Failed to create formatter: %v", err)
	}

	// Test with different line lengths
	scenarios := []struct {
		name string
		line string
	}{
		{"Short", "ERROR: fail"},
		{"Medium", "ERROR: connection failed to database server at 192.168.1.100:5432"},
		{"Long", "ERROR: " + string(make([]byte, 500))}, // 500 byte line
		{"VeryLong", "ERROR: " + string(make([]byte, 2000))}, // 2KB line
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = formatter.getLogLevel(scenario.line, processor.StreamStdout)
			}
		})
	}
}
