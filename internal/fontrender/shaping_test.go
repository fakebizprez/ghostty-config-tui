package fontrender

import (
	"strings"
	"testing"
)

// Fira Code's ligatures must come out of shaping the way HarfBuzz
// (hb-shape) produces them: substituted glyphs, not the plain cmap ones.
func TestShapingAppliesLigatures(t *testing.T) {
	var path string
	for _, fam := range []string{"FiraCode Nerd Font", "Fira Code", "FiraCode Nerd Font Mono"} {
		if p, ok := Find(fam); ok {
			path = p
			break
		}
	}
	if path == "" {
		t.Skip("Fira Code not installed")
	}
	for _, txt := range []string{"->", "!=", "www"} {
		shaped, plain, err := glyphIDs(path, txt)
		if err != nil {
			t.Fatal(err)
		}
		same := len(shaped) == len(plain)
		for i := 0; same && i < len(shaped); i++ {
			same = shaped[i] == plain[i]
		}
		if same {
			t.Errorf("%q: shaping kept plain glyphs %v", txt, shaped)
		}
	}
}

func TestFindPrefersRegularFace(t *testing.T) {
	p, ok := Find("FiraCode Nerd Font")
	if !ok {
		t.Skip("FiraCode Nerd Font not installed")
	}
	if !strings.Contains(p, "Regular") {
		t.Fatalf("Find picked %s, want the Regular face", p)
	}
}
