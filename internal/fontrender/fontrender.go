// Package fontrender finds installed font files by family name and draws
// sample text with them, for the font playground's real rendering.
package fontrender

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-text/typesetting/di"
	gofont "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

func fontDirs() []string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" {
		return []string{"/System/Library/Fonts", "/Library/Fonts", filepath.Join(home, "Library/Fonts")}
	}
	return []string{"/usr/share/fonts", "/usr/local/share/fonts", filepath.Join(home, ".local/share/fonts"), filepath.Join(home, ".fonts")}
}

var (
	indexOnce sync.Once
	index     map[string]string // lowercase family -> file
)

// Find returns a font file whose family (or typographic family) matches.
// The first call indexes the font directories.
func Find(family string) (string, bool) {
	// Fast path: most font files are named after their family.
	if p, ok := byFileName(family); ok {
		return p, true
	}
	indexOnce.Do(buildIndex)
	p, ok := index[strings.ToLower(strings.TrimSpace(family))]
	return p, ok
}

func squash(s string) string {
	return strings.ToLower(strings.NewReplacer(" ", "", "-", "", "_", "").Replace(s))
}

func byFileName(family string) (string, bool) {
	want := squash(family)
	var buf sfnt.Buffer
	found := ""
	for _, dir := range fontDirs() {
		matches, _ := filepath.Glob(filepath.Join(dir, "*"))
		for _, p := range matches {
			if !strings.HasPrefix(squash(filepath.Base(p)), want) {
				continue
			}
			b, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			for _, f := range parseAll(b) {
				name, err := f.Name(&buf, sfnt.NameIDFamily)
				if err != nil || !strings.EqualFold(name, family) {
					continue
				}
				if sub, _ := f.Name(&buf, sfnt.NameIDSubfamily); strings.EqualFold(sub, "Regular") {
					return p, true // the upright regular face wins
				}
				if found == "" {
					found = p
				}
			}
		}
	}
	return found, found != ""
}

func buildIndex() {
	index = map[string]string{}
	var buf sfnt.Buffer
	for _, dir := range fontDirs() {
		_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(p))
			if ext != ".ttf" && ext != ".otf" && ext != ".ttc" && ext != ".otc" {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			for _, f := range parseAll(b) {
				for _, id := range []sfnt.NameID{sfnt.NameIDFamily, sfnt.NameIDTypographicFamily} {
					if name, err := f.Name(&buf, id); err == nil && name != "" {
						key := strings.ToLower(name)
						// Prefer a regular face: keep the first file, but let a
						// file named *Regular* win.
						if _, seen := index[key]; !seen || strings.Contains(p, "Regular") {
							index[key] = p
						}
					}
				}
			}
			return nil
		})
	}
}

func parseAll(b []byte) []*sfnt.Font {
	if c, err := sfnt.ParseCollection(b); err == nil {
		var out []*sfnt.Font
		for i := 0; i < c.NumFonts(); i++ {
			if f, err := c.Font(i); err == nil {
				out = append(out, f)
			}
		}
		return out
	}
	if f, err := sfnt.Parse(b); err == nil {
		return []*sfnt.Font{f}
	}
	return nil
}

// loadFace parses the first face of a font file or collection.
func loadFace(path string) (*gofont.Face, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if faces, err := gofont.ParseTTC(bytes.NewReader(b)); err == nil && len(faces) > 0 {
		return faces[0], nil
	}
	return gofont.ParseTTF(bytes.NewReader(b))
}

func shape(face *gofont.Face, text string, px float64) shaping.Output {
	r := []rune(text)
	return (&shaping.HarfbuzzShaper{}).Shape(shaping.Input{
		Text: r, RunStart: 0, RunEnd: len(r),
		Direction: di.DirectionLTR, Face: face,
		Size:   fixed.Int26_6(px * 64),
		Script: language.Latin, Language: language.NewLanguage("en"),
	})
}

// hbGlyph is one glyph from HarfBuzz's hb-shape, in font units.
type hbGlyph struct {
	gid             gofont.GID
	xoff, yoff, adv int
}

var hbPath, _ = exec.LookPath("hb-shape")

func hasHBShape() bool { return hbPath != "" }

// hbShape shapes with the real HarfBuzz CLI when installed; go-text's
// shaper misses some variable-font substitutions (e.g. Cascadia Code).
func hbShape(path, text string) ([]hbGlyph, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, hbPath, "--no-glyph-names", path, "--", text).Output()
	if err != nil {
		return nil, err
	}
	return parseHB(string(out))
}

// parseHB reads `[gid=cluster@x,y+adv|...]`.
func parseHB(out string) ([]hbGlyph, error) {
	out = strings.Trim(strings.TrimSpace(out), "[]")
	if out == "" {
		return nil, nil
	}
	var gs []hbGlyph
	for _, item := range strings.Split(out, "|") {
		var g hbGlyph
		gid, rest, ok := strings.Cut(item, "=")
		n, err := strconv.Atoi(gid)
		if !ok || err != nil {
			return nil, fmt.Errorf("bad hb-shape glyph %q", item)
		}
		g.gid = gofont.GID(n)
		pos, adv, _ := strings.Cut(rest, "+")
		g.adv, _ = strconv.Atoi(adv)
		if _, off, ok := strings.Cut(pos, "@"); ok {
			x, y, _ := strings.Cut(off, ",")
			g.xoff, _ = strconv.Atoi(x)
			g.yoff, _ = strconv.Atoi(y)
		}
		gs = append(gs, g)
	}
	return gs, nil
}

// glyphRun shapes a line into glyphs positioned in pixels.
func glyphRun(face *gofont.Face, path, text string, px float64) (glyphs []hbGlyph, scale float32) {
	unit := float32(px) / float32(face.Upem())
	if hasHBShape() {
		if gs, err := hbShape(path, text); err == nil {
			return gs, unit
		}
	}
	// go-text positions are 26.6 fixed pixels; express them as 1/64 px units.
	for _, g := range shape(face, text, px).Glyphs {
		glyphs = append(glyphs, hbGlyph{gid: g.GlyphID, xoff: int(g.XOffset), yoff: int(g.YOffset), adv: int(g.XAdvance)})
	}
	return glyphs, 1.0 / 64
}

// glyphIDs returns shaped glyph ids next to plain cmap ids (for tests).
func glyphIDs(path, text string) (shaped, plain []gofont.GID, err error) {
	face, err := loadFace(path)
	if err != nil {
		return nil, nil, err
	}
	gs, _ := glyphRun(face, path, text, 30)
	for _, g := range gs {
		shaped = append(shaped, g.gid)
	}
	for _, r := range text {
		gid, _ := face.NominalGlyph(r)
		plain = append(plain, gid)
	}
	return shaped, plain, nil
}

// Render shapes each line with HarfBuzz-compatible shaping (so ligatures
// and contextual alternates apply) and rasterizes the glyph outlines.
// Glyphs the font lacks render as its .notdef box, which shows what this
// font covers on its own.
func Render(path string, lines []string, px float64, bg, fg color.Color) (image.Image, error) {
	face, err := loadFace(path)
	if err != nil {
		return nil, err
	}
	scale := float32(px) / float32(face.Upem())
	lineH := int(px*1.35) + 4
	type run struct {
		gs   []hbGlyph
		unit float32
	}
	runs := make([]run, len(lines))
	w := 0
	for i, l := range lines {
		gs, unit := glyphRun(face, path, l, px)
		runs[i] = run{gs, unit}
		adv := 0
		for _, g := range gs {
			adv += g.adv
		}
		w = max(w, int(float32(adv)*unit)+1)
	}
	img := image.NewRGBA(image.Rect(0, 0, w+24, lineH*len(lines)+16))
	draw.Draw(img, img.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	ras := vector.NewRasterizer(img.Bounds().Dx(), img.Bounds().Dy())
	for i, r := range runs {
		x := float32(12)
		base := float32(8 + i*lineH + int(px))
		for _, g := range r.gs {
			ox := x + float32(g.xoff)*r.unit
			oy := base - float32(g.yoff)*r.unit
			if out, ok := face.GlyphData(g.gid).(gofont.GlyphOutline); ok {
				pt := func(p ot.SegmentPoint) (float32, float32) { return ox + p.X*scale, oy - p.Y*scale }
				for _, seg := range out.Segments {
					a := seg.Args
					switch seg.Op {
					case ot.SegmentOpMoveTo:
						ras.MoveTo(pt(a[0]))
					case ot.SegmentOpLineTo:
						ras.LineTo(pt(a[0]))
					case ot.SegmentOpQuadTo:
						x1, y1 := pt(a[0])
						x2, y2 := pt(a[1])
						ras.QuadTo(x1, y1, x2, y2)
					case ot.SegmentOpCubeTo:
						x1, y1 := pt(a[0])
						x2, y2 := pt(a[1])
						x3, y3 := pt(a[2])
						ras.CubeTo(x1, y1, x2, y2, x3, y3)
					}
				}
				ras.ClosePath()
			}
			x += float32(g.adv) * r.unit
		}
	}
	ras.Draw(img, img.Bounds(), image.NewUniform(fg), image.Point{})
	return img, nil
}
