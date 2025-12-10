// Package formatter provides log line formatting functionality.
package formatter

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/itchyny/timefmt-go"
	"github.com/sgaunet/logwrap/pkg/config"
	"github.com/sgaunet/logwrap/pkg/processor"
)

// Formatter defines the interface for formatting log lines.
type Formatter interface {
	FormatLine(line string, streamType processor.StreamType) string
}

// DefaultFormatter provides the default implementation of log line formatting.
type DefaultFormatter struct {
	config   *config.Config
	template *template.Template
	userInfo *user.User
	pid      int
	colors   map[string]string
}

// TemplateData contains the data available for template rendering.
type TemplateData struct {
	Timestamp string
	Level     string
	User      string
	PID       string
	Line      string
}

// New creates a new DefaultFormatter with the given configuration.
func New(cfg *config.Config) (*DefaultFormatter, error) {
	tmpl, err := template.New("prefix").Parse(cfg.Prefix.Template)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	userInfo, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	colors := make(map[string]string)
	if cfg.Prefix.Colors.Enabled {
		colors = map[string]string{
			"info":      getColorCode(cfg.Prefix.Colors.Info),
			"error":     getColorCode(cfg.Prefix.Colors.Error),
			"timestamp": getColorCode(cfg.Prefix.Colors.Timestamp),
			"reset":     "\033[0m",
		}
	}

	return &DefaultFormatter{
		config:   cfg,
		template: tmpl,
		userInfo: userInfo,
		pid:      os.Getpid(),
		colors:   colors,
	}, nil
}

// FormatLine formats a log line according to the configured output format.
func (f *DefaultFormatter) FormatLine(line string, streamType processor.StreamType) string {
	if line == "" {
		return line
	}

	data := f.buildTemplateData(line, streamType)

	switch f.config.Output.Format {
	case "json":
		return f.formatJSON(data)
	case "structured":
		return f.formatStructured(data)
	default: // "text"
		return f.formatText(data)
	}
}

func (f *DefaultFormatter) formatText(data TemplateData) string {
	var builder strings.Builder
	if err := f.template.Execute(&builder, data); err != nil {
		return data.Line
	}

	prefix := builder.String()

	if f.config.Prefix.Colors.Enabled {
		colorizedPrefix := f.colorizePrefix(prefix)
		colorizedLine := f.colorizeLine(data.Line, data.Level)
		return colorizedPrefix + colorizedLine
	}

	return prefix + data.Line
}

func (f *DefaultFormatter) formatJSON(data TemplateData) string {
	jsonData := map[string]any{
		"timestamp": data.Timestamp,
		"level":     data.Level,
		"user":      data.User,
		"pid":       data.PID,
		"message":   data.Line,
	}

	jsonBytes, err := json.Marshal(jsonData)
	if err != nil {
		return data.Line
	}

	return string(jsonBytes)
}

func (f *DefaultFormatter) formatStructured(data TemplateData) string {
	return fmt.Sprintf("timestamp=%s level=%s user=%s pid=%s message=%s",
		quoteIfNeeded(data.Timestamp),
		quoteIfNeeded(data.Level),
		quoteIfNeeded(data.User),
		quoteIfNeeded(data.PID),
		strconv.Quote(data.Line)) // Always quote message field using strconv.Quote for proper escaping
}

// quoteIfNeeded quotes a string value if it contains special characters.
// Uses strconv.Quote for proper Go string escaping.
func quoteIfNeeded(s string) string {
	// Quote if string contains whitespace, quotes, backslashes, or control characters
	if needsQuoting(s) {
		return strconv.Quote(s)
	}
	return s
}

// needsQuoting returns true if the string contains characters that require quoting
// in structured log format (spaces, tabs, newlines, quotes, backslashes, etc.).
func needsQuoting(s string) bool {
	for _, r := range s {
		switch {
		case r == ' ', r == '\t', r == '\n', r == '\r':
			return true
		case r == '"', r == '\'', r == '\\':
			return true
		case r == '=': // Equals signs could break key=value parsing
			return true
		case r < 32 || r == 127: // Control characters
			return true
		}
	}
	return false
}

func (f *DefaultFormatter) buildTemplateData(line string, streamType processor.StreamType) TemplateData {
	return TemplateData{
		Timestamp: f.getTimestamp(),
		Level:     f.getLogLevel(line, streamType),
		User:      f.getUserString(),
		PID:       f.getPIDString(),
		Line:      line,
	}
}

func (f *DefaultFormatter) getTimestamp() string {
	now := time.Now()
	if f.config.Prefix.Timestamp.UTC {
		now = now.UTC()
	}
	return timefmt.Format(now, f.config.Prefix.Timestamp.Format)
}

func (f *DefaultFormatter) getLogLevel(line string, streamType processor.StreamType) string {
	if !f.config.LogLevel.Detection.Enabled {
		if streamType == processor.StreamStdout {
			return f.config.LogLevel.DefaultStdout
		}
		return f.config.LogLevel.DefaultStderr
	}

	lineUpper := strings.ToUpper(line)

	for level, keywords := range f.config.LogLevel.Detection.Keywords {
		for _, keyword := range keywords {
			if strings.Contains(lineUpper, strings.ToUpper(keyword)) {
				return strings.ToUpper(level)
			}
		}
	}

	if streamType == processor.StreamStdout {
		return f.config.LogLevel.DefaultStdout
	}
	return f.config.LogLevel.DefaultStderr
}

func (f *DefaultFormatter) getUserString() string {
	if !f.config.Prefix.User.Enabled {
		return ""
	}

	switch f.config.Prefix.User.Format {
	case "username":
		return f.userInfo.Username
	case "uid":
		return f.userInfo.Uid
	case "full":
		return fmt.Sprintf("%s(%s)", f.userInfo.Username, f.userInfo.Uid)
	default:
		return f.userInfo.Username
	}
}

func (f *DefaultFormatter) getPIDString() string {
	if !f.config.Prefix.PID.Enabled {
		return ""
	}

	switch f.config.Prefix.PID.Format {
	case "decimal":
		return strconv.Itoa(f.pid)
	case "hex":
		return fmt.Sprintf("0x%x", f.pid)
	default:
		return strconv.Itoa(f.pid)
	}
}

func (f *DefaultFormatter) colorizePrefix(prefix string) string {
	if !f.config.Prefix.Colors.Enabled {
		return prefix
	}

	if timestampColor, exists := f.colors["timestamp"]; exists && timestampColor != "" {
		prefix = f.applyTimestampColor(prefix, timestampColor)
	}

	return prefix
}

func (f *DefaultFormatter) colorizeLine(line, level string) string {
	if !f.config.Prefix.Colors.Enabled {
		return line
	}

	var color string
	switch strings.ToUpper(level) {
	case "ERROR", "FATAL", "PANIC":
		color = f.colors["error"]
	case "INFO", "DEBUG", "TRACE", "WARN", "WARNING":
		color = f.colors["info"]
	default:
		return line
	}

	if color != "" && f.colors["reset"] != "" {
		return color + line + f.colors["reset"]
	}

	return line
}

func (f *DefaultFormatter) applyTimestampColor(text, color string) string {
	if color != "" && f.colors["reset"] != "" {
		return color + text + f.colors["reset"]
	}
	return text
}

func getColorCode(colorName string) string {
	colors := map[string]string{
		"black":   "\033[30m",
		"red":     "\033[31m",
		"green":   "\033[32m",
		"yellow":  "\033[33m",
		"blue":    "\033[34m",
		"magenta": "\033[35m",
		"cyan":    "\033[36m",
		"white":   "\033[37m",
		"none":    "",
		"":        "",
	}

	return colors[strings.ToLower(colorName)]
}