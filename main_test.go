package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	// Schema order: theme before font-family/font-size; font-size before foreground.
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
	if themeIdx != 0 || sizeIdx != 1 || fgIdx != 2 {
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
	if !strings.Contains(out.String(), "0.1.0") {
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
