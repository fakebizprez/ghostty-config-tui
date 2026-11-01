package icons

import (
	"image/color"
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
	_ = color.RGBA{}
}

func TestParseHex(t *testing.T) {
	if c, ok := parseHex("#10ff20"); !ok || c != (color.RGBA{0x10, 0xff, 0x20, 0xff}) {
		t.Fatalf("parseHex = %v %v", c, ok)
	}
	if _, ok := parseHex("teal"); ok {
		t.Fatal("names are not hex")
	}
}
