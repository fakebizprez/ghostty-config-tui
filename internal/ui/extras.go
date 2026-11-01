package ui

import (
	"image"
	"image/color"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"ghostty-config-tui/internal/fontrender"
	"ghostty-config-tui/internal/ghostty"
	"ghostty-config-tui/internal/graphics"
	"ghostty-config-tui/internal/icons"
	"ghostty-config-tui/internal/schema"
)

// Kitty image ids (16..255 so they travel as 256-color foregrounds).
const (
	iconID   = 101
	fontID   = 102
	iconCols = 12
	iconRows = 6
)

// FontImageMsg carries the playground sample rendered in the real font.
type FontImageMsg struct {
	Img  image.Image
	File string
	Err  string
}

// renderFont draws sample lines with the installed font for family.
var renderFont = func(family string, lines []string, bg, fg string) (image.Image, string, error) {
	path, ok := fontrender.Find(family)
	if !ok {
		return nil, "", errNotInstalled(family)
	}
	img, err := fontrender.Render(path, lines, 30, hexColor(bg), hexColor(fg))
	return img, path, err
}

type errNotInstalled string

func (e errNotInstalled) Error() string {
	return "no font file for \"" + string(e) + "\" in the system font folders (Ghostty's bundled default has no file)"
}

func hexColor(h string) color.RGBA {
	c := color.RGBA{A: 255}
	if schema.IsHex(h) {
		var r, g, b uint8
		for i, p := range []*uint8{&r, &g, &b} {
			*p = uint8(hexVal(h[1+2*i])<<4 | hexVal(h[2+2*i]))
		}
		c.R, c.G, c.B = r, g, b
	}
	return c
}

func hexVal(b byte) byte {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10
	}
	return 0
}

func (m Model) renderFontCmd() tea.Cmd {
	family := firstOf(m.vals["font-family"])
	if family == "" {
		family = "JetBrains Mono"
	}
	pc := m.Colors()
	return func() tea.Msg {
		img, file, err := renderFont(family, playgroundSamples, pc.BG, pc.FG)
		if err != nil {
			return FontImageMsg{Err: err.Error()}
		}
		return FontImageMsg{Img: img, File: file}
	}
}

// fontImage sizes the rendered sample to the dialog and uploads it.
func (m Model) fontImage(msg FontImageMsg) (Model, tea.Cmd) {
	if msg.Img == nil {
		m.fontErr = msg.Err
		return m, nil
	}
	b := msg.Img.Bounds()
	cols := min(max(m.width-8, 20), 100)
	// Terminal cells are about twice as tall as wide.
	rows := max(1, (cols*b.Dy()+b.Dx())/(2*b.Dx()))
	m.fontImg, m.fontCols, m.fontRows, m.fontFile = true, cols, rows, msg.File
	return m, tea.Raw(graphics.Transmit(fontID, msg.Img, cols, rows))
}

// syncIcon uploads the app icon when an icon setting is shown and its
// look changed since the last upload.
func (m *Model) syncIcon() tea.Cmd {
	st, ok := m.Current()
	if m.ed != nil {
		st, ok = m.ed.st, true
	}
	if !m.opts.Graphics || !ok || !strings.HasPrefix(st.Key, "macos-icon") {
		return nil
	}
	style := m.effective("macos-icon")
	sig := strings.Join([]string{style, m.effective("macos-icon-frame"), m.explicit("macos-icon-ghost-color"), m.explicit("macos-icon-screen-color")}, "|")
	if sig == m.iconSent {
		return nil
	}
	img, ok := icons.Icon(style, m.effective("macos-icon-frame"), hexOr(m.explicit("macos-icon-ghost-color"), ""), hexOr(m.explicit("macos-icon-screen-color"), ""))
	if !ok {
		return nil
	}
	m.iconSent = sig
	return tea.Raw(graphics.Transmit(iconID, img, iconCols, iconRows))
}

// playgroundSamples exercise ligatures, box drawing, Nerd Font and
// Powerline glyphs, CJK, and emoji, the cases font fallback gets wrong.
var playgroundSamples = []string{
	"The quick brown fox 0123456789",
	"-> => != <= >= === :: ||",
	"┌─┬─┐ │ ├─┼─┤ └─┴─┘ ░▒▓█",
	"    ",
	"日本語 한국어 中文",
	"🙂 👻 ✨",
}

var showFace = ghostty.ShowFace

// FacesMsg delivers `ghostty +show-face` results for the playground.
type FacesMsg struct {
	Lines []string
	OK    bool
}

func (m Model) facesCmd() tea.Cmd {
	families := append([]string(nil), m.vals["font-family"]...)
	return func() tea.Msg {
		lines, ok := showFace(strings.Join(playgroundSamples, ""), families)
		return FacesMsg{Lines: lines, OK: ok}
	}
}

func (m Model) playgroundLines(w int) []string {
	fam := strings.Join(m.vals["font-family"], ", ")
	if fam == "" {
		fam = "Ghostty default"
	}
	out := []string{"font-family: " + hi.Render(fam) + faint.Render("  (staged values, unsaved edits included)"), ""}
	switch {
	case m.fontImg:
		out = append(out, faint.Render("Rendered with "+filepath.Base(m.fontFile)+" (shaped, so ligatures apply; glyphs this font lacks show as boxes):"))
		out = append(out, graphics.Placeholder(fontID, m.fontCols, m.fontRows)...)
	default:
		if m.fontErr != "" {
			out = append(out, warnSty.Render(m.fontErr))
		} else if m.opts.Graphics {
			out = append(out, faint.Render("rendering samples in your font…"))
		} else {
			out = append(out, faint.Render("This terminal can't show images, so samples use its own font (run inside Ghostty for real rendering):"))
		}
		for _, s := range playgroundSamples {
			out = append(out, "  "+s)
		}
	}
	out = append(out, "", bold.Render("Which face Ghostty picks for each character:"))
	switch {
	case m.faces == nil && !m.facesDone:
		out = append(out, faint.Render("  asking ghostty +show-face…"))
	case !m.facesOK:
		out = append(out, warnSty.Render("  ghostty not available — install it to see font fallback"))
	default:
		for _, l := range groupFaces(m.faces) {
			st := lipgloss.NewStyle()
			if strings.Contains(l, "not found") || strings.Contains(l, "Last Resort") {
				st = warnSty
			}
			out = append(out, "  "+st.Render(l))
		}
	}
	out = append(out, "", faint.Render("esc close · edit font-family, then f again to re-check"))
	return out
}

// IconPalette is the macOS app icon approximation.
type IconPalette struct{ Frame, Screen, Ghost string }

var frameColors = map[string]string{
	"aluminum": "#a8a9ad", "beige": "#d9cfb4", "plastic": "#3a3a3c", "chrome": "#e8e8ea",
}

// IconColors resolves the icon settings, falling back to the official look.
func (m Model) IconColors() IconPalette {
	ic := IconPalette{Frame: frameColors["aluminum"], Screen: "#1d1f2b", Ghost: "#ffffff"}
	custom := m.effective("macos-icon") == "custom-style"
	if f, ok := frameColors[m.effective("macos-icon-frame")]; ok && custom {
		ic.Frame = f
	}
	if custom {
		ic.Ghost = hexOr(m.explicit("macos-icon-ghost-color"), ic.Ghost)
		ic.Screen = hexOr(m.explicit("macos-icon-screen-color"), ic.Screen)
	}
	if m.effective("macos-icon") == "blueprint" {
		ic.Screen, ic.Ghost = "#1d4e89", "#d6e6ff"
	}
	return ic
}

// iconLines shows the real icon image when the terminal supports graphics,
// otherwise a block-art approximation.
func (m Model) iconLines() []string {
	if m.opts.Graphics && m.iconSent != "" {
		out := []string{bold.Render("App icon") + faint.Render("  "+m.effective("macos-icon")+" · "+m.effective("macos-icon-frame"))}
		return append(out, graphics.Placeholder(iconID, iconCols, iconRows)...)
	}
	return m.blockIcon()
}

func (m Model) blockIcon() []string {
	ic := m.IconColors()
	frame := lipgloss.NewStyle().Foreground(lipgloss.Color(ic.Frame)).Background(lipgloss.Color(ic.Screen))
	ghost := lipgloss.NewStyle().Foreground(lipgloss.Color(ic.Ghost)).Background(lipgloss.Color(ic.Screen))
	edge := lipgloss.NewStyle().Foreground(lipgloss.Color(ic.Frame))
	rows := []string{"   ▄▀▀▀▀▄   ", "  █ ▀  ▀ █  ", "  █      █  ", "  █▄▀▄▀▄▀█  "}
	out := []string{bold.Render("App icon") + faint.Render("  "+m.effective("macos-icon")+" · "+m.effective("macos-icon-frame")),
		edge.Render("▗▄▄▄▄▄▄▄▄▄▄▄▄▄▄▖")}
	for _, r := range rows {
		out = append(out, edge.Render("▐")+frame.Render(" ")+ghost.Render(r)+frame.Render(" ")+edge.Render("▌"))
	}
	out = append(out, edge.Render("▝▀▀▀▀▀▀▀▀▀▀▀▀▀▀▘"))
	if m.effective("macos-icon") != "custom-style" {
		out = append(out, faint.Render("frame and colors apply with macos-icon = custom-style"))
	}
	return out
}

// groupFaces turns `U+XXXX « c » outcome` lines into one line per outcome
// listing its characters, in first-seen order.
func groupFaces(lines []string) []string {
	var order []string
	chars := map[string][]string{}
	for _, l := range lines {
		_, rest, ok := strings.Cut(l, "« ")
		c, outcome, ok2 := strings.Cut(rest, " » ")
		if !ok || !ok2 {
			continue
		}
		if _, seen := chars[outcome]; !seen {
			order = append(order, outcome)
		}
		chars[outcome] = append(chars[outcome], c)
	}
	out := make([]string, 0, len(order))
	for _, o := range order {
		out = append(out, strings.TrimSuffix(o, ".")+": "+strings.Join(chars[o], " "))
	}
	return out
}
