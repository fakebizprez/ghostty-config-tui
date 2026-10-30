package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoldenRoundTrip(t *testing.T) {
	for _, name := range []string{"full.conf", "minimal.conf", "comments.conf"} {
		p := filepath.Join("..", "..", "testdata", name)
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := Parse(string(b)).Marshal(); got != string(b) {
			t.Fatalf("%s round-trip changed bytes:\n%q\nvs\n%q", name, got, b)
		}
	}
}

func TestRoundTripPreservesUnmanaged(t *testing.T) {
	in := "# comment\nfont-size = 12\n\nkeybinding = ctrl+c=copy\nbackground = #000000\n"
	f := Parse(in)
	if f.Get("font-size") != "12" {
		t.Fatalf("Get = %q", f.Get("font-size"))
	}
	out := f.Marshal()
	if out != in {
		t.Fatalf("round-trip changed bytes:\n%q\nvs\n%q", out, in)
	}
}

func TestWriterSingleKeyUntouchedOthers(t *testing.T) {
	f := Parse("font-size = 12\nbackground = #000000\n")
	f.Set("font-size", "14")
	out := f.Marshal()
	if !strings.Contains(out, "font-size = 14") || !strings.Contains(out, "background = #000000") {
		t.Fatalf("writer touched other lines: %q", out)
	}
}

func TestWriterAppendsMissing(t *testing.T) {
	f := Parse("font-size = 12\n")
	f.Set("theme", "Dracula")
	if f.Get("theme") != "Dracula" {
		t.Fatalf("append failed: %q", f.Marshal())
	}
}

func TestResolvePathXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
	t.Setenv("HOME", "/tmp/home-test")
	got := ResolveConfigPath()
	if got != "/tmp/xdg-test/ghostty/config" && got != "/tmp/xdg-test/ghostty/config.ghostty" {
		t.Fatalf("XDG path = %q", got)
	}
}

func TestAtomicWriteKeepsPerms(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config")
	if err := os.WriteFile(p, []byte("font-size = 12\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(p, "font-size = 14\n", 0); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "font-size = 14\n" {
		t.Fatalf("write = %q", b)
	}
	fi, _ := os.Stat(p)
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("perms = %o", fi.Mode().Perm())
	}
}

func TestUndoRestoreAndMissing(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config")
	if err := os.WriteFile(p, []byte("a = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	snap, err := Snapshot(p)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(p, []byte("a = 2\n"), 0o644)
	if err := snap.Restore(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "a = 1\n" {
		t.Fatalf("undo = %q", b)
	}
	missing := filepath.Join(dir, "none")
	snap2, err := Snapshot(missing)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(missing, []byte("x\n"), 0o644)
	if err := snap2.Restore(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("missing-file undo should remove file")
	}
}
