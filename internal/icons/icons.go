// Package icons renders Ghostty's macOS app icons. The images come from
// ghostty-org/ghostty macos/Assets.xcassets (MIT License, Copyright (c) 2024
// Mitchell Hashimoto, Ghostty contributors), downscaled to 160px, and the
// custom-style layering follows zerebos/ghostty-config (Apache-2.0).
package icons

import (
	"bytes"
	"embed"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strconv"
	"strings"
)

//go:embed assets/*.png
var assets embed.FS

func load(name string) (image.Image, bool) {
	b, err := assets.ReadFile("assets/" + name + ".png")
	if err != nil {
		return nil, false
	}
	img, err := png.Decode(bytes.NewReader(b))
	return img, err == nil
}

// Defaults match Ghostty's custom-style look when colors are unset.
const (
	DefaultGhost  = "#f6f7fb"
	DefaultScreen = "#4f5a6f"
)

// Icon returns the icon for a macos-icon value. custom-style composes the
// frame with tinted screen and ghost layers; "custom" (a user file) has no
// bundled image.
func Icon(style, frame, ghost, screen string) (image.Image, bool) {
	if style != "custom-style" {
		if style == "custom" {
			return nil, false
		}
		return load(style)
	}
	base, ok := load(frame)
	if !ok {
		base, _ = load("aluminum")
	}
	dst := image.NewRGBA(base.Bounds())
	draw.Draw(dst, dst.Bounds(), base, image.Point{}, draw.Src)
	tint := func(hex, def string, mask image.Image) {
		c, ok := parseHex(hex)
		if !ok {
			c, _ = parseHex(def)
		}
		draw.DrawMask(dst, dst.Bounds(), image.NewUniform(c), image.Point{}, mask, image.Point{}, draw.Over)
	}
	tint(screen, DefaultScreen, screenMask())
	tint(ghost, DefaultGhost, ghostMask())
	for _, layer := range []string{"crt", "gloss"} {
		if img, ok := load(layer); ok {
			draw.Draw(dst, dst.Bounds(), img, image.Point{}, draw.Over)
		}
	}
	return dst, true
}

func screenMask() image.Image { m, _ := load("mask"); return m }
func ghostMask() image.Image  { m, _ := load("ghost"); return m }

// maxAlpha finds the most opaque pixel of a mask (for tests).
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

func parseHex(s string) (color.RGBA, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return color.RGBA{}, false
	}
	n, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return color.RGBA{}, false
	}
	return color.RGBA{uint8(n >> 16), uint8(n >> 8), uint8(n), 0xff}, true
}
