// Package errors defines static errors used throughout the logwrap application.
package errors

import "errors"

// Configuration errors.
var (
	ErrTemplateEmpty               = errors.New("template cannot be empty")
	ErrTimestampFormatEmpty        = errors.New("timestamp format cannot be empty")
	ErrInvalidColor                = errors.New("invalid color")
	ErrInvalidUserFormat           = errors.New("invalid user format")
	ErrInvalidPIDFormat            = errors.New("invalid PID format")
	ErrInvalidOutputFormat         = errors.New("invalid output format")
	ErrInvalidBufferMode           = errors.New("invalid buffer mode")
	ErrInvalidStdoutLogLevel       = errors.New("invalid default stdout log level")
	ErrInvalidStderrLogLevel       = errors.New("invalid default stderr log level")
	ErrInvalidLogLevel             = errors.New("invalid log level")
	ErrNoDetectionKeywords         = errors.New("log level has no detection keywords")
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