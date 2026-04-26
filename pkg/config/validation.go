package config

import (
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"text/template"
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

	if err := c.validateFilter(); err != nil {
		return fmt.Errorf("filter configuration error: %w", err)
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

	if err := validateTemplate(c.Prefix.Template); err != nil {
		return fmt.Errorf("template error: %w", err)
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

// validateTemplate parses the Go template and executes it with test data
// to catch both syntax errors and unknown field references at validation
// time rather than at runtime.
//
// The test struct fields must match formatter.TemplateData. We define them
// locally to avoid a circular import (config ← formatter).
func validateTemplate(tmplStr string) error {
	tmpl, err := template.New("prefix").Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("%w: %w", apperrors.ErrInvalidTemplate, err)
	}

	testData := struct {
		Timestamp, Level, User, PID, Line string
	}{"t", "t", "t", "t", "t"}

	if err := tmpl.Execute(io.Discard, testData); err != nil {
		return fmt.Errorf("%w: %w", apperrors.ErrInvalidTemplate, err)
	}

	return nil
}

// validStrftimeDirectives lists the strftime directives supported by timefmt-go.
// Modifiers (-, _, 0) before a directive are also allowed (e.g., %-d, %_H, %0m).
var validStrftimeDirectives = map[byte]bool{
	// Date components
	'Y': true, // 4-digit year (2024)
	'y': true, // 2-digit year (24)
	'C': true, // Century (20)
	'm': true, // Month 01-12
	'd': true, // Day 01-31
	'e': true, // Day space-padded ( 1-31)
	'j': true, // Day of year 001-366
	'G': true, // ISO year
	'g': true, // ISO year 2-digit
	// Time components
	'H': true, // Hour 24h 00-23
	'I': true, // Hour 12h 01-12
	'k': true, // Hour 24h space-padded
	'l': true, // Hour 12h space-padded
	'M': true, // Minute 00-59
	'S': true, // Second 00-59
	'f': true, // Microseconds
	's': true, // Seconds since epoch
	'p': true, // AM/PM
	'P': true, // am/pm
	// Weekday
	'a': true, // Weekday short (Mon)
	'A': true, // Weekday full (Monday)
	'u': true, // Weekday number 1=Mon
	'w': true, // Weekday number 0=Sun
	// Month name
	'b': true, // Month short (Jan)
	'B': true, // Month full (January)
	'h': true, // Month short (alias for %b)
	// Week number
	'U': true, // Week number Sunday start
	'W': true, // Week number Monday start
	'V': true, // ISO week number
	// Timezone
	'z': true, // Timezone offset (-0700)
	'Z': true, // Timezone name (UTC)
	// Composite
	'c': true, // Locale date and time
	'D': true, // Equivalent to %m/%d/%y
	'F': true, // Equivalent to %Y-%m-%d
	'r': true, // 12-hour time
	'R': true, // 24-hour time HH:MM
	'T': true, // 24-hour time HH:MM:SS
	'x': true, // Locale date
	'X': true, // Locale time
	// Special
	'n': true, // Newline
	't': true, // Tab
	'%': true, // Literal %
}

// validateTimestamp validates the strftime timestamp format string.
//
// Validation is two-phase:
//  1. Directive check: scan for %X patterns and reject unknown directives
//  2. Round-trip test: format the current time and parse it back
//
// An empty format string is rejected. The format must use strftime directives
// (e.g., %Y-%m-%d %H:%M:%S), not Go time format (e.g., 2006-01-02).
func (c *Config) validateTimestamp() error {
	if c.Prefix.Timestamp.Format == "" {
		return apperrors.ErrTimestampFormatEmpty
	}

	// Phase 1: validate directives against whitelist
	if err := validateStrftimeDirectives(c.Prefix.Timestamp.Format); err != nil {
		return err
	}

	// Phase 2: round-trip test for format/parse compatibility
	now := time.Now()
	formatted := timefmt.Format(now, c.Prefix.Timestamp.Format)
	_, err := timefmt.Parse(formatted, c.Prefix.Timestamp.Format)
	if err != nil {
		return fmt.Errorf("%w '%s': %w", apperrors.ErrInvalidTimestampFormat,
			c.Prefix.Timestamp.Format, err)
	}

	return nil
}

// validateStrftimeDirectives scans a format string for %X patterns and rejects
// unknown directives. Modifiers (-, _, 0) before a directive are allowed.
func validateStrftimeDirectives(format string) error {
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			continue
		}
		i++ // skip '%'
		if i >= len(format) {
			return fmt.Errorf("%w: trailing '%%' in format '%s'",
				apperrors.ErrInvalidTimestampFormat, format)
		}

		// Skip optional modifier (-, _, 0)
		if format[i] == '-' || format[i] == '_' || format[i] == '0' {
			i++
			if i >= len(format) {
				return fmt.Errorf("%w: trailing modifier in format '%s'",
					apperrors.ErrInvalidTimestampFormat, format)
			}
		}

		if !validStrftimeDirectives[format[i]] {
			return fmt.Errorf("%w: unknown directive '%%%c' in format '%s'",
				apperrors.ErrInvalidTimestampFormat, format[i], format)
		}
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
	return validateOneOf(
		c.Prefix.User.Format, []string{"username", "uid", "full"},
		"formats", apperrors.ErrInvalidUserFormat,
	)
}

// validatePID validates the process ID display format.
//
// Valid formats:
//   - "decimal": displays PID as a decimal number (e.g., "1234")
//   - "hex": displays PID as a hexadecimal number (e.g., "0x4d2")
func (c *Config) validatePID() error {
	return validateOneOf(c.Prefix.PID.Format, []string{"decimal", "hex"}, "formats", apperrors.ErrInvalidPIDFormat)
}

// validateOutput validates the output format setting.
//
// Valid formats: "text", "json", "structured".
func (c *Config) validateOutput() error {
	return validateOneOf(
		c.Output.Format, []string{"text", "json", "structured"},
		"formats", apperrors.ErrInvalidOutputFormat,
	)
}

// validateOneOf checks that value is one of validValues. If not, it returns
// an error wrapping errType with the invalid value and list of valid options.
func validateOneOf(value string, validValues []string, desc string, errType error) error {
	if slices.Contains(validValues, value) {
		return nil
	}
	return fmt.Errorf("%w '%s', valid %s: %s",
		errType, value, desc, strings.Join(validValues, ", "))
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

// validateFilter validates filter patterns and level-based filtering rules.
//
// Empty strings in exclude_patterns or include_patterns are rejected because
// an empty regex matches every line, which would silently drop or pass all
// output — almost certainly a configuration mistake.
//
// Level-based filter rules (include_levels, exclude_levels) require
// detection to be enabled, since level detection is the mechanism
// that assigns levels to lines. Without detection, all lines have
// an empty detected level and level filters silently drop everything.
func (c *Config) validateFilter() error {
	if !c.Filter.Enabled {
		return nil
	}

	validLevels := []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

	if !c.LogLevel.Detection.Enabled {
		if len(c.Filter.IncludeLevels) > 0 || len(c.Filter.ExcludeLevels) > 0 {
			return apperrors.ErrFilterLevelsWithoutDetection
		}
	}
	if err := validateFilterLevelNames(c.Filter.IncludeLevels, "include_levels", validLevels); err != nil {
		return err
	}
	if err := validateFilterLevelNames(c.Filter.ExcludeLevels, "exclude_levels", validLevels); err != nil {
		return err
	}
	if err := validateFilterPatterns(c.Filter.ExcludePatterns, "exclude_patterns"); err != nil {
		return err
	}
	return validateFilterPatterns(c.Filter.IncludePatterns, "include_patterns")
}

// validateFilterPatterns checks that a pattern list contains no empty strings
// and that all entries are valid regular expressions.
func validateFilterPatterns(patterns []string, field string) error {
	if slices.Contains(patterns, "") {
		return fmt.Errorf("%w in %s", apperrors.ErrEmptyFilterPattern, field)
	}
	return validateRegexPatterns(patterns, field)
}

// validateFilterLevelNames checks that all level names in the list are valid
// log levels. This prevents typos from silently dropping all output.
func validateFilterLevelNames(levels []string, field string, validLevels []string) error {
	for _, level := range levels {
		if !isValidLogLevel(strings.ToUpper(level), validLevels) {
			return fmt.Errorf("%w %q in %s, valid levels: %s",
				apperrors.ErrInvalidFilterLevel, level, field, strings.Join(validLevels, ", "))
		}
	}
	return nil
}

// validateRegexPatterns compiles each pattern to check for syntax errors.
func validateRegexPatterns(patterns []string, field string) error {
	for _, p := range patterns {
		if _, err := regexp.Compile(p); err != nil {
			return fmt.Errorf("%w %q in %s: %w", apperrors.ErrInvalidFilterPattern, p, field, err)
		}
	}
	return nil
}

func getValidColorsString() string {
	colors := []string{"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white", "none"}
	return strings.Join(colors, ", ")
}