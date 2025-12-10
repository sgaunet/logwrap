package config

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/itchyny/timefmt-go"
	"github.com/sgaunet/logwrap/pkg/apperrors"
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
		return apperrors.ErrTemplateEmpty
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
		return apperrors.ErrTimestampFormatEmpty
	}

	// Validate strftime format by attempting to format and parse
	now := time.Now()
	formatted := timefmt.Format(now, c.Prefix.Timestamp.Format)
	_, err := timefmt.Parse(formatted, c.Prefix.Timestamp.Format)
	if err != nil {
		return fmt.Errorf("invalid strftime format '%s': %w", c.Prefix.Timestamp.Format, err)
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
				apperrors.ErrInvalidColor, color.value, color.name, getValidColorsString())
		}
	}

	return nil
}

func (c *Config) validateUser() error {
	validFormats := []string{"username", "uid", "full"}

	if slices.Contains(validFormats, c.Prefix.User.Format) {
		return nil
	}

	return fmt.Errorf("%w '%s', valid formats: %s",
		apperrors.ErrInvalidUserFormat, c.Prefix.User.Format, strings.Join(validFormats, ", "))
}

func (c *Config) validatePID() error {
	validFormats := []string{"decimal", "hex"}

	if slices.Contains(validFormats, c.Prefix.PID.Format) {
		return nil
	}

	return fmt.Errorf("%w '%s', valid formats: %s",
		apperrors.ErrInvalidPIDFormat, c.Prefix.PID.Format, strings.Join(validFormats, ", "))
}

func (c *Config) validateOutput() error {
	validFormats := []string{"text", "json", "structured"}

	if !slices.Contains(validFormats, c.Output.Format) {
		return fmt.Errorf("%w '%s', valid formats: %s",
			apperrors.ErrInvalidOutputFormat, c.Output.Format, strings.Join(validFormats, ", "))
	}

	return nil
}

func (c *Config) validateLogLevel() error {
	validLevels := []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

	if !isValidLogLevel(c.LogLevel.DefaultStdout, validLevels) {
		return fmt.Errorf("%w '%s', valid levels: %s",
			apperrors.ErrInvalidStdoutLogLevel, c.LogLevel.DefaultStdout, strings.Join(validLevels, ", "))
	}

	if !isValidLogLevel(c.LogLevel.DefaultStderr, validLevels) {
		return fmt.Errorf("%w '%s', valid levels: %s",
			apperrors.ErrInvalidStderrLogLevel, c.LogLevel.DefaultStderr, strings.Join(validLevels, ", "))
	}

	// Check for conflicting configuration: detection disabled but keywords provided
	if !c.LogLevel.Detection.Enabled && len(c.LogLevel.Detection.Keywords) > 0 {
		return apperrors.ErrDetectionDisabledWithKeywords
	}

	for level, keywords := range c.LogLevel.Detection.Keywords {
		if !isValidLogLevel(strings.ToUpper(level), validLevels) {
			return fmt.Errorf("%w '%s' in detection keywords", apperrors.ErrInvalidLogLevel, level)
		}

		if len(keywords) == 0 {
			return fmt.Errorf("%w '%s'", apperrors.ErrNoDetectionKeywords, level)
		}

		// Check for empty strings in keywords
		//nolint:modernize // Need to return error with level context, not just check existence
		for _, keyword := range keywords {
			if keyword == "" {
				return fmt.Errorf("%w for level '%s'", apperrors.ErrEmptyKeyword, level)
			}
		}
	}

	return nil
}

func isValidLogLevel(level string, validLevels []string) bool {
	// Check for exact uppercase match
	if slices.Contains(validLevels, level) {
		return true
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