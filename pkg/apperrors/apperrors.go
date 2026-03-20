// Package apperrors defines sentinel errors used throughout the logwrap
// application. All errors are package-level variables created with
// [errors.New], enabling callers to use [errors.Is] for matching.
//
// Errors are organized by subsystem: configuration, command line,
// executor, processor, and security.
package apperrors

import "errors"

// Configuration errors.
var (
	ErrTemplateEmpty               = errors.New("template cannot be empty")
	ErrInvalidTemplate             = errors.New("invalid template")
	ErrTimestampFormatEmpty        = errors.New("timestamp format cannot be empty")
	ErrInvalidTimestampFormat      = errors.New("invalid timestamp format")
	ErrInvalidTimezone             = errors.New("invalid timezone")
	ErrInvalidColor                = errors.New("invalid color")
	ErrInvalidColorTheme           = errors.New("unknown color theme")
	ErrInvalidUserFormat           = errors.New("invalid user format")
	ErrInvalidPIDFormat            = errors.New("invalid PID format")
	ErrInvalidOutputFormat         = errors.New("invalid output format")
	ErrInvalidStdoutLogLevel       = errors.New("invalid default stdout log level")
	ErrInvalidStderrLogLevel       = errors.New("invalid default stderr log level")
	ErrInvalidLogLevel             = errors.New("invalid log level")
	ErrNoDetectionKeywords         = errors.New("log level has no detection keywords")
	ErrEmptyKeyword                = errors.New("empty keyword in detection keywords")
	ErrDetectionDisabledWithKeywords = errors.New("detection disabled but keywords are configured")
	ErrEmptyFilterPattern            = errors.New("empty string in filter patterns is not allowed")
	ErrFilterLevelsWithoutDetection  = errors.New("filter include_levels/exclude_levels require detection to be enabled")
	ErrInvalidFilterPattern          = errors.New("invalid regex in filter pattern")
	ErrInvalidFilterLevel            = errors.New("invalid log level in filter")
)

// Command line errors.
var (
	ErrOptionRequiresValue = errors.New("option requires a value")
)

// Executor errors.
var (
	ErrCommandEmpty      = errors.New("command cannot be empty")
	ErrExecutorStarted   = errors.New("executor already started")
	ErrExecutorNotStarted = errors.New("executor not started")
)

// Processor errors.
var (
	ErrReadersNil        = errors.New("stdout and stderr readers cannot be nil")
	ErrProcessingErrors  = errors.New("processing errors occurred")
	ErrProcessorTimeout  = errors.New("processor wait timeout")
)

// Security errors.
var (
	ErrPathTraversal        = errors.New("path traversal not allowed")
	ErrInvalidFileType      = errors.New("only .yaml and .yml files are allowed")
	ErrCommandPathTraversal = errors.New("path traversal not allowed in command")
)
