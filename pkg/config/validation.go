package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/sgaunet/logwrap/pkg/errors"
)

// Validate checks if the configuration is valid and returns an error if not.
func (c *Config) Validate() error {
	if err := c.validatePrefix(); err != nil {
		return fmt.Errorf("prefix configuration error: %w", err)
	}

	if err := c.validateOutput(); err != nil {
		return fmt.Errorf("output configuration error: %w", err)
	}

	if err := c.validateLogLevel(); err != nil {
		return fmt.Errorf("log level configuration error: %w", err)
	}

	return nil
}

func (c *Config) validatePrefix() error {
	if c.Prefix.Template == "" {
		return errors.ErrTemplateEmpty
	}

	if err := c.validateTimestamp(); err != nil {
		return fmt.Errorf("timestamp config error: %w", err)
	}

	if err := c.validateColors(); err != nil {
		return fmt.Errorf("colors config error: %w", err)
	}

	if err := c.validateUser(); err != nil {
		return fmt.Errorf("user config error: %w", err)
	}

	if err := c.validatePID(); err != nil {
		return fmt.Errorf("PID config error: %w", err)
	}

	return nil
}

func (c *Config) validateTimestamp() error {
	if c.Prefix.Timestamp.Format == "" {
		return errors.ErrTimestampFormatEmpty
	}

	_, err := time.Parse(c.Prefix.Timestamp.Format, time.Now().Format(c.Prefix.Timestamp.Format))
	if err != nil {
		return fmt.Errorf("invalid timestamp format '%s': %w", c.Prefix.Timestamp.Format, err)
	}

	return nil
}

func (c *Config) validateColors() error {
	validColors := map[string]bool{
		"black":   true,
		"red":     true,
		"green":   true,
		"yellow":  true,
		"blue":    true,
		"magenta": true,
		"cyan":    true,
		"white":   true,
		"none":    true,
		"":        true,
	}

	colors := []struct {
		name  string
		value string
	}{
		{"info", c.Prefix.Colors.Info},
		{"error", c.Prefix.Colors.Error},
		{"timestamp", c.Prefix.Colors.Timestamp},
	}

	for _, color := range colors {
		if !validColors[strings.ToLower(color.value)] {
			return fmt.Errorf("%w '%s' for %s, valid colors: %s",
				errors.ErrInvalidColor, color.value, color.name, getValidColorsString())
		}
	}

	return nil
}

func (c *Config) validateUser() error {
	validFormats := []string{"username", "uid", "full"}

	for _, format := range validFormats {
		if c.Prefix.User.Format == format {
			return nil
		}
	}

	return fmt.Errorf("%w '%s', valid formats: %s",
		errors.ErrInvalidUserFormat, c.Prefix.User.Format, strings.Join(validFormats, ", "))
}

func (c *Config) validatePID() error {
	validFormats := []string{"decimal", "hex"}

	for _, format := range validFormats {
		if c.Prefix.PID.Format == format {
			return nil
		}
	}

	return fmt.Errorf("%w '%s', valid formats: %s",
		errors.ErrInvalidPIDFormat, c.Prefix.PID.Format, strings.Join(validFormats, ", "))
}

func (c *Config) validateOutput() error {
	validFormats := []string{"text", "json", "structured"}

	for _, format := range validFormats {
		if c.Output.Format == format {
			break
		}
	}

	if c.Output.Format != "text" && c.Output.Format != "json" && c.Output.Format != "structured" {
		return fmt.Errorf("%w '%s', valid formats: %s",
			errors.ErrInvalidOutputFormat, c.Output.Format, strings.Join(validFormats, ", "))
	}

	validBuffers := []string{"line", "none", "full"}

	for _, buffer := range validBuffers {
		if c.Output.Buffer == buffer {
			return nil
		}
	}

	return fmt.Errorf("%w '%s', valid modes: %s",
		errors.ErrInvalidBufferMode, c.Output.Buffer, strings.Join(validBuffers, ", "))
}

func (c *Config) validateLogLevel() error {
	validLevels := []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

	if !isValidLogLevel(c.LogLevel.DefaultStdout, validLevels) {
		return fmt.Errorf("%w '%s', valid levels: %s",
			errors.ErrInvalidStdoutLogLevel, c.LogLevel.DefaultStdout, strings.Join(validLevels, ", "))
	}

	if !isValidLogLevel(c.LogLevel.DefaultStderr, validLevels) {
		return fmt.Errorf("%w '%s', valid levels: %s",
			errors.ErrInvalidStderrLogLevel, c.LogLevel.DefaultStderr, strings.Join(validLevels, ", "))
	}

	for level, keywords := range c.LogLevel.Detection.Keywords {
		if !isValidLogLevel(strings.ToUpper(level), validLevels) {
			return fmt.Errorf("%w '%s' in detection keywords", errors.ErrInvalidLogLevel, level)
		}

		if len(keywords) == 0 {
			return fmt.Errorf("%w '%s'", errors.ErrNoDetectionKeywords, level)
		}
	}

	return nil
}

func isValidLogLevel(level string, validLevels []string) bool {
	// Check for exact uppercase match
	for _, valid := range validLevels {
		if level == valid {
			return true
		}
	}

	// Check for exact lowercase match
	for _, valid := range validLevels {
		if level == strings.ToLower(valid) {
			return true
		}
	}

	return false
}

func getValidColorsString() string {
	colors := []string{"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white", "none"}
	return strings.Join(colors, ", ")
}