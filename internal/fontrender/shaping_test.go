package fontrender

import (
	"strings"
	"testing"

	gofont "github.com/go-text/typesetting/font"
)

// glyphIDs returns shaped glyph ids next to plain cmap ids.
func glyphIDs(path, text string) (shaped, plain []gofont.GID, err error) {
	face, err := loadFace(path)
	if err != nil {
		return nil, nil, err
	}
	for _, g := range glyphRun(face, path, text, 30) {
		shaped = append(shaped, g.gid)
	}
	for _, r := range text {
		gid, _ := face.NominalGlyph(r)
		plain = append(plain, gid)
	}
	return shaped, plain, nil
}

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

func TestHarfBuzzShapesVariableFontLigatures(t *testing.T) {
	path, ok := Find("Cascadia Code")
	if !ok || !hasHBShape() {
		t.Skip("needs Cascadia Code and hb-shape")
	}
	shaped, plain, err := glyphIDs(path, "->")
	if err != nil {
		t.Fatal(err)
	}
	if len(shaped) == len(plain) && shaped[0] == plain[0] && shaped[1] == plain[1] {
		t.Fatalf("Cascadia -> not substituted: %v", shaped)
	}
}

func TestParseHBShape(t *testing.T) {
	got, err := parseHB("[1970=0+1200|2728=1@-10,5+1200]")
	if err != nil || len(got) != 2 || got[0].gid != 1970 || got[1].xoff != -10 || got[1].yoff != 5 || got[1].adv != 1200 {
		t.Fatalf("parseHB = %+v err=%v", got, err)
	}
}
