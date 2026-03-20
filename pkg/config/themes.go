package config

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sgaunet/logwrap/pkg/apperrors"
)

// ColorTheme defines a predefined set of color assignments for log output.
type ColorTheme struct {
	Info      string
	Error     string
	Timestamp string
}

// predefinedThemes maps theme names to their color configurations.
// All color values use the names accepted by the formatter's getColorCode
// function (black, red, green, yellow, blue, magenta, cyan, white, none).
var predefinedThemes = map[string]ColorTheme{
	"default": {
		Info:      "green",
		Error:     "red",
		Timestamp: "blue",
	},
	"warm": {
		Info:      "yellow",
		Error:     "red",
		Timestamp: "magenta",
	},
	"cool": {
		Info:      "cyan",
		Error:     "magenta",
		Timestamp: "blue",
	},
	"ocean": {
		Info:      "cyan",
		Error:     "red",
		Timestamp: "blue",
	},
	"monochrome": {
		Info:      "white",
		Error:     "white",
		Timestamp: "white",
	},
}

// applyTheme sets the color fields from the named theme, overriding any
// previously set values. This is called before validation so theme colors
// get validated. Individual color fields set in YAML/CLI after this call
// will override the theme values.
func applyTheme(colors *ColorsConfig, themeName string) error {
	theme, ok := predefinedThemes[strings.ToLower(themeName)]
	if !ok {
		return fmt.Errorf("%w %q, available: %s",
			apperrors.ErrInvalidColorTheme, themeName, strings.Join(ThemeNames(), ", "))
	}

	colors.Info = theme.Info
	colors.Error = theme.Error
	colors.Timestamp = theme.Timestamp

	return nil
}

// applyThemeWithOverrides applies a theme and then restores any color fields
// that were explicitly set in the config file.
func applyThemeWithOverrides(colors *ColorsConfig, explicit explicitColorFields) error {
	saved := *colors

	if err := applyTheme(colors, colors.Theme); err != nil {
		return err
	}

	if explicit.info {
		colors.Info = saved.Info
	}
	if explicit.errColor {
		colors.Error = saved.Error
	}
	if explicit.timestamp {
		colors.Timestamp = saved.Timestamp
	}

	return nil
}

// ThemeNames returns the sorted list of available theme names.
func ThemeNames() []string {
	names := make([]string, 0, len(predefinedThemes))
	for name := range predefinedThemes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
