package formatter_test

import (
	"fmt"
	"log"

	"github.com/sgaunet/logwrap/pkg/config"
	"github.com/sgaunet/logwrap/pkg/formatter"
	"github.com/sgaunet/logwrap/pkg/processor"
)

// ExampleNew demonstrates creating a formatter with a level-only template.
func ExampleNew() {
	cfg := &config.Config{
		Prefix: config.PrefixConfig{
			Template: "[{{.Level}}] ",
			Timestamp: config.TimestampConfig{
				Format: "%Y-%m-%d %H:%M:%S",
			},
			User: config.UserConfig{
				Enabled: false,
				Format:  "username",
			},
			PID: config.PIDConfig{
				Enabled: false,
				Format:  "decimal",
			},
		},
		Output: config.OutputConfig{
			Format: "text",
		},
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
			DefaultStderr: "ERROR",
			Detection:     config.DetectionConfig{Enabled: false},
		},
	}

	f, err := formatter.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(f.FormatLine("application started", processor.StreamStdout))
	// Output: [INFO] application started
}

// ExampleDefaultFormatter_FormatLine demonstrates formatting lines from
// different streams with log level detection disabled.
func ExampleDefaultFormatter_FormatLine() {
	cfg := &config.Config{
		Prefix: config.PrefixConfig{
			Template: "{{.Level}}: ",
			Timestamp: config.TimestampConfig{
				Format: "%Y-%m-%d %H:%M:%S",
			},
			User: config.UserConfig{
				Enabled: false,
				Format:  "username",
			},
			PID: config.PIDConfig{
				Enabled: false,
				Format:  "decimal",
			},
		},
		Output: config.OutputConfig{
			Format: "text",
		},
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
			DefaultStderr: "ERROR",
			Detection:     config.DetectionConfig{Enabled: false},
		},
	}

	f, err := formatter.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(f.FormatLine("request handled", processor.StreamStdout))
	fmt.Println(f.FormatLine("connection refused", processor.StreamStderr))
	// Output:
	// INFO: request handled
	// ERROR: connection refused
}
