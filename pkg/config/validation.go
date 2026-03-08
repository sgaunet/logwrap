package config

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/itchyny/timefmt-go"
	"github.com/sgaunet/logwrap/pkg/apperrors"
)

// Validate checks the entire configuration and returns the first error found.
//
// Validation follows a fail-fast strategy: it stops at the first error rather
// than collecting all errors. This keeps error messages actionable — users fix
// one issue at a time.
//
// Validation order: prefix → output → log level. Within prefix validation,
// sub-fields are checked in order: template → timestamp → colors → user → PID.
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

// validatePrefix validates all prefix-related configuration.
//
// It requires a non-empty template, then validates sub-fields in order:
// timestamp format, colors, user format, and PID format. Returns the first
// error encountered.
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

// validateTimestamp validates the strftime timestamp format string.
//
// Validation uses a round-trip strategy: the format is used to format the
// current time, then the result is parsed back using the same format. If the
// parse fails, the format string contains invalid strftime directives.
//
// An empty format string is rejected. The format must use strftime directives
// (e.g., %Y-%m-%d %H:%M:%S), not Go time format (e.g., 2006-01-02).
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

// validateColors validates color names for info, error, and timestamp fields.
//
// Valid colors: black, red, green, yellow, blue, magenta, cyan, white, none.
// An empty string is also accepted (treated as no color override).
// Matching is case-insensitive: "Red", "RED", and "red" are all valid.
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

// validateUser validates the user display format.
//
// Valid formats:
//   - "username": displays the login name (e.g., "alice")
//   - "uid": displays the numeric user ID (e.g., "1000")
//   - "full": displays both as username(uid) (e.g., "alice(1000)")
func (c *Config) validateUser() error {
	validFormats := []string{"username", "uid", "full"}

	if slices.Contains(validFormats, c.Prefix.User.Format) {
		return nil
	}

	return fmt.Errorf("%w '%s', valid formats: %s",
		apperrors.ErrInvalidUserFormat, c.Prefix.User.Format, strings.Join(validFormats, ", "))
}

// validatePID validates the process ID display format.
//
// Valid formats:
//   - "decimal": displays PID as a decimal number (e.g., "1234")
//   - "hex": displays PID as a hexadecimal number (e.g., "0x4d2")
func (c *Config) validatePID() error {
	validFormats := []string{"decimal", "hex"}

	if slices.Contains(validFormats, c.Prefix.PID.Format) {
		return nil
	}

	return fmt.Errorf("%w '%s', valid formats: %s",
		apperrors.ErrInvalidPIDFormat, c.Prefix.PID.Format, strings.Join(validFormats, ", "))
}

// validateOutput validates the output format setting.
//
// Valid formats: "text", "json", "structured".
func (c *Config) validateOutput() error {
	validFormats := []string{"text", "json", "structured"}

	if !slices.Contains(validFormats, c.Output.Format) {
		return fmt.Errorf("%w '%s', valid formats: %s",
			apperrors.ErrInvalidOutputFormat, c.Output.Format, strings.Join(validFormats, ", "))
	}

	return nil
}

// validateLogLevel validates log level defaults and detection keyword rules.
//
// Valid log levels: TRACE, DEBUG, INFO, WARN, ERROR, FATAL. Levels accept
// exact uppercase (e.g., "INFO") or exact lowercase (e.g., "info") only —
// mixed case like "Info" is rejected.
//
// Detection keyword rules:
//   - If detection is disabled, keywords must not be provided (conflicting config)
//   - Each keyword map key must be a valid log level
//   - Empty keyword arrays are rejected — if a level is listed, it must have keywords
//   - Empty strings within keyword arrays are rejected
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

// isValidLogLevel checks whether a level string matches one of the valid levels.
//
// It accepts exact uppercase (e.g., "INFO") or exact lowercase (e.g., "info").
// Mixed case like "Info" or "iNFO" is not accepted. This strict matching avoids
// ambiguity while supporting both common conventions.
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