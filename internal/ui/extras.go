package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"ghostty-config-tui/internal/ghostty"
)

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
	out := []string{
		"font-family: " + hi.Render(fam) + faint.Render("  (staged values, unsaved edits included)"),
		faint.Render("This terminal draws the samples in its own font; Ghostty's resolution for your fonts is below."),
		"",
	}
	for _, s := range playgroundSamples {
		out = append(out, "  "+s)
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

// iconLines draws a small block-art approximation of the Dock icon.
func (m Model) iconLines() []string {
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
