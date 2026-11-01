package graphics

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
)

func TestSupportedDetection(t *testing.T) {
	for _, c := range []struct {
		env  map[string]string
		want bool
	}{
		{map[string]string{"TERM_PROGRAM": "ghostty"}, true},
		{map[string]string{"TERM": "xterm-ghostty"}, true},
		{map[string]string{"TERM": "xterm-kitty"}, true},
		{map[string]string{"TERM_PROGRAM": "ghostty", "TMUX": "/tmp/tmux"}, false},
		{map[string]string{"TERM": "xterm-256color"}, false},
	} {
		got := supported(func(k string) string { return c.env[k] })
		if got != c.want {
			t.Errorf("supported(%v) = %v", c.env, got)
		}
	}
}

func TestTransmitIsChunkedKittyVirtualPlacement(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	x := uint32(1)
	for i := range img.Pix {
		x = x*1664525 + 1013904223 // LCG noise: incompressible, so several chunks
		img.Pix[i] = byte(x >> 24)
	}
	seq := Transmit(7, img, 8, 4)
	if !strings.HasPrefix(seq, "\x1b_G") || !strings.Contains(seq, "i=7") || !strings.Contains(seq, "U=1") ||
		!strings.Contains(seq, "c=8") || !strings.Contains(seq, "r=4") || strings.Count(seq, "\x1b_G") < 2 {
		t.Fatalf("unexpected transmit sequence prefix %q (%d chunks)", seq[:min(80, len(seq))], strings.Count(seq, "\x1b_G"))
	}
}

func TestPlaceholderGrid(t *testing.T) {
	rows := Placeholder(101, 3, 2)
	if len(rows) != 2 {
		t.Fatalf("rows = %d", len(rows))
	}
	plain := ansi.Strip(rows[1])
	r := []rune(plain)
	if len(r) != 9 || r[0] != kitty.Placeholder || r[1] != kitty.Diacritic(1) || r[2] != kitty.Diacritic(0) || r[5] != kitty.Diacritic(1) {
		t.Fatalf("row 1 runes = %U", r)
	}
	if !strings.Contains(rows[0], "38;5;101") {
		t.Fatalf("placeholder must carry the image id as 256-color fg: %q", rows[0])
	}
	_ = color.RGBA{}
}
