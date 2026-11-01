package ui

import (
	"strconv"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"ghostty-config-tui/internal/ghostty"
	"ghostty-config-tui/internal/schema"
)

// PreviewColors are the colors the mock terminal is drawn with.
type PreviewColors struct {
	BG, FG, Cursor, CursorText, SelBG, SelFG, Bold string
	Palette                                        [16]string
}

// explicit is a value set by the user (or being edited), ignoring defaults.
func (m Model) explicit(key string) string {
	if m.ed != nil && !m.ed.list && m.ed.st.Key == key {
		if v, ok := m.ed.live(); ok {
			return v
		}
	}
	return m.scalar(key)
}

func hexOr(v, fallback string) string {
	if c := schema.NormalizeColor(v); schema.IsHex(c) {
		return c[:7]
	}
	return fallback
}

// Colors resolves Ghostty defaults, then the theme, then explicit keys.
func (m Model) Colors() PreviewColors {
	pc := PreviewColors{
		BG: hexOr(m.defaultOf("background"), "#282c34"),
		FG: hexOr(m.defaultOf("foreground"), "#ffffff"),
	}
	pc.Palette = ghostty.DefaultPalette
	pal, _ := m.sch.ByKey("palette")
	applyPalette(&pc.Palette, pal.Defaults)

	theme := m.explicit("theme")
	if m.ed != nil && m.ed.st.Kind == schema.Theme {
		theme = m.ed.highlighted()
	}
	if tc, ok := m.themeColors[ghostty.ThemeForMode(theme)]; ok && theme != "" {
		pc.BG, pc.FG = hexOr(tc.Background, pc.BG), hexOr(tc.Foreground, pc.FG)
		pc.Cursor, pc.CursorText = hexOr(tc.Cursor, ""), hexOr(tc.CursorText, "")
		pc.SelBG, pc.SelFG = hexOr(tc.SelBG, ""), hexOr(tc.SelFG, "")
		for i, c := range tc.Palette {
			pc.Palette[i] = hexOr(c, pc.Palette[i])
		}
	}
	pc.BG = hexOr(m.explicit("background"), pc.BG)
	pc.FG = hexOr(m.explicit("foreground"), pc.FG)
	entries := m.vals["palette"]
	if m.ed != nil && m.ed.list && m.ed.st.Key == "palette" {
		entries = m.ed.items
	}
	applyPalette(&pc.Palette, entries)

	special := func(v string) string {
		switch v {
		case "cell-foreground":
			return pc.FG
		case "cell-background":
			return pc.BG
		}
		return ""
	}
	cur := m.explicit("cursor-color")
	pc.Cursor = hexOr(cur, firstNonEmpty(special(cur), pc.Cursor, pc.FG))
	ct := m.explicit("cursor-text")
	pc.CursorText = hexOr(ct, firstNonEmpty(special(ct), pc.CursorText, pc.BG))
	sb := m.explicit("selection-background")
	pc.SelBG = hexOr(sb, firstNonEmpty(special(sb), pc.SelBG, pc.FG))
	sf := m.explicit("selection-foreground")
	pc.SelFG = hexOr(sf, firstNonEmpty(special(sf), pc.SelFG, pc.BG))
	bc := m.explicit("bold-color")
	pc.Bold = hexOr(bc, pc.FG)
	return pc
}

func (m Model) defaultOf(key string) string {
	st, _ := m.sch.ByKey(key)
	return st.Default
}

func applyPalette(pal *[16]string, entries []string) {
	for _, e := range entries {
		n, c, ok := strings.Cut(e, "=")
		i, err := strconv.Atoi(strings.TrimSpace(n))
		if !ok || err != nil || i < 0 || i > 15 {
			continue
		}
		if h := hexOr(c, ""); h != "" {
			pal[i] = h
		}
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

type seg struct {
	text   string
	fg, bg string
	bold   bool
}

func (pc PreviewColors) line(w int, segs ...seg) string {
	base := lipgloss.NewStyle().Background(lipgloss.Color(pc.BG))
	var sb strings.Builder
	used := 0
	for _, s := range segs {
		if used >= w {
			break
		}
		t := ansi.Truncate(s.text, w-used, "")
		st := base.Foreground(lipgloss.Color(firstNonEmpty(s.fg, pc.FG)))
		if s.bg != "" {
			st = st.Background(lipgloss.Color(s.bg))
		}
		if s.bold {
			st = st.Bold(true)
		}
		sb.WriteString(st.Render(t))
		used += lipgloss.Width(t)
	}
	if used < w {
		sb.WriteString(base.Render(strings.Repeat(" ", w-used)))
	}
	return sb.String()
}

// previewLines draws the mock terminal: prompt, ls/git output, palette
// swatches, selected text, and the cursor in its configured style.
func (m Model) previewLines(w int) []string {
	pc := m.Colors()
	p := pc.Palette
	font := firstNonEmpty(m.explicit("font-family"), firstOf(m.vals["font-family"]), "default font")
	size := firstNonEmpty(m.explicit("font-size"), m.defaultOf("font-size"))
	prompt := func(cmd string) []seg {
		return []seg{{text: " ~/code ", fg: p[4], bold: true}, {text: "❯ ", fg: p[5]}, {text: cmd}}
	}
	swatches := func(from int) []seg {
		s := []seg{{text: " "}}
		for i := from; i < from+8; i++ {
			s = append(s, seg{text: "   ", bg: p[i]})
		}
		return s
	}
	cursor := m.cursorSeg(pc)
	lines := []string{
		pc.line(w, seg{text: " ●", fg: p[1]}, seg{text: " ●", fg: p[3]}, seg{text: " ●", fg: p[2]},
			seg{text: "  " + font + " · " + size + "pt", fg: p[8]}),
		pc.line(w, prompt("ls")...),
		pc.line(w, seg{text: " src/  docs/", fg: pc.Bold, bold: true}, seg{text: "  build.sh", fg: p[2], bold: true},
			seg{text: "  main.go  README.md"}, seg{text: "  link", fg: p[6]}),
		pc.line(w, prompt("git status -s")...),
		pc.line(w, seg{text: "  M ui.go", fg: p[1]}, seg{text: "  A schema.go", fg: p[2]}, seg{text: "  ?? notes.txt", fg: p[3]}),
		pc.line(w, swatches(0)...),
		pc.line(w, swatches(8)...),
		pc.line(w, append(prompt("echo "), seg{text: "selected text", fg: pc.SelFG, bg: pc.SelBG}, seg{text: " "}, cursor)...),
	}
	// Typed commands replace the canned git lines; the prompt shows the input.
	for i, cmd := range m.history {
		lines[3+i] = pc.line(w, prompt(cmd)...)
	}
	if m.typing || m.typed != "" {
		lines[7] = pc.line(w, append(prompt(m.typed), cursor)...)
	}
	opacity := firstNonEmpty(m.explicit("background-opacity"), "1")
	blur := firstNonEmpty(m.explicit("background-blur"), "false")
	lines = append(lines, faint.Render(" opacity "+opacity+" · blur "+blur+" · cursor "+m.effective("cursor-style")))
	return lines
}

func firstOf(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return v[0]
}

func (m Model) cursorSeg(pc PreviewColors) seg {
	blinkSetting := m.effective("cursor-style-blink")
	blinks := blinkSetting == "" || blinkSetting == "true" // Ghostty blinks by default
	if blinks && !m.blinkOn {
		return seg{text: " "}
	}
	switch m.effective("cursor-style") {
	case "bar":
		return seg{text: "▎", fg: pc.Cursor}
	case "underline":
		return seg{text: "▁", fg: pc.Cursor}
	case "block_hollow":
		return seg{text: "▯", fg: pc.Cursor}
	}
	return seg{text: " ", fg: pc.CursorText, bg: pc.Cursor}
}
