// Package icons renders Ghostty's macOS app icons. The images come from
// ghostty-org/ghostty macos/Assets.xcassets (MIT License, Copyright (c) 2024
// Mitchell Hashimoto, Ghostty contributors), downscaled to 160px, and the
// custom-style layering follows zerebos/ghostty-config (Apache-2.0).
package icons

import (
	"bytes"
	"embed"
	"image"
	"image/draw"
	"image/png"

	"ghostty-config-tui/internal/schema"
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
		c, ok := schema.ParseHex(hex)
		if !ok {
			c, _ = schema.ParseHex(def)
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
