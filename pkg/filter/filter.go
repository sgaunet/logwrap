// Package filter provides line filtering for logwrap output streams.
//
// The filter evaluates each raw log line against configured rules before
// formatting. Lines that do not pass the filter are silently dropped.
//
// # Filter Rules
//
// Rules are evaluated in this order:
//  1. Include levels (if set, line must match one)
//  2. Exclude levels (if set, line must not match any)
//  3. Include patterns (if set, line must match one)
//  4. Exclude patterns (if set, line must not match any)
//
// All rules that are configured must pass for the line to be included.
// An empty/disabled filter includes all lines.
package filter

import (
	"fmt"
	"regexp"
	"strings"
)

// Config holds the filter configuration.
type Config struct {
	Enabled         bool     `yaml:"enabled"`
	ExcludePatterns []string `yaml:"exclude_patterns"`
	IncludePatterns []string `yaml:"include_patterns"`
	ExcludeLevels   []string `yaml:"exclude_levels"`
	IncludeLevels   []string `yaml:"include_levels"`
}

// Filter evaluates log lines against configured include/exclude rules.
type Filter struct {
	excludePatterns []*regexp.Regexp
	includePatterns []*regexp.Regexp
	excludeLevels   map[string]bool
	includeLevels   map[string]bool
	// levelKeywords maps uppercase level names to their detection keywords.
	// Used to check whether a line "is" at a given level.
	levelKeywords map[string][]string
}

// New creates a Filter from the given config and detection keywords.
// The keywords map keys are lowercase level names (e.g., "error") and
// values are the keywords that indicate that level (e.g., ["ERROR", "FATAL"]).
func New(cfg Config, keywords map[string][]string) (*Filter, error) {
	f := &Filter{
		excludeLevels: make(map[string]bool),
		includeLevels: make(map[string]bool),
		levelKeywords: make(map[string][]string),
	}

	for _, p := range cfg.ExcludePatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid exclude pattern %q: %w", p, err)
		}
		f.excludePatterns = append(f.excludePatterns, re)
	}

	for _, p := range cfg.IncludePatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid include pattern %q: %w", p, err)
		}
		f.includePatterns = append(f.includePatterns, re)
	}

	for _, level := range cfg.ExcludeLevels {
		f.excludeLevels[strings.ToUpper(level)] = true
	}
	for _, level := range cfg.IncludeLevels {
		f.includeLevels[strings.ToUpper(level)] = true
	}

	// Store keywords for level detection, keyed by uppercase level name.
	for level, kws := range keywords {
		f.levelKeywords[strings.ToUpper(level)] = kws
	}

	return f, nil
}

// ShouldInclude returns true if the line passes all configured filter rules.
func (f *Filter) ShouldInclude(line string) bool {
	if !f.passesLevelFilter(line) {
		return false
	}
	if !f.passesPatternFilter(line) {
		return false
	}
	return true
}

func (f *Filter) passesLevelFilter(line string) bool {
	if len(f.includeLevels) == 0 && len(f.excludeLevels) == 0 {
		return true
	}

	detectedLevel := f.detectLevel(strings.ToUpper(line))
	if len(f.includeLevels) > 0 && !f.includeLevels[detectedLevel] {
		return false
	}
	return !f.excludeLevels[detectedLevel]
}

func (f *Filter) passesPatternFilter(line string) bool {
	if len(f.includePatterns) > 0 && !f.matchesAny(line, f.includePatterns) {
		return false
	}
	return !f.matchesAny(line, f.excludePatterns)
}

func (f *Filter) matchesAny(line string, patterns []*regexp.Regexp) bool {
	for _, re := range patterns {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

// detectLevel returns the uppercase level name for a line, or empty string if none detected.
// Uses the same keyword scanning approach as the formatter but with simplified priority.
func (f *Filter) detectLevel(lineUpper string) string {
	// Check levels in deterministic priority order (most to least severe).
	priorities := []string{"FATAL", "ERROR", "WARN", "INFO", "DEBUG", "TRACE"}
	for _, level := range priorities {
		keywords := f.levelKeywords[level]
		for _, kw := range keywords {
			if strings.Contains(lineUpper, strings.ToUpper(kw)) {
				return level
			}
		}
	}
	return ""
}
