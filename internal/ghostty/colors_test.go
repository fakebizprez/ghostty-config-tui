package ghostty

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseThemeColors(t *testing.T) {
	in := "# a comment\n" +
		"background = #282a36\n" +
		"foreground = #f8f8f2\n" +
		"cursor-color = #ff0000\n" +
		"palette = 0=#21222c\n" +
		"palette = 1=#ff5555\n" +
		"palette = 15=#ffffff\n" +
		"palette = 99=#000000\n" +
		"palette = nope\n" +
		"background = notacolor\n" +
		"unmanaged = keep\n"
	c := ParseThemeColors(in)
	if c.Background != "#282a36" || c.Foreground != "#f8f8f2" || c.Cursor != "#ff0000" {
		t.Fatalf("colors = %+v", c)
	}
	if c.Palette[0] != "#21222c" || c.Palette[1] != "#ff5555" || c.Palette[15] != "#ffffff" {
		t.Fatalf("palette = %v", c.Palette)
	}
	if c.Palette[2] != "" {
		t.Fatalf("unset slot should stay empty, got %q", c.Palette[2])
	}
}

func TestThemeColorsWithDefaults(t *testing.T) {
	c := ThemeColors{Background: "#111111"}.WithDefaults()
	if c.Background != "#111111" {
		t.Fatalf("explicit background lost: %q", c.Background)
	}
	if c.Foreground == "" || c.Palette[0] == "" || c.Palette[15] == "" {
		t.Fatalf("defaults not filled: %+v", c)
	}
	if c.Cursor != c.Foreground {
		t.Fatalf("cursor should fall back to foreground, got %q", c.Cursor)
	}
}

func TestParseThemePaths(t *testing.T) {
	out := "Dracula (resources) /Applications/Ghostty.app/Contents/Resources/ghostty/themes/Dracula\n" +
		"My (Paren) Theme (custom) /tmp/themes/My (Paren) Theme\n" +
		"Bad Line Without Path\n"
	got := ParseThemePaths(out)
	if len(got) != 2 {
		t.Fatalf("paths = %v", got)
	}
	if got[0].Name != "Dracula" || got[0].Source != "resources" {
		t.Fatalf("first = %+v", got[0])
	}
	if got[1].Name != "My (Paren) Theme" || got[1].Path != "/tmp/themes/My (Paren) Theme" {
		t.Fatalf("second = %+v", got[1])
	}
}

func TestResolveThemeColorsMissingBinary(t *testing.T) {
	oldLook := lookPath
	defer func() { lookPath = oldLook }()
	lookPath = func(string) (string, error) { return "", errors.New("not found") }
	if _, ok := ResolveThemeColors("Dracula"); ok {
		t.Fatal("missing binary should not resolve")
	}
}

func TestResolveThemeColorsFromList(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "MyTheme"), []byte("background = #010203\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldLook, oldRun := lookPath, runCommand
	defer func() { lookPath, runCommand = oldLook, oldRun }()
	lookPath = func(string) (string, error) { return "/usr/bin/ghostty", nil }
	runCommand = func(context.Context, string, ...string) ([]byte, error) {
		return []byte("MyTheme (custom) " + filepath.Join(dir, "MyTheme") + "\n"), nil
	}
	c, ok := ResolveThemeColors("MyTheme")
	if !ok {
		t.Fatal("should resolve theme from stubbed list")
	}
	if c.Background != "#010203" {
		t.Fatalf("background = %q", c.Background)
	}
	if c.Foreground == "" || c.Palette[0] == "" {
		t.Fatalf("defaults should fill gaps: %+v", c)
	}
	if _, ok := ResolveThemeColors("Missing"); ok {
		t.Fatal("unknown theme should not resolve")
	}
}
