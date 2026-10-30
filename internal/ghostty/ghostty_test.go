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
