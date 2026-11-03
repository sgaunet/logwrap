package config

// Color names accepted in color configuration and themes.
// They must match the names understood by the formatter's color lookup.
const (
	colorBlack   = "black"
	colorRed     = "red"
	colorGreen   = "green"
	colorYellow  = "yellow"
	colorBlue    = "blue"
	colorMagenta = "magenta"
	colorCyan    = "cyan"
	colorWhite   = "white"
	colorNone    = "none"
)

// Log level names accepted in log level and filter configuration.
const (
	levelTrace = "TRACE"
	levelDebug = "DEBUG"
	levelInfo  = "INFO"
	levelWarn  = "WARN"
	levelError = "ERROR"
	levelFatal = "FATAL"
)

// validLogLevels returns the log levels accepted in configuration, ordered
// from least to most severe. A new slice is returned on each call so callers
// cannot mutate shared state.
func validLogLevels() []string {
	return []string{levelTrace, levelDebug, levelInfo, levelWarn, levelError, levelFatal}
}

// validColorNames returns the color names accepted in configuration.
func validColorNames() []string {
	return []string{
		colorBlack, colorRed, colorGreen, colorYellow,
		colorBlue, colorMagenta, colorCyan, colorWhite, colorNone,
	}
}
