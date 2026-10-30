package ghostty

import (
	"errors"
	"testing"
)

func TestParseThemes(t *testing.T) {
	out := "Dracula (dark)\nGruvbox Dark (dark)\nGruvbox Dark (dark)\nMy (Paren) Theme (custom)\n"
	got := ParseThemes(out)
	if len(got) != 3 || got[0] != "Dracula" || got[2] != "My (Paren) Theme" {
		t.Fatalf("ParseThemes = %v", got)
	}
}

func TestParseFonts(t *testing.T) {
	out := "JetBrains Mono\n  Regular\n  Bold\nFira Code\n  Regular\n"
	got := ParseFontFamilies(out)
	if len(got) != 2 || got[0] != "JetBrains Mono" || got[1] != "Fira Code" {
		t.Fatalf("ParseFontFamilies = %v", got)
	}
}

func TestParseVersion(t *testing.T) {
	real := "Ghostty 1.4.1-main+9781c0afc\n\nVersion\n  - version: 1.4.1-main+9781c0afc\n  - channel: tip\n"
	for _, tc := range []struct {
		in            string
		maj, min, pat int
		ok            bool
	}{
		{real, 1, 4, 1, true},
		{"Ghostty 1.2.3", 1, 2, 3, true},
		{"Ghostty 1.1.9", 1, 1, 9, true},
		{"v0.9.0", 0, 9, 0, true},
		{"1.2", 1, 2, 0, true},
		{"", 0, 0, 0, false},
		{"no version here", 0, 0, 0, false},
		{"Ghostty tip", 0, 0, 0, false},
	} {
		maj, min, pat, ok := ParseVersion(tc.in)
		if maj != tc.maj || min != tc.min || pat != tc.pat || ok != tc.ok {
			t.Errorf("ParseVersion(%q) = %d.%d.%d,%v, want %d.%d.%d,%v",
				tc.in, maj, min, pat, ok, tc.maj, tc.min, tc.pat, tc.ok)
		}
	}
}

func TestAtLeast123(t *testing.T) {
	for _, tc := range []struct {
		maj, min, pat int
		want          bool
	}{
		{1, 4, 1, true},
		{1, 2, 3, true},
		{1, 2, 2, false},
		{1, 1, 9, false},
		{0, 9, 0, false},
		{2, 0, 0, true},
	} {
		if got := AtLeast123(tc.maj, tc.min, tc.pat); got != tc.want {
			t.Errorf("AtLeast123(%d,%d,%d) = %v, want %v", tc.maj, tc.min, tc.pat, got, tc.want)
		}
	}
}

func TestVersionMissingBinary(t *testing.T) {
	oldLook := lookPath
	defer func() { lookPath = oldLook }()
	lookPath = func(string) (string, error) { return "", errors.New("not found") }
	if _, _, _, ok := Version(); ok {
		t.Fatal("missing binary should report unknown version")
	}
}

func TestMissingBinaryDegraded(t *testing.T) {
	oldLook := lookPath
	oldRun := runCommand
	defer func() { lookPath = oldLook; runCommand = oldRun }()
	lookPath = func(string) (string, error) { return "", errors.New("not found") }
	themes, st := ListThemes()
	if len(themes) != 0 || !st.Degraded {
		t.Fatalf("missing binary should degrade, got %v %+v", themes, st)
	}
	fonts, st := ListFonts()
	if len(fonts) != 0 || !st.Degraded {
		t.Fatalf("missing binary should degrade, got %v %+v", fonts, st)
	}
}
