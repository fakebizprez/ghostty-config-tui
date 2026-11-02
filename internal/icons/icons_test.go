package icons

import (
	"image"
	"testing"
)

func TestNamedIconsLoad(t *testing.T) {
	for _, n := range []string{"official", "blueprint", "chalkboard", "microchip", "glass", "holographic", "paper", "retro", "xray"} {
		img, ok := Icon(n, "", "", "")
		if !ok || img.Bounds().Dx() != 160 {
			t.Fatalf("icon %s ok=%v", n, ok)
		}
	}
	if _, ok := Icon("custom", "", "", ""); ok {
		t.Fatal("custom (a user file) has no bundled image")
	}
}

func TestCustomStyleTintsGhostAndScreen(t *testing.T) {
	img, ok := Icon("custom-style", "beige", "#ff0000", "#0000ff")
	if !ok {
		t.Fatal("custom-style should compose")
	}
	// The ghost mask's most opaque pixel must come out red-dominant and a
	// screen-only pixel blue-dominant.
	gx, gy := maxAlpha(ghostMask())
	r, g, b, _ := img.At(gx, gy).RGBA()
	if !(r > g && r > b) {
		t.Fatalf("ghost pixel (%d,%d) = %v,%v,%v, want red", gx, gy, r>>8, g>>8, b>>8)
	}
	sx, sy := screenOnlyPixel()
	r, g, b, _ = img.At(sx, sy).RGBA()
	if !(b > r && b > g) {
		t.Fatalf("screen pixel (%d,%d) = %v,%v,%v, want blue", sx, sy, r>>8, g>>8, b>>8)
	}
}

// maxAlpha finds the most opaque pixel of a mask.
func maxAlpha(m image.Image) (int, int) {
	bx, by, best := 0, 0, uint32(0)
	b := m.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := m.At(x, y).RGBA(); a > best {
				bx, by, best = x, y, a
			}
		}
	}
	return bx, by
}

// screenOnlyPixel finds an opaque screen-mask pixel outside the ghost.
func screenOnlyPixel() (int, int) {
	s, g := screenMask(), ghostMask()
	b := s.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, sa := s.At(x, y).RGBA()
			_, _, _, ga := g.At(x, y).RGBA()
			if sa > 0xf000 && ga == 0 {
				return x, y
			}
		}
	}
	return 0, 0
}
