package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testKeywords = map[string][]string{
	"error": {"ERROR", "FATAL"},
	"warn":  {"WARN", "WARNING"},
	"info":  {"INFO"},
	"debug": {"DEBUG", "TRACE"},
}

func TestNew_Success(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Enabled:         true,
		ExcludePatterns: []string{"heartbeat"},
		IncludePatterns: []string{"important"},
		ExcludeLevels:   []string{"DEBUG"},
		IncludeLevels:   []string{"ERROR", "WARN"},
	}

	f, err := New(cfg, testKeywords)
	require.NoError(t, err)
	require.NotNil(t, f)
}

func TestNew_InvalidPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "invalid exclude pattern",
			cfg:  Config{ExcludePatterns: []string{"[invalid"}},
		},
		{
			name: "invalid include pattern",
			cfg:  Config{IncludePatterns: []string{"[invalid"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f, err := New(tt.cfg, nil)
			assert.Error(t, err)
			assert.Nil(t, f)
		})
	}
}

func TestFilter_ExcludePatterns(t *testing.T) {
	t.Parallel()

	f, err := New(Config{ExcludePatterns: []string{"heartbeat", "^GC stats:"}}, nil)
	require.NoError(t, err)

	tests := []struct {
		line     string
		expected bool
	}{
		{"INFO: starting up", true},
		{"heartbeat: ok", false},
		{"System heartbeat check", false},
		{"GC stats: 100ms", false},
		{"Not GC stats: something", true},
		{"ERROR: connection failed", true},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, f.ShouldInclude(tt.line))
		})
	}
}

func TestFilter_IncludePatterns(t *testing.T) {
	t.Parallel()

	f, err := New(Config{IncludePatterns: []string{"important", "^ERROR:"}}, nil)
	require.NoError(t, err)

	tests := []struct {
		line     string
		expected bool
	}{
		{"important: system update", true},
		{"very important message", true},
		{"ERROR: connection failed", true},
		{"regular log line", false},
		{"INFO: nothing special", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, f.ShouldInclude(tt.line))
		})
	}
}

func TestFilter_ExcludeLevels(t *testing.T) {
	t.Parallel()

	f, err := New(Config{ExcludeLevels: []string{"DEBUG", "TRACE"}}, testKeywords)
	require.NoError(t, err)

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{"error line", "ERROR: something failed", true},
		{"warn line", "WARN: disk space low", true},
		{"info line", "INFO: started", true},
		{"debug line", "DEBUG: variable dump", false},
		{"trace line", "TRACE: detailed output", false},
		{"no keyword", "regular message", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, f.ShouldInclude(tt.line))
		})
	}
}

func TestFilter_IncludeLevels(t *testing.T) {
	t.Parallel()

	f, err := New(Config{IncludeLevels: []string{"ERROR", "WARN"}}, testKeywords)
	require.NoError(t, err)

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{"error line", "ERROR: something failed", true},
		{"fatal line", "FATAL: crash", true},
		{"warn line", "WARN: disk space low", true},
		{"info line", "INFO: started", false},
		{"debug line", "DEBUG: variable dump", false},
		{"no keyword", "regular message", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, f.ShouldInclude(tt.line))
		})
	}
}

func TestFilter_CombinedRules(t *testing.T) {
	t.Parallel()

	f, err := New(Config{
		IncludeLevels:   []string{"ERROR", "WARN"},
		ExcludePatterns: []string{"heartbeat"},
	}, testKeywords)
	require.NoError(t, err)

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{"error passes", "ERROR: connection failed", true},
		{"warn passes", "WARN: disk low", true},
		{"info excluded by level", "INFO: started", false},
		{"error heartbeat excluded by pattern", "ERROR: heartbeat timeout", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, f.ShouldInclude(tt.line))
		})
	}
}

func TestFilter_EmptyConfig(t *testing.T) {
	t.Parallel()

	f, err := New(Config{}, nil)
	require.NoError(t, err)

	// Empty filter should include everything.
	assert.True(t, f.ShouldInclude("anything"))
	assert.True(t, f.ShouldInclude("ERROR: something"))
	assert.True(t, f.ShouldInclude(""))
}

func TestFilter_CaseInsensitiveLevels(t *testing.T) {
	t.Parallel()

	f, err := New(Config{ExcludeLevels: []string{"debug"}}, testKeywords)
	require.NoError(t, err)

	assert.False(t, f.ShouldInclude("DEBUG: variable dump"))
	assert.True(t, f.ShouldInclude("ERROR: failed"))
}
