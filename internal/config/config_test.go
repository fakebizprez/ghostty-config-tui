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

func TestForVersionNewFileDefault(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if got := ResolveConfigPathForVersion(true, 1, 4, 1); !strings.HasSuffix(got, "config.ghostty") {
		t.Fatalf("new Ghostty should prefer config.ghostty, got %q", got)
	}
	if got := ResolveConfigPathForVersion(false, 0, 0, 0); !strings.HasSuffix(got, "config.ghostty") {
		t.Fatalf("unknown version should prefer config.ghostty, got %q", got)
	}
	if got := ResolveConfigPathForVersion(true, 1, 1, 9); strings.HasSuffix(got, ".ghostty") {
		t.Fatalf("old Ghostty should prefer plain config, got %q", got)
	}
}

func TestVersionGatingExistingWins(t *testing.T) {
	exists := func(p string) bool { return strings.HasSuffix(p, string(filepath.Separator)+"config") }
	for _, prefer := range []bool{true, false} {
		got := resolveWithEnv("linux", "/xdg", "/home/u", exists, prefer)
		if got != "/xdg/ghostty/config" {
			t.Fatalf("prefer=%v existing plain config should win, got %q", prefer, got)
		}
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

func TestSetAllReplacesInPlaceAndPreservesOthers(t *testing.T) {
	src := "# head\nfont-family = A\nfont-size=12   # odd spacing kept\nfont-family = B\n\nkeybind = x=y\n"
	f := Parse(src)
	f.SetAll("font-family", []string{"C", "D", "E"})
	want := "# head\nfont-family = C\nfont-family = D\nfont-family = E\nfont-size=12   # odd spacing kept\n\nkeybind = x=y\n"
	if got := f.Marshal(); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	if got := f.GetAll("font-family"); strings.Join(got, ",") != "C,D,E" {
		t.Fatalf("GetAll = %v", got)
	}
}

func TestSetAllRemoveAndAppend(t *testing.T) {
	f := Parse("a = 1\nb = 2\na = 3")
	f.SetAll("a", nil)
	f.SetAll("c", []string{"9"})
	if got := f.Marshal(); got != "b = 2\nc = 9\n" {
		t.Fatalf("got %q", got)
	}
	empty := Parse("")
	empty.SetAll("theme", []string{"Nord"})
	if got := empty.Marshal(); got != "theme = Nord\n" {
		t.Fatalf("new file = %q", got)
	}
	if keys := Parse("x = 1\ny = 2\nx = 3\n# x = 4\n").Keys(); strings.Join(keys, ",") != "x,y" {
		t.Fatalf("Keys = %v", keys)
	}
}
