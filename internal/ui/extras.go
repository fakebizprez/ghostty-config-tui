package ui

import (
	"errors"
	"fmt"
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
	Gen  int // the playground opening it answers; older ones are dropped
}

// renderFont draws sample lines with the installed font for family.
var renderFont = func(family string, lines []string, bg, fg string) (image.Image, string, error) {
	f, ok := fontrender.Find(family)
	if !ok {
		return nil, "", fmt.Errorf("no font file for %q in the system font folders (Ghostty's bundled default has no file)", family)
	}
	img, err := fontrender.Render(f, lines, 30, opaque(bg), opaque(fg))
	return img, f.Path, err
}

// unquote strips the double quotes Ghostty removes when it reads a value
// from the file (it keeps them on the command line).
func unquote(v string) string {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v[1 : len(v)-1]
	}
	return v
}

// primaryFamily is the first font-family after the last reset (an empty value).
func primaryFamily(vals []string) string {
	family := ""
	for _, v := range vals {
		if u := unquote(v); u == "" {
			family = ""
		} else if family == "" {
			family = u
		}
	}
	return family
}

// opaque decodes a hex color, black when it is not hex.
func opaque(h string) color.RGBA {
	c, _ := schema.ParseHex(h)
	c.A = 0xff
	return c
}

// renderFontCmd is nil when the terminal can't show the image.
func (m Model) renderFontCmd() tea.Cmd {
	if !m.opts.Graphics {
		return nil
	}
	family := primaryFamily(m.vals["font-family"])
	if family == "" {
		family = "JetBrains Mono"
	}
	pc, gen := m.Colors(), m.fontGen
	return func() tea.Msg {
		img, file, err := renderFont(family, playgroundSamples, pc.BG, pc.FG)
		if err != nil {
			return FontImageMsg{Err: err.Error(), Gen: gen}
		}
		return FontImageMsg{Img: img, File: file, Gen: gen}
	}
}

// fontImage sizes the rendered sample to the dialog and uploads it.
func (m Model) fontImage(msg FontImageMsg) (Model, tea.Cmd) {
	if msg.Gen != m.fontGen {
		return m, nil
	}
	if msg.Img == nil {
		m.fontErr = msg.Err
		return m, nil
	}
	b := msg.Img.Bounds()
	cols := min(max(m.width-8, 20), 100)
	// Terminal cells are about twice as tall as wide.
	rows := max(1, (cols*b.Dy()+b.Dx())/(2*b.Dx()))
	// Leave the dialog room for the header above and the face list heading below.
	if maxRows := max(3, m.dialogHeight()-7); rows > maxRows {
		rows, cols = maxRows, max(1, maxRows*2*b.Dx()/b.Dy())
	}
	m.fontCols, m.fontRows, m.fontFile = cols, rows, msg.File
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
	style, frame := m.effective("macos-icon"), m.effective("macos-icon-frame")
	ghost, screen := m.explicit("macos-icon-ghost-color"), m.explicit("macos-icon-screen-color")
	sig := strings.Join([]string{style, frame, ghost, screen}, "|")
	if sig == m.iconSent {
		return nil
	}
	img, ok := icons.Icon(style, frame, hexOr(ghost, ""), hexOr(screen, ""))
	if !ok {
		m.iconSent = "" // no bundled image (custom): show block art, not the last upload
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
	Err   error
	Gen   int // the playground opening it answers; older ones are dropped
}

// fontSettings are the staged font-* values as unquoted key=value pairs.
func (m Model) fontSettings() []string {
	var out []string
	for _, st := range m.sch.Settings {
		if strings.HasPrefix(st.Key, "font-") {
			for _, v := range m.vals[st.Key] {
				out = append(out, st.Key+"="+unquote(v))
			}
		}
	}
	return out
}

func (m Model) facesCmd() tea.Cmd {
	settings, gen := m.fontSettings(), m.fontGen
	return func() tea.Msg {
		lines, err := showFace(strings.Join(playgroundSamples, ""), settings)
		return FacesMsg{Lines: lines, Err: err, Gen: gen}
	}
}

func (m Model) playgroundLines(w, h int) []string {
	fam := strings.Join(m.vals["font-family"], ", ")
	if fam == "" {
		fam = "Ghostty default"
	}
	out := []string{"font-family: " + hi.Render(fam) + faint.Render("  (staged values, unsaved edits included)"), ""}
	switch {
	case m.fontFile != "":
		out = append(out, faint.Render("Rendered with "+filepath.Base(m.fontFile)+" (shaped, so ligatures apply; glyphs this font lacks show as boxes):"))
		out = append(out, graphics.Placeholder(fontID, m.fontCols, m.fontRows)...)
	case m.fontErr != "":
		out = append(out, warnSty.Render(m.fontErr))
	case m.opts.Graphics:
		out = append(out, faint.Render("rendering samples in your font…"))
	default:
		out = append(out, faint.Render("This terminal can't show images, so samples use its own font (run inside Ghostty for real rendering):"))
	}
	if m.fontFile == "" {
		for _, s := range playgroundSamples {
			out = append(out, "  "+s)
		}
	}
	out = append(out, "", bold.Render("Which face Ghostty picks for each character:"))
	switch {
	case !m.facesDone:
		out = append(out, faint.Render("  asking ghostty +show-face…"))
	case errors.Is(m.facesErr, ghostty.ErrNotInstalled):
		out = append(out, warnSty.Render("  ghostty not available — install it to see font fallback"))
	case m.facesErr != nil:
		out = append(out, warnSty.Render("  "+m.facesErr.Error()))
	default:
		for _, l := range groupFaces(m.faces) {
			st := lipgloss.NewStyle()
			if strings.Contains(l, "not found") || strings.Contains(l, "Last Resort") {
				st = warnSty
			}
			out = append(out, "  "+st.Render(l))
		}
	}
	out = append(out, "", faint.Render("↑↓ scroll · esc close · edit font-family, then f again to re-check"))
	off := min(m.dlgOff, max(0, len(out)-h))
	return out[off:]
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
	if m.iconSent != "" { // only set when graphics are on
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
