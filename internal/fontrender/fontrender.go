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

	foundMu sync.Mutex
	found   = map[string]string{} // family -> file, from earlier Find calls
)

// Find returns a font file whose family (or typographic family) matches.
// The first miss indexes the font directories.
func Find(family string) (string, bool) {
	foundMu.Lock()
	defer foundMu.Unlock()
	if p, ok := found[family]; ok {
		return p, true
	}
	// Fast path: most font files are named after their family.
	p, ok := byFileName(family)
	if !ok {
		indexOnce.Do(buildIndex)
		p, ok = index[strings.ToLower(strings.TrimSpace(family))]
	}
	if ok {
		found[family] = p
	}
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
			regular := false
			eachFont(p, func(f *sfnt.Font) {
				name, err := f.Name(&buf, sfnt.NameIDFamily)
				if err != nil || !strings.EqualFold(name, family) {
					return
				}
				if sub, _ := f.Name(&buf, sfnt.NameIDSubfamily); strings.EqualFold(sub, "Regular") {
					regular = true
				}
				if found == "" {
					found = p
				}
			})
			if regular {
				return p, true // the upright regular face wins
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
			eachFont(p, func(f *sfnt.Font) {
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
			})
			return nil
		})
	}
}

// eachFont calls fn for every face in the font file at p. It reads through
// the open file, so only the tables fn asks for are loaded, not the whole file.
func eachFont(p string, fn func(*sfnt.Font)) {
	file, err := os.Open(p)
	if err != nil {
		return
	}
	defer file.Close()
	if c, err := sfnt.ParseCollectionReaderAt(file); err == nil {
		for i := 0; i < c.NumFonts(); i++ {
			if f, err := c.Font(i); err == nil {
				fn(f)
			}
		}
		return
	}
	if f, err := sfnt.ParseReaderAt(file); err == nil {
		fn(f)
	}
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

// glyph is one shaped glyph: font units from parseHB, pixels from glyphRun.
type glyph struct {
	gid             gofont.GID
	xoff, yoff, adv float32
}

var hbPath, _ = exec.LookPath("hb-shape")

func hasHBShape() bool { return hbPath != "" }

// hbShape shapes with the real HarfBuzz CLI when installed; go-text's
// shaper misses some variable-font substitutions (e.g. Cascadia Code).
func hbShape(path, text string) ([]glyph, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, hbPath, "--no-glyph-names", path, "--", text).Output()
	if err != nil {
		return nil, err
	}
	return parseHB(string(out))
}

// parseHB reads `[gid=cluster@x,y+adv|...]`.
func parseHB(out string) ([]glyph, error) {
	out = strings.Trim(strings.TrimSpace(out), "[]")
	if out == "" {
		return nil, nil
	}
	var gs []glyph
	for _, item := range strings.Split(out, "|") {
		var g glyph
		gid, rest, ok := strings.Cut(item, "=")
		n, err := strconv.Atoi(gid)
		if !ok || err != nil {
			return nil, fmt.Errorf("bad hb-shape glyph %q", item)
		}
		g.gid = gofont.GID(n)
		pos, adv, _ := strings.Cut(rest, "+")
		g.adv = atof(adv)
		if _, off, ok := strings.Cut(pos, "@"); ok {
			x, y, _ := strings.Cut(off, ",")
			g.xoff, g.yoff = atof(x), atof(y)
		}
		gs = append(gs, g)
	}
	return gs, nil
}

func atof(s string) float32 { n, _ := strconv.Atoi(s); return float32(n) }

// glyphRun shapes a line into glyphs positioned in pixels.
func glyphRun(face *gofont.Face, path, text string, px float64) []glyph {
	if hasHBShape() {
		if gs, err := hbShape(path, text); err == nil {
			unit := float32(px) / float32(face.Upem())
			for i := range gs {
				gs[i].xoff, gs[i].yoff, gs[i].adv = gs[i].xoff*unit, gs[i].yoff*unit, gs[i].adv*unit
			}
			return gs
		}
	}
	var gs []glyph // go-text positions are 26.6 fixed-point pixels
	for _, g := range shape(face, text, px).Glyphs {
		gs = append(gs, glyph{gid: g.GlyphID, xoff: float32(g.XOffset) / 64, yoff: float32(g.YOffset) / 64, adv: float32(g.XAdvance) / 64})
	}
	return gs
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
	runs := make([][]glyph, len(lines))
	w := 0
	for i, l := range lines {
		runs[i] = glyphRun(face, path, l, px)
		adv := float32(0)
		for _, g := range runs[i] {
			adv += g.adv
		}
		w = max(w, int(adv)+1)
	}
	img := image.NewRGBA(image.Rect(0, 0, w+24, lineH*len(lines)+16))
	draw.Draw(img, img.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	ras := vector.NewRasterizer(img.Bounds().Dx(), img.Bounds().Dy())
	for i, gs := range runs {
		x := float32(12)
		base := float32(8 + i*lineH + int(px))
		for _, g := range gs {
			ox := x + g.xoff
			oy := base - g.yoff
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
			x += g.adv
		}
	}
	ras.Draw(img, img.Bounds(), image.NewUniform(fg), image.Point{})
	return img, nil
}
