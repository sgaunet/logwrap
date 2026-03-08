package config_test

import (
	"fmt"

	"github.com/sgaunet/logwrap/pkg/config"
)

func ExampleConfig_Validate() {
	cfg := &config.Config{
		Prefix: config.PrefixConfig{
			Template: "[{{.Level}}] ",
			Timestamp: config.TimestampConfig{
				Format: "%Y-%m-%d %H:%M:%S",
			},
			Colors: config.ColorsConfig{
				Enabled: false,
				Info:    "green",
				Error:   "red",
			},
			User: config.UserConfig{
				Enabled: true,
				Format:  "username",
			},
			PID: config.PIDConfig{
				Enabled: true,
				Format:  "decimal",
			},
		},
		Output: config.OutputConfig{
			Format: "text",
		},
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

	if err := cfg.Validate(); err != nil {
		fmt.Println("invalid:", err)
		return
	}

	fmt.Println("config is valid")
	// Output: config is valid
}

func ExampleConfig_Validate_invalidOutputFormat() {
	cfg := &config.Config{
		Prefix: config.PrefixConfig{
			Template: "[{{.Level}}] ",
			Timestamp: config.TimestampConfig{
				Format: "%Y-%m-%d",
			},
			User: config.UserConfig{Format: "username"},
			PID:  config.PIDConfig{Format: "decimal"},
		},
		Output: config.OutputConfig{
			Format: "xml",
		},
		LogLevel: config.LogLevelConfig{
			DefaultStdout: "INFO",
			DefaultStderr: "ERROR",
			Detection:     config.DetectionConfig{Enabled: false},
		},
	}

	err := cfg.Validate()
	fmt.Println(err)
	// Output: output configuration error: invalid output format 'xml', valid formats: text, json, structured
}
