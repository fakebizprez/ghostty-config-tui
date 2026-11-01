package fontrender

import (
	"image/color"
	"runtime"
	"testing"
)

func TestFindAndRenderSystemFont(t *testing.T) {
	family := "DejaVu Sans Mono"
	if runtime.GOOS == "darwin" {
		family = "Menlo"
	}
	path, ok := Find(family)
	if !ok {
		t.Skipf("%s not installed", family)
	}
	img, err := Render(path, []string{"The quick brown fox", "-> => !="}, 28, color.RGBA{0, 0, 0, 255}, color.RGBA{255, 255, 255, 255})
	if err != nil {
		t.Fatal(err)
	}
	lit := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if r, _, _, _ := img.At(x, y).RGBA(); r > 0x8000 {
				lit++
			}
		}
	}
	if lit < 200 {
		t.Fatalf("only %d lit pixels; text did not render", lit)
	}
}

func TestFindUnknownFamily(t *testing.T) {
	if _, ok := Find("No Such Font Family 123"); ok {
		t.Fatal("unknown family should not be found")
	}
}
