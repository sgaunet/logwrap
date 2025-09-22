package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Prefix   PrefixConfig   `yaml:"prefix"`
	Output   OutputConfig   `yaml:"output"`
	LogLevel LogLevelConfig `yaml:"log_level"`
}

type PrefixConfig struct {
	Template  string          `yaml:"template"`
	Timestamp TimestampConfig `yaml:"timestamp"`
	Colors    ColorsConfig    `yaml:"colors"`
	User      UserConfig      `yaml:"user"`
	PID       PIDConfig       `yaml:"pid"`
}

type TimestampConfig struct {
	Format string `yaml:"format"`
	UTC    bool   `yaml:"utc"`
}

type ColorsConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Info      string `yaml:"info"`
	Error     string `yaml:"error"`
	Timestamp string `yaml:"timestamp"`
}

type UserConfig struct {
	Enabled bool   `yaml:"enabled"`
	Format  string `yaml:"format"`
}

type PIDConfig struct {
	Enabled bool   `yaml:"enabled"`
	Format  string `yaml:"format"`
}

type OutputConfig struct {
	Format string `yaml:"format"`
	Buffer string `yaml:"buffer"`
}

type LogLevelConfig struct {
	DefaultStdout string              `yaml:"default_stdout"`
	DefaultStderr string              `yaml:"default_stderr"`
	Detection     DetectionConfig     `yaml:"detection"`
}

type DetectionConfig struct {
	Enabled  bool                `yaml:"enabled"`
	Keywords map[string][]string `yaml:"keywords"`
}

type CLIFlags struct {
	ConfigFile    *string
	Template      *string
	TimestampUTC  *bool
	ColorsEnabled *bool
	UserEnabled   *bool
	PIDEnabled    *bool
	OutputFormat  *string
	Help          *bool
	Version       *bool
}

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
				Format: time.RFC3339,
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
			Buffer: "line",
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
	data, err := os.ReadFile(configFile)
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
	flags.UserEnabled = fs.Bool("user", true, "Include user in prefix")
	flags.PIDEnabled = fs.Bool("pid", true, "Include PID in prefix")
	flags.OutputFormat = fs.String("format", "", "Output format (text, json, structured)")
	flags.Help = fs.Bool("help", false, "Show help")
	flags.Version = fs.Bool("version", false, "Show version")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	return flags, nil
}

func applyCLIOverrides(config *Config, flags *CLIFlags) {
	if flags.Template != nil && *flags.Template != "" {
		config.Prefix.Template = *flags.Template
	}
	if flags.TimestampUTC != nil {
		config.Prefix.Timestamp.UTC = *flags.TimestampUTC
	}
	if flags.ColorsEnabled != nil {
		config.Prefix.Colors.Enabled = *flags.ColorsEnabled
	}
	if flags.UserEnabled != nil {
		config.Prefix.User.Enabled = *flags.UserEnabled
	}
	if flags.PIDEnabled != nil {
		config.Prefix.PID.Enabled = *flags.PIDEnabled
	}
	if flags.OutputFormat != nil && *flags.OutputFormat != "" {
		config.Output.Format = *flags.OutputFormat
	}
}

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