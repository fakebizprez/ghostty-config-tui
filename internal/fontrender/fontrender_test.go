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

// Avenir Next.ttc stores Bold first; its plain Regular face is index 7.
func TestFindPicksRegularFaceInCollection(t *testing.T) {
	f, ok := Find("Avenir Next")
	if !ok {
		t.Skip("Avenir Next not installed")
	}
	if f.Index != 7 {
		t.Fatalf("Find picked face %d of %s, want 7 (Regular)", f.Index, f.Path)
	}
	face, err := loadFace(f)
	if err != nil {
		t.Fatal(err)
	}
	if d := face.Describe(); d.Aspect.Weight != 400 {
		t.Fatalf("loaded face weight %v, want 400 (Regular)", d.Aspect.Weight)
	}
}
