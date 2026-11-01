// Package fontrender finds installed font files by family name and draws
// sample text with them, for the font playground's real rendering.
package fontrender

import (
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
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
				if name, err := f.Name(&buf, sfnt.NameIDFamily); err == nil && strings.EqualFold(name, family) {
					return p, true
				}
			}
		}
	}
	return "", false
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

// Render draws lines with the font at path (first face of a collection).
// Glyphs the font lacks render as its .notdef box, which is the point: it
// shows what this font covers on its own.
func Render(path string, lines []string, px float64, bg, fg color.Color) (image.Image, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f *opentype.Font
	if c, cerr := opentype.ParseCollection(b); cerr == nil {
		f, err = c.Font(0)
	} else {
		f, err = opentype.Parse(b)
	}
	if err != nil {
		return nil, err
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: px, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, err
	}
	defer face.Close()
	m := face.Metrics()
	lineH := (m.Height).Ceil() + 4
	w := 0
	for _, l := range lines {
		w = max(w, font.MeasureString(face, l).Ceil())
	}
	img := image.NewRGBA(image.Rect(0, 0, w+24, lineH*len(lines)+16))
	draw.Draw(img, img.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	d := &font.Drawer{Dst: img, Src: image.NewUniform(fg), Face: face}
	for i, l := range lines {
		d.Dot = fixed.P(12, 8+i*lineH+m.Ascent.Ceil())
		d.DrawString(l)
	}
	return img, nil
}
