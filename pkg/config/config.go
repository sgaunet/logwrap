// Package config provides configuration management for logwrap.
//
// The config package handles YAML configuration file parsing, CLI flag parsing,
// configuration merging, and validation.
//
// # Configuration Sources
//
// Configuration is loaded from multiple sources with this precedence
// (highest to lowest):
//  1. CLI flags (highest priority)
//  2. Config file specified via -config flag
//  3. ./logwrap.yaml or ./logwrap.yml (current directory)
//  4. ~/.config/logwrap/config.yaml
//  5. ~/.logwrap.yaml
//  6. Built-in defaults (lowest priority)
//
// Use [LoadConfig] to load and merge all sources, or [FindConfigFile]
// to locate a configuration file in standard locations.
//
// # Configuration Structure
//
// The [Config] struct is organized into sections:
//   - Prefix: Template, timestamp format, colors, user/PID display
//   - Output: Format (text, json, structured)
//   - LogLevel: Default levels and keyword-based detection rules
//
// # Validation
//
// All configuration is validated before use via [Config.Validate]:
//   - Strftime format: round-trip format/parse testing
//   - Log levels: must be TRACE, DEBUG, INFO, WARN, ERROR, or FATAL
//   - Output format: must be text, json, or structured
//   - Colors: validated against known color names when enabled
//   - File paths: path traversal protection and extension validation
//
// # Validation Strategy
//
// The validation system follows these design principles:
//
//   - Fail-fast: validation stops at the first error and returns it. This
//     keeps error messages clear and actionable.
//   - Explicit over implicit: invalid values produce errors rather than being
//     silently corrected. For example, an unknown color name is rejected rather
//     than falling back to a default.
//   - Deterministic order: fields are validated in a fixed order
//     (prefix → output → log level) so that errors are reproducible.
//   - Descriptive errors: error messages include the invalid value and list
//     all valid options, so users can fix issues without consulting documentation.
//
// # Security
//
// Configuration file loading includes security measures:
//   - Path traversal prevention (rejects paths containing "..")
//   - File type validation (only .yaml/.yml files accepted)
package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sgaunet/logwrap/pkg/apperrors"
	"gopkg.in/yaml.v3"
)

// Config represents the complete configuration for logwrap.
type Config struct {
	Prefix   PrefixConfig   `yaml:"prefix"`
	Output   OutputConfig   `yaml:"output"`
	LogLevel LogLevelConfig `yaml:"log_level"`
}

// PrefixConfig contains configuration for log prefixes.
type PrefixConfig struct {
	Template  string          `yaml:"template"`
	Timestamp TimestampConfig `yaml:"timestamp"`
	Colors    ColorsConfig    `yaml:"colors"`
	User      UserConfig      `yaml:"user"`
	PID       PIDConfig       `yaml:"pid"`
}

// TimestampConfig contains timestamp formatting configuration.
type TimestampConfig struct {
	Format string `yaml:"format"`
	UTC    bool   `yaml:"utc"`
}

// ColorsConfig contains color configuration for output.
type ColorsConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Info      string `yaml:"info"`
	Error     string `yaml:"error"`
	Timestamp string `yaml:"timestamp"`
}

// UserConfig contains user information configuration.
type UserConfig struct {
	Enabled bool   `yaml:"enabled"`
	Format  string `yaml:"format"`
}

// PIDConfig contains process ID configuration.
type PIDConfig struct {
	Enabled bool   `yaml:"enabled"`
	Format  string `yaml:"format"`
}

// OutputConfig contains output formatting configuration.
type OutputConfig struct {
	Format string `yaml:"format"`
}

// LogLevelConfig contains log level detection configuration.
type LogLevelConfig struct {
	DefaultStdout string              `yaml:"default_stdout"`
	DefaultStderr string              `yaml:"default_stderr"`
	Detection     DetectionConfig     `yaml:"detection"`
}

// DetectionConfig contains configuration for automatic log level detection.
type DetectionConfig struct {
	Enabled  bool                `yaml:"enabled"`
	Keywords map[string][]string `yaml:"keywords"`
}

// CLIFlags contains parsed command line flags.
type CLIFlags struct {
	ConfigFile    *string
	Template      *string
	TimestampUTC  *bool
	ColorsEnabled *bool
	OutputFormat  *string
	Help          *bool
	Version       *bool
	setFlags      map[string]bool // tracks which flags were explicitly set on the command line
}

// LoadConfig loads configuration from file and applies CLI overrides.
func LoadConfig(configFile string, args []string) (*Config, error) {
	config := getDefaultConfig()

	if configFile != "" {
		if err := loadConfigFile(config, configFile); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	flags, err := parseCLIFlags(args)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CLI flags: %w", err)
	}

	applyCLIOverrides(config, flags)

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

func getDefaultConfig() *Config {
	return &Config{
		Prefix: PrefixConfig{
			Template: "[{{.Timestamp}}] [{{.Level}}] [{{.User}}:{{.PID}}] ",
			Timestamp: TimestampConfig{
				Format: "%Y-%m-%dT%H:%M:%S%z", // RFC3339-like format using strftime
				UTC:    false,
			},
			Colors: ColorsConfig{
				Enabled:   false,
				Info:      "green",
				Error:     "red",
				Timestamp: "blue",
			},
			User: UserConfig{
				Enabled: true,
				Format:  "username",
			},
			PID: PIDConfig{
				Enabled: true,
				Format:  "decimal",
			},
		},
		Output: OutputConfig{
			Format: "text",
		},
		LogLevel: LogLevelConfig{
			DefaultStdout: "INFO",
			DefaultStderr: "ERROR",
			Detection: DetectionConfig{
				Enabled: true,
				Keywords: map[string][]string{
					"error": {"ERROR", "FATAL", "PANIC", "error:", "Error:", "ERROR:"},
					"warn":  {"WARN", "WARNING", "warn:", "Warn:", "WARN:", "WARNING:"},
					"debug": {"DEBUG", "TRACE", "debug:", "Debug:", "DEBUG:", "TRACE:"},
					"info":  {"INFO", "info:", "Info:", "INFO:"},
				},
			},
		},
	}
}

func loadConfigFile(config *Config, configFile string) error {
	// #nosec G304 - configFile is validated or comes from trusted sources
	if err := validateConfigPath(configFile); err != nil {
		return fmt.Errorf("invalid config file path: %w", err)
	}

	data, err := os.ReadFile(configFile) // #nosec G304 - path is validated above
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", configFile, err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to parse YAML config: %w", err)
	}

	return nil
}

func parseCLIFlags(args []string) (*CLIFlags, error) {
	flags := &CLIFlags{}

	fs := flag.NewFlagSet("logwrap", flag.ContinueOnError)
	flags.ConfigFile = fs.String("config", "", "Configuration file path")
	flags.Template = fs.String("template", "", "Log prefix template")
	flags.TimestampUTC = fs.Bool("utc", false, "Use UTC timestamps")
	flags.ColorsEnabled = fs.Bool("colors", false, "Enable colored output")
	flags.OutputFormat = fs.String("format", "", "Output format (text, json, structured)")
	flags.Help = fs.Bool("help", false, "Show help")
	flags.Version = fs.Bool("version", false, "Show version")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("failed to parse flags: %w", err)
	}

	flags.setFlags = make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		flags.setFlags[f.Name] = true
	})

	return flags, nil
}

func applyCLIOverrides(config *Config, flags *CLIFlags) {
	if flags.Template != nil && *flags.Template != "" {
		config.Prefix.Template = *flags.Template
	}
	if flags.setFlags["utc"] {
		config.Prefix.Timestamp.UTC = *flags.TimestampUTC
	}
	if flags.setFlags["colors"] {
		config.Prefix.Colors.Enabled = *flags.ColorsEnabled
	}
	if flags.OutputFormat != nil && *flags.OutputFormat != "" {
		config.Output.Format = *flags.OutputFormat
	}
}

// FindConfigFile searches for configuration files in standard locations.
func FindConfigFile() string {
	candidates := []string{
		"logwrap.yaml",
		"logwrap.yml",
		".logwrap.yaml",
		".logwrap.yml",
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		candidates = append(candidates,
			filepath.Join(homeDir, ".config", "logwrap", "config.yaml"),
			filepath.Join(homeDir, ".config", "logwrap", "config.yml"),
			filepath.Join(homeDir, ".logwrap.yaml"),
			filepath.Join(homeDir, ".logwrap.yml"),
		)
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return ""
}

// validateConfigPath validates that a configuration file path is safe to read.
//
// Security checks:
//   - Path traversal: rejects paths containing ".." after filepath.Clean
//   - File type: only .yaml and .yml extensions are accepted (case-insensitive)
func validateConfigPath(configFile string) error {
	// Prevent path traversal attacks
	cleaned := filepath.Clean(configFile)
	if strings.Contains(cleaned, "..") {
		return apperrors.ErrPathTraversal
	}

	// Only allow .yaml, .yml files
	ext := strings.ToLower(filepath.Ext(cleaned))
	if ext != ".yaml" && ext != ".yml" {
		return apperrors.ErrInvalidFileType
	}

	return nil
}