package ui

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"ghostty-config-tui/internal/schema"
)

var (
	accent    = lipgloss.Color("#7aa2f7")
	dimColor  = lipgloss.Color("#6b7089")
	warnColor = lipgloss.Color("#e0af68")
	errColor  = lipgloss.Color("#f7768e")
	okColor   = lipgloss.Color("#9ece6a")

	bold    = lipgloss.NewStyle().Bold(true)
	faint   = lipgloss.NewStyle().Foreground(dimColor)
	hi      = lipgloss.NewStyle().Foreground(accent).Bold(true)
	sel     = lipgloss.NewStyle().Background(lipgloss.Color("#2f3549")).Bold(true)
	warnSty = lipgloss.NewStyle().Foreground(warnColor)
	errSty  = lipgloss.NewStyle().Foreground(errColor)
	okSty   = lipgloss.NewStyle().Foreground(okColor)
	badge   = lipgloss.NewStyle().Foreground(lipgloss.Color("#1a1b26")).Background(dimColor)
	badgeHi = lipgloss.NewStyle().Foreground(lipgloss.Color("#1a1b26")).Background(accent)
)

// View implements tea.Model in the alternate screen buffer.
func (m Model) View() tea.View {
	v := tea.NewView(m.Render())
	v.AltScreen = true
	return v
}

// Render draws the full screen as a string (used by View and tests).
func (m Model) Render() string {
	w, h := max(m.width, 60), max(m.height, 16)
	bodyH := h - 3
	header := m.header(w)
	var body string
	if m.dlg != dlgNone {
		body = box(m.dialogTitle(), m.dialogLines(w-2, bodyH-2), w, bodyH, true)
	} else {
		catW, detW := 22, max(44, (w-22)*11/20)
		if w < 110 {
			catW = 15
			detW = max(36, (w-catW)*9/20)
		}
		setW := w - catW - detW
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			box("Categories", m.catLines(catW-2, bodyH-2), catW, bodyH, m.pane == paneCats && m.ed == nil),
			box(m.listTitle(), m.rowLines(setW-2, bodyH-2), setW, bodyH, m.pane == paneSettings && m.ed == nil),
			box(m.detailTitle(), m.detailLines(detW-2, bodyH-2), detW, bodyH, m.ed != nil),
		)
	}
	return header + "\n" + body + "\n" + m.statusLines(w)
}

func (m Model) header(w int) string {
	left := hi.Render(" ghostty-config-tui ") + faint.Render(fmt.Sprintf(" %d settings", len(m.sch.Settings)))
	right := faint.Render("? help ")
	return left + strings.Repeat(" ", max(0, w-lipgloss.Width(left)-lipgloss.Width(right))) + right
}

// box draws a rounded border with a title around exactly-sized content.
func box(title string, lines []string, w, h int, focused bool) string {
	bc := faint
	if focused {
		bc = lipgloss.NewStyle().Foreground(accent)
	}
	inner := w - 2
	t := ansi.Truncate(" "+title+" ", max(0, inner-2), "…")
	top := bc.Render("╭─") + bold.Render(t) + bc.Render(strings.Repeat("─", max(0, inner-1-lipgloss.Width(t)))+"╮")
	var sb strings.Builder
	sb.WriteString(top)
	for i := 0; i < h-2; i++ {
		l := ""
		if i < len(lines) {
			l = lines[i]
		}
		sb.WriteString("\n" + bc.Render("│") + fit(l, inner) + bc.Render("│"))
	}
	sb.WriteString("\n" + bc.Render("╰"+strings.Repeat("─", max(0, inner))+"╯"))
	return sb.String()
}

// fit truncates or pads an ANSI string to exactly w cells.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(0, w-lipgloss.Width(s)))
}

// window keeps the cursor row visible in a list of height h.
func window(n, cursor, h int) (int, int) {
	if n <= h {
		return 0, n
	}
	start := min(max(cursor-h/2, 0), n-h)
	return start, start + h
}

func (m Model) catLines(w, h int) []string {
	var out []string
	start, end := window(len(m.cats), m.cat, h)
	for i := start; i < end; i++ {
		c := m.cats[i]
		set := 0
		var rows []schema.Setting
		if c == ModifiedCategory {
			set = len(m.vals)
		} else {
			rows = m.sch.InCategory(c)
			for _, s := range rows {
				if len(m.vals[s.Key]) > 0 {
					set++
				}
			}
		}
		count := ""
		if set > 0 {
			count = itoa(set)
		}
		line := " " + fit(c, w-5) + fmt.Sprintf("%3s ", count)
		switch {
		case i == m.cat && m.search == "" && !m.searching:
			if m.pane == paneCats {
				line = sel.Render(line)
			} else {
				line = hi.Render(line)
			}
		case set > 0:
			line = okSty.Render(line)
		}
		out = append(out, line)
	}
	return out
}

func (m Model) listTitle() string {
	if m.searching || m.search != "" {
		return fmt.Sprintf("Search: %s%s (%d)", m.search, cursorMark(m.searching), len(m.Rows()))
	}
	return m.cats[m.cat]
}

func cursorMark(on bool) string {
	if on {
		return "▌"
	}
	return ""
}

func (m Model) rowLines(w, h int) []string {
	rows := m.Rows()
	if len(rows) == 0 {
		if m.cats[m.cat] == ModifiedCategory && m.search == "" {
			return []string{faint.Render(" nothing set yet — every setting"), faint.Render(" is at Ghostty's default")}
		}
		return []string{faint.Render(" no matching settings")}
	}
	var out []string
	start, end := window(len(rows), m.row, h)
	valW := min(max(w/3, 6), 24)
	for i := start; i < end; i++ {
		s := rows[i]
		mark := " "
		switch {
		case m.dirtyKey(s.Key):
			mark = warnSty.Render("●")
		case len(m.vals[s.Key]) > 0:
			mark = okSty.Render("●")
		}
		name := fit(s.Title, w-valW-3)
		val := m.rowValue(s, valW)
		line := mark + " " + name + " " + val
		if i == m.row {
			if m.pane == paneSettings {
				line = mark + sel.Render(" "+name+" ") + val
			} else {
				line = mark + hi.Render(" "+name+" ") + val
			}
		}
		out = append(out, line)
	}
	return out
}

func (m Model) rowValue(s schema.Setting, w int) string {
	vals := m.vals[s.Key]
	if s.Repeatable {
		if len(vals) == 0 {
			return faint.Render(fit("—", w))
		}
		return fit(plural(len(vals), "entry"), w)
	}
	v := m.scalar(s.Key)
	styled := func(txt string) string {
		if v == "" {
			return faint.Render(fit(txt, w))
		}
		return fit(txt, w)
	}
	shown := v
	if shown == "" {
		shown = s.Default
	}
	if shown == "" {
		return faint.Render(fit("—", w))
	}
	if s.Kind == schema.Color && schema.IsHex(schema.NormalizeColor(shown)) {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(schema.NormalizeColor(shown))).Render("██") + " " + styled(shown)[0:]
	}
	return styled(shown)
}

func (m Model) detailTitle() string {
	if m.ed != nil {
		return "Edit " + m.ed.st.Key
	}
	if st, ok := m.Current(); ok {
		return st.Key
	}
	return "Detail"
}

func (m Model) detailLines(w, h int) []string {
	var out []string
	if m.showPreview && h >= 18 {
		out = append(out, m.previewLines(w)...)
		out = append(out, faint.Render(strings.Repeat("─", w)))
	}
	st, ok := m.Current()
	if m.ed != nil {
		st, ok = m.ed.st, true
	}
	if !ok {
		return out
	}
	out = append(out, bold.Render(st.Title)+"  "+m.badges(st))
	if m.ed != nil {
		out = append(out, m.editorLines(m.ed, w)...)
	} else {
		out = append(out, m.valueLines(st, w)...)
	}
	out = append(out, "")
	docs := wrap(st.Docs, w)
	room := h - len(out)
	off := min(m.docOff, max(0, len(docs)-room))
	if room > 0 && off < len(docs) {
		end := min(len(docs), off+room)
		for _, l := range docs[off:end] {
			out = append(out, faint.Render(l))
		}
		if end < len(docs) && len(out) > 0 {
			out[len(out)-1] = faint.Render(fmt.Sprintf("… pgdn for more (%d/%d)", end, len(docs)))
		}
	}
	return out
}

func (m Model) badges(st schema.Setting) string {
	var b []string
	for _, p := range st.Platforms {
		b = append(b, badgeHi.Render(" "+p+" "))
	}
	if st.Since != "" {
		b = append(b, badge.Render(" ≥ "+st.Since+" "))
	}
	b = append(b, badge.Render(" "+string(st.Kind)+" "))
	if st.Repeatable {
		b = append(b, badge.Render(" list "))
	}
	return strings.Join(b, " ")
}

func (m Model) valueLines(st schema.Setting, w int) []string {
	var out []string
	if st.Repeatable {
		vals := m.vals[st.Key]
		if len(vals) == 0 {
			out = append(out, faint.Render("no entries set"))
		}
		for i, v := range vals {
			if i == 6 {
				out = append(out, faint.Render(fmt.Sprintf("… %d more", len(vals)-6)))
				break
			}
			out = append(out, "• "+v)
		}
		if n := len(st.Defaults); n > 0 {
			out = append(out, faint.Render(fmt.Sprintf("Ghostty ships %s; entries add to them", plural(n, "default"))))
		}
		out = append(out, faint.Render("enter: open list editor"))
		return out
	}
	v := m.scalar(st.Key)
	if v == "" {
		out = append(out, "value  "+faint.Render(orNone(st.Default)+" (default)"))
	} else {
		out = append(out, "value  "+hi.Render(v), faint.Render("default "+orNone(st.Default)))
	}
	if m.dirtyKey(st.Key) {
		out = append(out, warnSty.Render("unsaved · file has "+orNone(strings.Join(m.orig[st.Key], ", "))))
	}
	hint := "enter: edit · d: reset to default"
	if st.Kind == schema.Bool {
		hint = "enter/space: toggle · d: reset to default"
	}
	out = append(out, faint.Render(hint))
	return out
}

func (m Model) editorLines(e *editor, w int) []string {
	if e.list {
		return m.listLines(e, w)
	}
	return m.scalarLines(e, w)
}

func (m Model) listLines(e *editor, w int) []string {
	var out []string
	if len(e.items) == 0 {
		out = append(out, faint.Render("(empty) — a: add entry"))
	}
	start, end := window(len(e.items), e.idx, 8)
	for i := start; i < end; i++ {
		v := e.items[i]
		line := "  " + m.itemPreview(e.st, v)
		if i == e.idx && e.sub == nil {
			line = sel.Render("> " + v)
		}
		out = append(out, line)
	}
	if e.sub != nil {
		label := "new entry"
		if e.subAt >= 0 {
			label = "entry " + itoa(e.subAt+1)
		}
		out = append(out, hi.Render("─ "+label+" ─"))
		out = append(out, m.scalarLines(e.sub, w)...)
		return out
	}
	if e.warn != "" {
		out = append(out, warnSty.Render("warning: "+e.warn))
	}
	out = append(out, faint.Render("a add · enter edit · x delete · K/J move · esc done"))
	return out
}

func (m Model) itemPreview(st schema.Setting, v string) string {
	if st.Kind == schema.Palette {
		if _, c, ok := strings.Cut(v, "="); ok && schema.IsHex(schema.NormalizeColor(c)) {
			return lipgloss.NewStyle().Foreground(lipgloss.Color(schema.NormalizeColor(c))).Render("██") + " " + v
		}
	}
	return v
}

func (m Model) scalarLines(e *editor, w int) []string {
	var out []string
	input := func(label, v string, on bool) string {
		s := label + v
		if on {
			s += "▌"
		}
		return s
	}
	switch {
	case e.st.Kind == schema.Keybind:
		out = append(out, m.keybindLines(e, w)...)
	case e.picker():
		out = append(out, input("filter ", e.buf, true))
		ms := e.matches()
		if e.buf != "" {
			row := "  use \"" + e.buf + "\" as typed"
			if e.idx < 0 {
				row = sel.Render("> use \"" + e.buf + "\" as typed")
			}
			out = append(out, row)
		}
		start, end := window(len(ms), e.idx, 8)
		for i := start; i < end; i++ {
			if i == e.idx {
				out = append(out, sel.Render("> "+ms[i]))
			} else {
				out = append(out, "  "+ms[i])
			}
		}
		switch {
		case len(ms) == 0 && e.buf != "":
			out = append(out, warnSty.Render("no match — enter uses the typed value"))
		case len(ms) > 0:
			out = append(out, faint.Render(fmt.Sprintf("%d/%d · ↑↓ move · type to filter · enter pick", e.idx+1, len(ms))))
		}
		if e.st.Kind == schema.Theme {
			out = append(out, faint.Render("preview follows the highlighted theme"))
		}
	case e.st.Kind == schema.Flags:
		for i, o := range e.st.Options {
			box := "[ ]"
			if e.flags[o] {
				box = okSty.Render("[x]")
			}
			line := box + " " + o
			if e.flags[o] != schema.FlagState("", e.st.Default, o) {
				line += warnSty.Render("  (changed)")
			}
			if i == e.idx {
				line = sel.Render("›") + line
			} else {
				line = " " + line
			}
			out = append(out, line)
		}
		out = append(out, faint.Render("space toggle · enter apply · esc cancel"))
	case e.st.Kind == schema.Int || e.st.Kind == schema.Float:
		out = append(out, input("value ", e.buf, true))
		if e.st.HasRange {
			f, _ := schema.ParseFloat(e.buf)
			out = append(out, slider(f, e.st.Min, e.st.Max, w-14)+fmt.Sprintf(" %s–%s", schema.FormatNumber(e.st.Min), schema.FormatNumber(e.st.Max)))
		}
		out = append(out, faint.Render("←/→ step "+schema.FormatNumber(e.st.Step)+" · shift ×10 · type a number"))
	case e.st.Kind == schema.Color:
		c := schema.NormalizeColor(e.buf)
		sw := faint.Render("(no preview)")
		if schema.IsHex(c) {
			sw = lipgloss.NewStyle().Background(lipgloss.Color(c)).Render("        ")
		}
		out = append(out, input("color ", e.buf, true)+"  "+sw)
		rgb := e.RGB()
		for i, name := range []string{"R", "G", "B"} {
			l := name + " " + slider(float64(rgb[i]), 0, 255, w-12) + fmt.Sprintf(" %3d", rgb[i])
			if i == e.idx {
				l = hi.Render("›") + l
			} else {
				l = " " + l
			}
			out = append(out, l)
		}
		hint := "↑↓ channel · ←/→ ±1 · shift ±16 · type hex or X11 name"
		if len(e.st.Options) > 0 {
			hint += " · also: " + strings.Join(e.st.Options, ", ")
		}
		out = append(out, faint.Render(hint))
	case e.st.Kind == schema.Duration:
		out = append(out, input("duration ", e.buf, true), faint.Render("units: ms s m h d · e.g. 750ms, 1h30m"))
	default:
		out = append(out, input("value ", e.buf, true))
		if e.st.Kind == schema.Palette {
			out = append(out, faint.Render("format: N=#RRGGBB with N from 0 to 255"))
		}
	}
	if e.err != "" {
		out = append(out, errSty.Render("✗ "+e.err))
	} else if v, warn, err := schema.Validate(e.st, e.value()); err == nil && warn != "" && v != "" {
		out = append(out, warnSty.Render("! "+warn))
	}
	if e.st.Kind != schema.Flags {
		out = append(out, faint.Render("enter apply · esc cancel · ctrl+u clear"))
	}
	return out
}

func (m Model) keybindLines(e *editor, w int) []string {
	label := func(i int, s string) string {
		if e.kbField == i {
			return hi.Render("› " + s)
		}
		return "  " + s
	}
	var pre []string
	for i, p := range schema.KeybindPrefixes {
		t := "[ ] " + p
		if e.kbPrefix[p] {
			t = "[x] " + p
		}
		if e.kbField == 0 && i == e.kbPreIdx {
			t = sel.Render(t)
		}
		pre = append(pre, t)
	}
	trig := e.kbTrig
	switch e.kbRecord {
	case 1:
		trig = warnSty.Render("press the key combo…")
	case 2:
		trig += warnSty.Render(" > press the next key…")
	}
	out := []string{
		label(0, "prefixes ") + strings.Join(pre, " "),
		label(1, "trigger  ") + trig + cursorMark(e.kbField == 1 && e.kbRecord == 0),
		label(2, "action   ") + e.highlighted() + faint.Render("  filter: "+e.kbAct) + cursorMark(e.kbField == 2),
	}
	if e.kbField == 2 {
		ms := e.matches()
		start, end := window(len(ms), e.idx, 5)
		for i := start; i < end; i++ {
			if i == e.idx {
				out = append(out, "    "+sel.Render(ms[i]))
			} else {
				out = append(out, "    "+faint.Render(ms[i]))
			}
		}
	}
	out = append(out, label(3, "param    ")+e.kbParam+cursorMark(e.kbField == 3))
	v := e.value()
	out = append(out, "", "keybind = "+hi.Render(v))
	if kb, err := schema.ParseKeybind(v); err != nil {
		out = append(out, errSty.Render("✗ "+err.Error()))
	} else if warn := kb.Warning(m.sch.Actions); warn != "" {
		out = append(out, warnSty.Render("! "+warn))
	} else if def := m.defaultBinding(kb.Trigger); def != "" {
		out = append(out, warnSty.Render("! overrides default "+def))
	} else {
		out = append(out, okSty.Render("✓ valid"))
	}
	out = append(out, faint.Render("tab field · ctrl+r record key · ctrl+t record next step (sequence)"))
	return out
}

// defaultBinding returns the default keybind using trigger, if any.
func (m Model) defaultBinding(trigger string) string {
	st, _ := m.sch.ByKey("keybind")
	for _, d := range st.Defaults {
		if kb, err := schema.ParseKeybind(d); err == nil && kb.Trigger == trigger {
			return d
		}
	}
	return ""
}

func slider(v, lo, hi float64, w int) string {
	w = max(w, 4)
	pos := 0
	if hi > lo {
		pos = int((v - lo) / (hi - lo) * float64(w-1))
	}
	pos = min(max(pos, 0), w-1)
	return lipgloss.NewStyle().Foreground(accent).Render(strings.Repeat("━", pos)+"●") + faint.Render(strings.Repeat("─", w-1-pos))
}

// wrap word-wraps docs, keeping blank lines and indented code as-is.
func wrap(text string, w int) []string {
	// Rejoin Ghostty's hard-wrapped lines into paragraphs first.
	var paras []string
	for _, l := range strings.Split(text, "\n") {
		t := strings.TrimSpace(l)
		n := len(paras)
		joinable := n > 0 && paras[n-1] != "" && t != "" &&
			!strings.HasPrefix(l, "    ") && !strings.HasPrefix(paras[n-1], "    ") &&
			!strings.HasPrefix(t, "* ") && !strings.HasPrefix(t, "- ")
		if joinable {
			paras[n-1] += " " + t
			continue
		}
		paras = append(paras, l)
	}
	var out []string
	for _, para := range paras {
		if strings.TrimSpace(para) == "" {
			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			continue
		}
		if strings.HasPrefix(para, "    ") {
			out = append(out, para)
			continue
		}
		indent := para[:len(para)-len(strings.TrimLeft(para, " "))]
		line := indent
		for _, word := range strings.Fields(para) {
			if lipgloss.Width(line)+len(word)+1 > w && strings.TrimSpace(line) != "" {
				out = append(out, line)
				line = indent
			}
			if strings.TrimSpace(line) != "" {
				line += " "
			}
			line += word
		}
		out = append(out, line)
	}
	return out
}

func (m Model) statusLines(w int) string {
	dirty := okSty.Render("saved")
	if n := len(m.changed()); n > 0 {
		dirty = warnSty.Render("● " + itoa(n) + " unsaved")
	}
	if m.opts.DryRun {
		dirty += warnSty.Render(" [dry-run]")
	}
	msg := m.msg
	if m.msgErr {
		msg = errSty.Render(msg)
	}
	path := m.opts.ConfigPath
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}
	line1 := " " + dirty + "  " + msg
	if gap := w - lipgloss.Width(line1) - lipgloss.Width(path) - 2; gap > 0 {
		line1 += strings.Repeat(" ", gap) + faint.Render(path)
	}
	var hints string
	switch {
	case m.dlg != dlgNone:
		hints = "esc close"
	case m.ed != nil:
		hints = "enter apply · esc cancel"
	case m.searching:
		hints = "type to search keys and docs · enter keep results · esc clear"
	default:
		hints = "↑↓ move · ←→ pane · enter edit · / search · s save · u undo · r revert · d default · i import · x export · p preview · q quit"
	}
	return fit(line1, w) + "\n" + fit(faint.Render(" "+hints), w)
}

func (m Model) dialogTitle() string {
	switch m.dlg {
	case dlgHelp:
		return "Help"
	case dlgImport:
		return "Import config"
	case dlgExport:
		return "Export minimal config"
	}
	return "Export to file"
}

func (m Model) dialogLines(w, h int) []string {
	switch m.dlg {
	case dlgHelp:
		return helpLines
	case dlgImport:
		lines := strings.Split(m.dlgBuf, "\n")
		out := []string{
			"Paste a config (bracketed paste) or type a file path, then enter.",
			"Imported values are staged; nothing is written until you press s.",
			"",
		}
		if len(lines) > 1 {
			out = append(out, hi.Render(fmt.Sprintf("%d pasted lines:", len(lines))))
			for _, l := range lines[:min(len(lines), h-6)] {
				out = append(out, "  "+l)
			}
		} else {
			out = append(out, "path or text: "+m.dlgBuf+"▌")
		}
		return out
	case dlgExportPath:
		return []string{"Write the minimal config to:", "", m.dlgBuf + "▌", "", faint.Render("enter write · esc cancel")}
	}
	text := m.ExportText()
	if text == "" {
		text = "# every setting is at its Ghostty default\n"
	}
	lines := append([]string{faint.Render("c copy to clipboard (OSC 52) · w write to file · ↑↓ scroll · esc close"), ""}, strings.Split(strings.TrimRight(text, "\n"), "\n")...)
	off := min(m.dlgOff, max(0, len(lines)-h))
	return lines[off:]
}

var helpLines = []string{
	bold.Render("Navigate"),
	"  ↑/↓ j/k       move          ←/→ h/l tab   switch categories / settings",
	"  /             search every key and its docs      g/G   first / last",
	"  pgup/pgdn     scroll docs   p   toggle live preview",
	"",
	bold.Render("Edit"),
	"  enter         edit (bools toggle; lists open the list editor)",
	"  space         toggle a bool",
	"  r / R         revert field / all unsaved changes to the file",
	"  d             reset to Ghostty default (removes the line)",
	"",
	bold.Render("Editors"),
	"  pickers       type to filter, ↑/↓ choose, enter pick (custom text allowed)",
	"  numbers       ←/→ step, shift ×10       colors   ↑/↓ channel, ←/→ adjust",
	"  flags         space toggles a flag      lists    a add, x delete, K/J move",
	"  keybinds      tab between fields, ctrl+r records a key, ctrl+t adds a step",
	"",
	bold.Render("File"),
	"  s             save (atomic; only changed keys are rewritten)",
	"  u             undo the last save        i / x   import / export",
	"  q             quit (asks again when there are unsaved changes)",
	"",
	faint.Render("After saving, reload Ghostty with ctrl+shift+r. Press any key to close."),
}
