package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ghostty-config-tui/internal/ui"
)

func TestParseFlagsDefaults(t *testing.T) {
	f, rest, err := parseFlags([]string{})
	if err != nil || len(rest) != 0 {
		t.Fatalf("defaults err=%v rest=%v", err, rest)
	}
	if f.print || f.dryRun || f.version || f.help {
		t.Fatalf("defaults all false, got %+v", f)
	}
}

func TestParseFlagsEach(t *testing.T) {
	for _, tc := range []struct {
		arg   string
		check func(flags) bool
	}{
		{"--print", func(f flags) bool { return f.print }},
		{"--dry-run", func(f flags) bool { return f.dryRun }},
		{"--version", func(f flags) bool { return f.version }},
		{"--help", func(f flags) bool { return f.help }},
	} {
		f, _, err := parseFlags([]string{tc.arg})
		if err != nil || !tc.check(f) {
			t.Fatalf("%s parsed as %+v err=%v", tc.arg, f, err)
		}
	}
}

func TestMain(m *testing.M) {
	// Keep tests on the bundled schema regardless of the installed Ghostty.
	loadSchema = func() string { return "" }
	os.Exit(m.Run())
}

func TestParseFlagsConfig(t *testing.T) {
	for _, args := range [][]string{{"--config", "/x/c"}, {"--config=/x/c"}} {
		f, _, err := parseFlags(args)
		if err != nil || f.config != "/x/c" {
			t.Fatalf("%v parsed as %+v err=%v", args, f, err)
		}
	}
	if _, _, err := parseFlags([]string{"--config"}); err == nil {
		t.Fatal("--config without a path should error")
	}
}

func TestSaveRepeatableAndRemove(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config")
	orig := "keybind = ctrl+a=new_tab\n# note\nfont-size = 12\nkeybind = ctrl+b=close_surface\nmystery = 1\n"
	if err := os.WriteFile(p, []byte(orig), 0o600); err != nil {
		t.Fatal(err)
	}
	opts, err := newSession(p, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := opts.Values["keybind"]; len(got) != 2 {
		t.Fatalf("loaded keybinds = %v", got)
	}
	if _, ok := opts.Values["mystery"]; ok {
		t.Fatal("unknown keys must not be loaded as managed values")
	}
	_, err = opts.OnSave(map[string][]string{
		"keybind":   {"ctrl+a=new_tab", "ctrl+shift+t=new_window"},
		"font-size": nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	want := "keybind = ctrl+a=new_tab\nkeybind = ctrl+shift+t=new_window\n# note\nmystery = 1\n"
	if string(b) != want {
		t.Fatalf("saved:\n%s\nwant:\n%s", b, want)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
		t.Fatalf("permissions changed to %v", fi.Mode().Perm())
	}
	if _, err := opts.OnUndo(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != orig {
		t.Fatalf("undo restored:\n%s", b)
	}
	if _, err := opts.OnUndo(); err == nil {
		t.Fatal("second undo with no new save should error")
	}
}

func TestDryRunSaveDoesNotWrite(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config")
	os.WriteFile(p, []byte("font-size = 12\n"), 0o644)
	opts, _ := newSession(p, true)
	if _, err := opts.OnSave(map[string][]string{"font-size": {"20"}}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "font-size = 12\n" {
		t.Fatalf("dry-run wrote: %s", b)
	}
}

func TestRunExport(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config")
	os.WriteFile(p, []byte("cursor-style = bar\nfont-size = 15\nmystery = 1\n"), 0o644)
	var out, errBuf bytes.Buffer
	if code := run(&out, &errBuf, []string{"--export", "--config", p}, false); code != 0 {
		t.Fatalf("export exit %d: %s", code, errBuf.String())
	}
	want := "# Fonts\nfont-size = 15\n\n# Cursor\ncursor-style = bar\n"
	if out.String() != want {
		t.Fatalf("export = %q, want %q", out.String(), want)
	}
}

func TestParseFlagsUnknown(t *testing.T) {
	if _, _, err := parseFlags([]string{"--bogus"}); err == nil {
		t.Fatal("unknown flag should error")
	}
}

func TestPrintConfigSchemaOrder(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config")
	content := "foreground = #ffffff\n# comment\ntheme = Dracula\nfont-size = 14\nunmanaged = keep\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := printConfig(&buf, p); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("print = %q, want 3 managed lines", buf.String())
	}
	// Ghostty's own option order: font-size, then theme, then foreground.
	themeIdx, sizeIdx, fgIdx := -1, -1, -1
	for i, l := range lines {
		switch {
		case strings.HasPrefix(l, "theme "):
			themeIdx = i
		case strings.HasPrefix(l, "font-size "):
			sizeIdx = i
		case strings.HasPrefix(l, "foreground "):
			fgIdx = i
		}
	}
	if sizeIdx != 0 || themeIdx != 1 || fgIdx != 2 {
		t.Fatalf("schema order wrong: %q", buf.String())
	}
}

func TestPrintConfigMissingErrors(t *testing.T) {
	var buf bytes.Buffer
	if err := printConfig(&buf, filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("missing file should error")
	}
}

func TestRunVersion(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := run(&out, &errBuf, []string{"--version"}, true); code != 0 {
		t.Fatalf("version exit = %d", code)
	}
	if !strings.Contains(out.String(), "0.2.0") {
		t.Fatalf("version output = %q", out.String())
	}
}

func TestRunHelp(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := run(&out, &errBuf, []string{"--help"}, true); code != 0 {
		t.Fatalf("help exit = %d", code)
	}
	if !strings.Contains(out.String(), "ctrl+shift+r") {
		t.Fatalf("help missing reload hint: %q", out.String())
	}
}

func TestRunUnknownFlagExit2(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := run(&out, &errBuf, []string{"--bogus"}, true); code != 2 {
		t.Fatalf("unknown exit = %d, stderr=%q", code, errBuf.String())
	}
}

func TestRunSaveUndoRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	cfgDir := filepath.Join(dir, "ghostty")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(cfgDir, "config.ghostty")
	orig := "# keep me\nfont-size = 12\nunmanaged = keep\n"
	if err := os.WriteFile(cfg, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	oldStart := startProgram
	defer func() { startProgram = oldStart }()
	var got ui.Options
	startProgram = func(o ui.Options) error { got = o; return nil }
	var out, errBuf bytes.Buffer
	if code := run(&out, &errBuf, []string{}, true); code != 0 {
		t.Fatalf("run exit = %d, stderr=%q", code, errBuf.String())
	}
	if _, err := got.OnSave(map[string][]string{"font-size": {"14"}}); err != nil {
		t.Fatal(err)
	}
	saved, _ := os.ReadFile(cfg)
	if !strings.Contains(string(saved), "font-size = 14") || !strings.Contains(string(saved), "unmanaged = keep") {
		t.Fatalf("save should rewrite only managed keys, got:\n%s", saved)
	}
	restored, err := got.OnUndo()
	if err != nil {
		t.Fatal(err)
	}
	if len(restored["font-size"]) != 1 || restored["font-size"][0] != "12" {
		t.Fatalf("undo values = %v, want font-size 12", restored)
	}
	after, _ := os.ReadFile(cfg)
	if string(after) != orig {
		t.Fatalf("undo should restore exact bytes, got:\n%s", after)
	}
}

func TestFailedSaveDoesNotLeakIntoNextSave(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config")
	os.WriteFile(p, []byte("font-size = 12\n"), 0o644)
	opts, _ := newSession(p, false)
	// Make the write fail by replacing the target with a non-empty directory.
	os.Remove(p)
	os.MkdirAll(filepath.Join(p, "x"), 0o755)
	if _, err := opts.OnSave(map[string][]string{"font-size": {"20"}}); err == nil {
		t.Fatal("expected the save to fail")
	}
	os.RemoveAll(p)
	if _, err := opts.OnSave(map[string][]string{"theme": {"Nord"}}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "font-size = 12\ntheme = Nord\n" {
		t.Fatalf("reverted change leaked into the next save:\n%s", b)
	}
}

func TestRunShare(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config")
	os.WriteFile(p, []byte("theme = Nord\n"), 0o644)
	var out, errBuf bytes.Buffer
	if code := run(&out, &errBuf, []string{"--share", "--config", p}, false); code != 0 {
		t.Fatalf("share exit %d: %s", code, errBuf.String())
	}
	if got := strings.TrimSpace(out.String()); got != "https://ghostty.zerebos.com/app/import-export#share=dGhlbWUgPSBOb3JkCg" {
		t.Fatalf("share = %q", got)
	}
}
