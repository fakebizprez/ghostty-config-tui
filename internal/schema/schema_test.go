package schema

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// GHOSTTY_SCHEMA_DUMP=1 go test ./internal/schema -run TestDump -v prints
// the inferred registry for review after refreshing the snapshot.
func TestDump(t *testing.T) {
	if os.Getenv("GHOSTTY_SCHEMA_DUMP") == "" {
		t.Skip("set GHOSTTY_SCHEMA_DUMP=1")
	}
	for _, s := range All() {
		fmt.Printf("%-16s %-40s %-8s r=%-5v p=%v since=%s def=%q opts=%s\n", s.Category, s.Key, s.Kind, s.Repeatable, s.Platforms, s.Since, s.Default, strings.Join(s.Options, "|"))
	}
}

func TestEverySettingIsComplete(t *testing.T) {
	if n := len(All()); n < 180 {
		t.Fatalf("only %d settings parsed from the bundled Ghostty docs", n)
	}
	cats := map[string]bool{}
	for _, c := range Categories {
		cats[c] = true
	}
	for _, s := range All() {
		if s.Title == "" || s.Kind == "" || !cats[s.Category] {
			t.Errorf("%s incomplete: %+v", s.Key, s)
		}
		if s.Docs == "" {
			t.Errorf("%s has no docs", s.Key)
		}
		if (s.Kind == Enum || s.Kind == Flags) && len(s.Options) < 2 {
			t.Errorf("%s is %s with options %v", s.Key, s.Kind, s.Options)
		}
	}
}

func TestInferredKinds(t *testing.T) {
	for key, want := range map[string]Kind{
		"theme": Theme, "font-family": Font, "font-size": Float, "background": Color,
		"background-opacity": Float, "cursor-style": Enum, "cursor-style-blink": Bool,
		"window-decoration": Enum, "bell-features": Flags, "shell-integration-features": Flags,
		"resize-overlay-duration": Duration, "keybind": Keybind, "palette": Palette,
		"scrollback-limit-bytes": Int, "macos-titlebar-style": Enum, "command": String,
		"cursor-color": Color, "copy-on-select": Enum,
	} {
		s, ok := ByKey(key)
		if !ok || s.Kind != want {
			t.Errorf("%s kind = %q, want %q", key, s.Kind, want)
		}
	}
	shell, _ := ByKey("shell-integration")
	if strings.Join(shell.Options, ",") != "none,detect,bash,elvish,fish,nushell,zsh" {
		t.Errorf("shell-integration options = %v", shell.Options)
	}
}

func TestMetadata(t *testing.T) {
	kb, _ := ByKey("keybind")
	if !kb.Repeatable || len(kb.Defaults) < 20 {
		t.Errorf("keybind repeatable=%v defaults=%d", kb.Repeatable, len(kb.Defaults))
	}
	if ff, _ := ByKey("font-feature"); !ff.Repeatable {
		t.Error("font-feature should be repeatable")
	}
	if s, _ := ByKey("macos-titlebar-style"); strings.Join(s.Platforms, ",") != "macOS" {
		t.Errorf("macos platforms = %v", s.Platforms)
	}
	if s, _ := ByKey("language"); s.Since != "1.3.0" || strings.Join(s.Platforms, ",") != "GTK" {
		t.Errorf("language since=%q platforms=%v", s.Since, s.Platforms)
	}
	if s, _ := ByKey("window-decoration"); s.Since != "" {
		t.Errorf("per-option 'Available since' must not apply to the setting, got %q", s.Since)
	}
	if s, _ := ByKey("custom-shader-animation"); s.Category != "Shaders" {
		t.Errorf("custom-shader-animation category = %q", s.Category)
	}
}

func TestSearchRanksKeyMatchesFirst(t *testing.T) {
	got := Active().Search("opacity")
	if len(got) < 4 || !strings.Contains(got[0].Key, "opacity") {
		t.Fatalf("search opacity = %v", keys(got))
	}
	if got := Active().Search("hollow"); len(got) == 0 || got[0].Key != "cursor-style" {
		t.Fatalf("docs search hollow = %v", keys(got))
	}
	if Active().Search("") != nil {
		t.Fatal("empty search should return nil")
	}
}

func keys(ss []Setting) []string {
	var out []string
	for _, s := range ss {
		out = append(out, s.Key)
	}
	return out
}

func TestUseFallsBackOnGarbage(t *testing.T) {
	before := len(All())
	Use("not a config", "")
	if len(All()) != before {
		t.Fatal("garbage docs replaced the schema")
	}
}

func TestValidate(t *testing.T) {
	get := func(k string) Setting { s, _ := ByKey(k); return s }
	cases := []struct {
		key, in, want string
		err, warn     bool
	}{
		{"font-size", "13.5", "13.5", false, false},
		{"font-size", "big", "", true, false},
		{"font-size", "900", "", true, false},
		{"background-opacity", "1.5", "", true, false},
		{"scrollback-limit-bytes", "1.5", "", true, false},
		{"background", "FFAA00", "#ffaa00", false, false},
		{"background", "#abc", "#aabbcc", false, false},
		{"background", "rebeccapurple", "rebeccapurple", false, true},
		{"background", "#12", "", true, false},
		{"cursor-color", "cell-foreground", "cell-foreground", false, false},
		{"cursor-style", "bar", "bar", false, false},
		{"cursor-style", "triangle", "triangle", false, true},
		{"bell-features", "audio,no-title", "audio,no-title", false, false},
		{"bell-features", "bogus", "bogus", false, true},
		{"resize-overlay-duration", "1h30m", "1h30m", false, false},
		{"resize-overlay-duration", "soon", "", true, false},
		{"palette", "3=#FF0000", "3=#ff0000", false, false},
		{"palette", "300=#ff0000", "", true, false},
		{"keybind", "ctrl+shift+t=new_tab", "ctrl+shift+t=new_tab", false, false},
		{"keybind", "global:cmd+grave_accent=toggle_quick_terminal", "global:cmd+grave_accent=toggle_quick_terminal", false, false},
		{"keybind", "ctrl+a>n=new_window", "ctrl+a>n=new_window", false, false},
		{"keybind", "hyper+x=new_tab", "", true, false},
		{"keybind", "ctrl+x=launch_rockets", "ctrl+x=launch_rockets", false, true},
		{"keybind", "clear", "clear", false, false},
		{"cursor-style-blink", "yes", "true", false, false},
		{"theme", "", "", false, false},
	}
	for _, c := range cases {
		got, warn, err := Validate(get(c.key), c.in)
		if (err != nil) != c.err || (warn != "") != c.warn || (err == nil && got != c.want) {
			t.Errorf("Validate(%s, %q) = %q warn=%q err=%v", c.key, c.in, got, warn, err)
		}
	}
}

func TestKeybindRoundTrip(t *testing.T) {
	kb, err := ParseKeybind("all:unconsumed:ctrl+shift+plus=increase_font_size:1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(kb.Prefixes, ",") != "all,unconsumed" || kb.Trigger != "ctrl+shift+plus" || kb.Action != "increase_font_size" || kb.Param != "1" {
		t.Fatalf("parsed %+v", kb)
	}
	if kb.String() != "all:unconsumed:ctrl+shift+plus=increase_font_size:1" {
		t.Fatalf("String() = %q", kb.String())
	}
}

func TestTriggerFromKey(t *testing.T) {
	for in, want := range map[string]string{
		"ctrl+shift+t": "ctrl+shift+t", "meta+k": "super+k", "alt+pgup": "alt+page_up",
		"ctrl+space": "ctrl+space", "f5": "f5",
	} {
		if got := TriggerFromKey(in); got != want {
			t.Errorf("TriggerFromKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFlagState(t *testing.T) {
	def := "no-system,no-audio,attention,title,no-border"
	if FlagState("", def, "audio") || !FlagState("", def, "title") {
		t.Fatal("defaults misread")
	}
	if !FlagState("audio", def, "audio") || FlagState("no-title", def, "title") {
		t.Fatal("explicit value should override default")
	}
	if FlagState("false", def, "title") || !FlagState("true", def, "audio") {
		t.Fatal("true/false should set every flag")
	}
}
