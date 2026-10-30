// Theme color files: `ghostty +list-themes --path` locations and parsing.
package ghostty

import (
	"context"
	"os"
	"strconv"
	"strings"

	"ghostty-config-tui/internal/schema"
)

// ThemeColors holds a theme's live preview colors.
type ThemeColors struct {
	Background string
	Foreground string
	Cursor     string
	Palette    [16]string
}

// DefaultPalette is the fallback 16-color swatch row.
var DefaultPalette = [16]string{
	"#000000", "#800000", "#008000", "#808000",
	"#000080", "#800080", "#008080", "#c0c0c0",
	"#808080", "#ff0000", "#00ff00", "#ffff00",
	"#0000ff", "#ff00ff", "#00ffff", "#ffffff",
}

// WithDefaults fills unset colors from Ghostty defaults and the ANSI palette.
func (c ThemeColors) WithDefaults() ThemeColors {
	if !schema.ValidateColor(c.Background) {
		c.Background = schema.Defaults()["background"]
	}
	if !schema.ValidateColor(c.Foreground) {
		c.Foreground = schema.Defaults()["foreground"]
	}
	for i := range c.Palette {
		if !schema.ValidateColor(c.Palette[i]) {
			c.Palette[i] = DefaultPalette[i]
		}
	}
	return c
}

// ParseThemeColors reads background/foreground/cursor-color and
// `palette = N=#hex` lines. Invalid lines are ignored.
func ParseThemeColors(text string) ThemeColors {
	var c ThemeColors
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		// Strip inline comments: `palette = 1=#ff5555 # red`.
		if i := strings.Index(val, "# "); i >= 0 {
			val = strings.TrimSpace(val[:i])
		}
		switch key {
		case "background", "foreground", "cursor-color":
			if !schema.ValidateColor(val) {
				continue
			}
			if key == "background" {
				c.Background = val
			} else if key == "foreground" {
				c.Foreground = val
			} else {
				c.Cursor = val
			}
		case "palette":
			slot, color, ok := strings.Cut(val, "=")
			if !ok {
				continue
			}
			n, err := strconv.Atoi(strings.TrimSpace(slot))
			if err != nil || n < 0 || n > 15 {
				continue
			}
			color = strings.TrimSpace(color)
			if !schema.ValidateColor(color) {
				continue
			}
			c.Palette[n] = color
		}
	}
	return c
}

// ThemeFile names one installed theme and its file on disk.
type ThemeFile struct {
	Name   string
	Source string
	Path   string
}

// ParseThemePaths parses `ghostty +list-themes --path` output,
// whose lines look like `Dracula (resources) /abs/path/Dracula`.
func ParseThemePaths(out string) []ThemeFile {
	var files []ThemeFile
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		i := strings.LastIndex(line, " /")
		if i < 0 {
			continue
		}
		head, path := strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
		if path == "" {
			continue
		}
		name := head
		source := ""
		if j := strings.LastIndex(head, " ("); j >= 0 && strings.HasSuffix(head, ")") {
			name = strings.TrimSpace(head[:j])
			source = head[j+2 : len(head)-1]
		}
		if name == "" {
			continue
		}
		files = append(files, ThemeFile{Name: name, Source: source, Path: path})
	}
	return files
}

// themePaths lists installed theme files via the Ghostty binary.
func themePaths() ([]ThemeFile, error) {
	if _, err := lookPath("ghostty"); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	out, err := runCommand(ctx, "ghostty", "+list-themes", "--path")
	if err != nil {
		return nil, err
	}
	return ParseThemePaths(string(out)), nil
}

// ResolveThemeColors loads a theme's colors by name (case-sensitive).
// ok is false when Ghostty is missing, the theme is unknown, or its file
// cannot be read. Returned colors always carry defaults.
func ResolveThemeColors(name string) (ThemeColors, bool) {
	files, err := themePaths()
	if err != nil {
		return ThemeColors{}.WithDefaults(), false
	}
	for _, f := range files {
		if f.Name != name {
			continue
		}
		b, err := os.ReadFile(f.Path)
		if err != nil {
			return ThemeColors{}.WithDefaults(), false
		}
		return ParseThemeColors(string(b)).WithDefaults(), true
	}
	return ThemeColors{}.WithDefaults(), false
}
