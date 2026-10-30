// Package schema is the typed setting source of truth for v1 managed keys.
package schema

import (
	"strconv"
	"strings"
)

// Setting describes one managed Ghostty config key.
type Setting struct {
	Key     string
	Title   string
	Type    string // string, int, float, color, enum, bool
	Default string
	Help    string
	Options []string // enum choices only
	Min     string
	Max     string
}

var all = []Setting{
	{Key: "theme", Title: "Theme", Type: "string", Default: "", Help: "Color theme name from `ghostty +list-themes` (empty = Ghostty default)."},
	{Key: "font-family", Title: "Font Family", Type: "string", Default: "", Help: "Font family from `ghostty +list-fonts` (empty = Ghostty default)."},
	{Key: "font-size", Title: "Font Size", Type: "int", Default: "12", Min: "4", Max: "72", Help: "Font size in points (4-72)."},
	{Key: "background", Title: "Background", Type: "color", Default: "#000000", Help: "Background color as #RRGGBB or #RRGGBBAA."},
	{Key: "foreground", Title: "Foreground", Type: "color", Default: "#ffffff", Help: "Foreground (text) color as #RRGGBB or #RRGGBBAA."},
	{Key: "background-opacity", Title: "Background Opacity", Type: "float", Default: "1", Min: "0", Max: "1", Help: "Background opacity from 0 (transparent) to 1 (opaque)."},
	{Key: "cursor-style", Title: "Cursor Style", Type: "enum", Default: "block", Options: []string{"block", "bar", "underline"}, Help: "Cursor shape: block, bar, or underline."},
	{Key: "cursor-style-blink", Title: "Cursor Blink", Type: "bool", Default: "false", Help: "Whether the cursor blinks (true/false)."},
}

// All returns all v1 settings in stable sidebar order.
func All() []Setting { return append([]Setting(nil), all...) }

// ByKey looks up a setting by its config key.
func ByKey(key string) (Setting, bool) {
	for _, s := range all {
		if s.Key == key {
			return s, true
		}
	}
	return Setting{}, false
}

// Defaults returns the default value per managed key.
func Defaults() map[string]string {
	m := make(map[string]string, len(all))
	for _, s := range all {
		m[s.Key] = s.Default
	}
	return m
}

// Search is a token-AND, case-insensitive search over key+title.
func Search(q string) []Setting {
	toks := strings.Fields(strings.ToLower(q))
	if len(toks) == 0 {
		return All()
	}
	var out []Setting
	for _, s := range all {
		hay := strings.ToLower(s.Key + " " + s.Title + " " + s.Help)
		match := true
		for _, tok := range toks {
			if !strings.Contains(hay, tok) {
				match = false
				break
			}
		}
		if match {
			out = append(out, s)
		}
	}
	return out
}

// ClampFontSize clamps to 4-72.
func ClampFontSize(n int) int {
	if n < 4 {
		return 4
	}
	if n > 72 {
		return 72
	}
	return n
}

// ClampOpacity clamps to 0-1.
func ClampOpacity(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// NormalizeColor lowercases a color value.
func NormalizeColor(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// ValidateColor accepts #RRGGBB or #RRGGBBAA.
func ValidateColor(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != 7 && len(s) != 9 {
		return false
	}
	if !strings.HasPrefix(s, "#") {
		return false
	}
	for _, c := range s[1:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// ParseBool parses Ghostty bools (true/false/1/0/yes/no).
func ParseBool(s string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes":
		return true, true
	case "false", "0", "no":
		return false, true
	}
	return false, false
}

// ParseFloat parses a float value.
func ParseFloat(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return f, true
}
