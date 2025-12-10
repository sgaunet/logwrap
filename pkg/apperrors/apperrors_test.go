package apperrors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigurationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "ErrTemplateEmpty",
			err:      ErrTemplateEmpty,
			expected: "template cannot be empty",
		},
		{
			name:     "ErrTimestampFormatEmpty",
			err:      ErrTimestampFormatEmpty,
			expected: "timestamp format cannot be empty",
		},
		{
			name:     "ErrInvalidColor",
			err:      ErrInvalidColor,
			expected: "invalid color",
		},
		{
			name:     "ErrInvalidUserFormat",
			err:      ErrInvalidUserFormat,
			expected: "invalid user format",
		},
		{
			name:     "ErrInvalidPIDFormat",
			err:      ErrInvalidPIDFormat,
			expected: "invalid PID format",
		},
		{
			name:     "ErrInvalidOutputFormat",
			err:      ErrInvalidOutputFormat,
			expected: "invalid output format",
		},
		{
			name:     "ErrInvalidBufferMode",
			err:      ErrInvalidBufferMode,
			expected: "invalid buffer mode",
		},
		{
			name:     "ErrInvalidStdoutLogLevel",
			err:      ErrInvalidStdoutLogLevel,
			expected: "invalid default stdout log level",
		},
		{
			name:     "ErrInvalidStderrLogLevel",
			err:      ErrInvalidStderrLogLevel,
			expected: "invalid default stderr log level",
		},
		{
			name:     "ErrInvalidLogLevel",
			err:      ErrInvalidLogLevel,
			expected: "invalid log level",
		},
		{
			name:     "ErrNoDetectionKeywords",
			err:      ErrNoDetectionKeywords,
			expected: "log level has no detection keywords",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Error(t, tt.err)
			assert.Equal(t, tt.expected, tt.err.Error())

			// Test that errors can be wrapped
			wrapped := fmt.Errorf("configuration failed: %w", tt.err)
			assert.True(t, errors.Is(wrapped, tt.err))
		})
	}
}

func TestCommandLineErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "ErrOptionRequiresValue",
			err:      ErrOptionRequiresValue,
			expected: "option requires a value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Error(t, tt.err)
			assert.Equal(t, tt.expected, tt.err.Error())

			// Test that errors can be wrapped
			wrapped := fmt.Errorf("argument parsing failed: %w", tt.err)
			assert.True(t, errors.Is(wrapped, tt.err))
		})
	}
}

func TestExecutorErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "ErrCommandEmpty",
			err:      ErrCommandEmpty,
			expected: "command cannot be empty",
		},
		{
			name:     "ErrExecutorStarted",
			err:      ErrExecutorStarted,
			expected: "executor already started",
		},
		{
			name:     "ErrExecutorNotStarted",
			err:      ErrExecutorNotStarted,
			expected: "executor not started",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Error(t, tt.err)
			assert.Equal(t, tt.expected, tt.err.Error())

			// Test that errors can be wrapped
			wrapped := fmt.Errorf("executor operation failed: %w", tt.err)
			assert.True(t, errors.Is(wrapped, tt.err))
		})
	}
}

func TestProcessorErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "ErrReadersNil",
			err:      ErrReadersNil,
			expected: "stdout and stderr readers cannot be nil",
		},
		{
			name:     "ErrProcessingErrors",
			err:      ErrProcessingErrors,
			expected: "processing errors occurred",
		},
		{
			name:     "ErrProcessorTimeout",
			err:      ErrProcessorTimeout,
			expected: "processor wait timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Error(t, tt.err)
			assert.Equal(t, tt.expected, tt.err.Error())

			// Test that errors can be wrapped
			wrapped := fmt.Errorf("stream processing failed: %w", tt.err)
			assert.True(t, errors.Is(wrapped, tt.err))
		})
	}
}

func TestSecurityErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "ErrPathTraversal",
			err:      ErrPathTraversal,
			expected: "path traversal not allowed",
		},
		{
			name:     "ErrInvalidFileType",
			err:      ErrInvalidFileType,
			expected: "only .yaml and .yml files are allowed",
		},
		{
			name:     "ErrCommandPathTraversal",
			err:      ErrCommandPathTraversal,
			expected: "path traversal not allowed in command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Error(t, tt.err)
			assert.Equal(t, tt.expected, tt.err.Error())

			// Test that errors can be wrapped
			wrapped := fmt.Errorf("security validation failed: %w", tt.err)
			assert.True(t, errors.Is(wrapped, tt.err))
		})
	}
}

func TestErrorWrapping(t *testing.T) {
	t.Parallel()

	// Test that all errors can be properly wrapped and unwrapped
	allErrors := []error{
		// Configuration errors
		ErrTemplateEmpty,
		ErrTimestampFormatEmpty,
		ErrInvalidColor,
		ErrInvalidUserFormat,
		ErrInvalidPIDFormat,
		ErrInvalidOutputFormat,
		ErrInvalidBufferMode,
		ErrInvalidStdoutLogLevel,
		ErrInvalidStderrLogLevel,
		ErrInvalidLogLevel,
		ErrNoDetectionKeywords,

		// Command line errors
		ErrOptionRequiresValue,

		// Executor errors
		ErrCommandEmpty,
		ErrExecutorStarted,
		ErrExecutorNotStarted,

		// Processor errors
		ErrReadersNil,
		ErrProcessingErrors,
		ErrProcessorTimeout,

		// Security errors
		ErrPathTraversal,
		ErrInvalidFileType,
		ErrCommandPathTraversal,
	}

	for i, err := range allErrors {
		t.Run(fmt.Sprintf("Error_%d_%s", i, err.Error()), func(t *testing.T) {
			t.Parallel()

			// Test wrapping with fmt.Errorf
			wrapped := fmt.Errorf("operation failed: %w", err)
			assert.True(t, errors.Is(wrapped, err))

			// Test unwrapping
			unwrapped := errors.Unwrap(wrapped)
			assert.Equal(t, err, unwrapped)

			// Test that the error is not nil
			assert.NotNil(t, err)
			assert.NotEmpty(t, err.Error())
		})
	}
}

func TestErrorChaining(t *testing.T) {
	t.Parallel()

	// Test complex error chaining scenarios
	baseErr := ErrCommandEmpty

	// Create a chain of wrapped errors
	level1 := fmt.Errorf("executor creation failed: %w", baseErr)
	level2 := fmt.Errorf("command execution failed: %w", level1)
	level3 := fmt.Errorf("application startup failed: %w", level2)

	// Test that errors.Is works through the chain
	assert.True(t, errors.Is(level3, baseErr))
	assert.True(t, errors.Is(level2, baseErr))
	assert.True(t, errors.Is(level1, baseErr))

	// Test that we can distinguish between different base errors
	assert.False(t, errors.Is(level3, ErrTemplateEmpty))
	assert.False(t, errors.Is(level3, ErrProcessingErrors))
}

func TestErrorTypes(t *testing.T) {
	t.Parallel()

	// Test that all errors are of the correct underlying type
	allErrors := []error{
		ErrTemplateEmpty,
		ErrCommandEmpty,
		ErrReadersNil,
		ErrPathTraversal,
	}

	for _, err := range allErrors {
		t.Run(err.Error(), func(t *testing.T) {
			t.Parallel()

			// Test that errors implement the error interface
			var _ error = err

			// Test that Error() method returns non-empty string
			errorMsg := err.Error()
			assert.NotEmpty(t, errorMsg)
			assert.NotEqual(t, "", errorMsg)
		})
	}
}
