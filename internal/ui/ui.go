// Package ui is the Bubble Tea fullscreen editor: categories, settings,
// a detail/editor pane with live preview, and a status bar.
package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"ghostty-config-tui/internal/config"
	"ghostty-config-tui/internal/ghostty"
	"ghostty-config-tui/internal/schema"
)

// Options wires the model to storage and picker data.
type Options struct {
	ConfigPath string
	Values     map[string][]string // keys present in the config file
	Themes     []string
	Fonts      []string
	// OnSave persists changed keys (nil slice = remove the key) and returns
	// Ghostty's validation problems for the written file.
	OnSave func(changed map[string][]string) (problems []string, err error)
	// OnUndo restores the file as it was before the last save.
	OnUndo   func() (map[string][]string, error)
	DryRun   bool
	Warning  string // shown at startup, e.g. ghostty missing
	Graphics bool   // terminal speaks the Kitty graphics protocol
}

// ThemeResolvedMsg delivers asynchronously loaded theme colors.
type ThemeResolvedMsg struct {
	Name   string
	Colors ghostty.ThemeColors
	OK     bool
}

// BlinkMsg flips the preview cursor blink phase.
type BlinkMsg struct{}

var resolveTheme = ghostty.ResolveThemeColors

func resolveThemeCmd(name string) tea.Cmd {
	return func() tea.Msg {
		c, ok := resolveTheme(name)
		return ThemeResolvedMsg{Name: name, Colors: c, OK: ok}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(530*time.Millisecond, func(time.Time) tea.Msg { return BlinkMsg{} })
}

type pane int

const (
	paneCats pane = iota
	paneSettings
)

type dialog int

const (
	dlgNone dialog = iota
	dlgHelp
	dlgImport
	dlgExport
	dlgExportPath
	dlgFonts
)

// ModifiedCategory lists every key set in the file or staged.
const ModifiedCategory = "★ Modified"

// Model is the Tea Elm model.
type Model struct {
	sch         *schema.Schema
	cats        []string
	cat         int
	row, rowOff int
	pane        pane
	search      string
	searching   bool

	vals, orig map[string][]string
	opts       Options

	themeColors map[string]ghostty.ThemeColors
	themeAsked  map[string]bool

	ed *editor

	dlg    dialog
	dlgBuf string
	dlgOff int

	iconSent  string // signature of the icon last uploaded
	fontImg   bool   // playground font image uploaded
	fontCols  int
	fontRows  int
	fontFile  string
	fontErr   string
	faces     []string // font playground results
	facesOK   bool
	facesDone bool

	typing  bool     // keys go to the preview's prompt
	typed   string   // current preview command line
	history []string // last preview commands

	msg         string
	msgErr      bool
	armed       string // "q" or "u" awaiting confirmation
	showPreview bool
	docOff      int
	blinkOn     bool
	width       int
	height      int
}

// New builds a Model. Every setting comes from internal/schema.
func New(opts Options) Model {
	sch := schema.Active()
	cats := []string{ModifiedCategory}
	for _, c := range schema.Categories {
		if len(sch.InCategory(c)) > 0 {
			cats = append(cats, c)
		}
	}
	m := Model{
		sch:         sch,
		cats:        cats,
		cat:         1,
		pane:        paneSettings,
		vals:        clone(opts.Values),
		orig:        clone(opts.Values),
		opts:        opts,
		themeColors: map[string]ghostty.ThemeColors{},
		themeAsked:  map[string]bool{},
		showPreview: true,
		blinkOn:     true,
		width:       120,
		height:      36,
		msg:         opts.Warning,
	}
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tickCmd()}
	if t := m.scalar("theme"); t != "" {
		cmds = append(cmds, resolveThemeCmd(t))
	}
	return tea.Batch(cmds...)
}

func clone(in map[string][]string) map[string][]string {
	out := make(map[string][]string, len(in))
	for k, v := range in {
		if len(v) > 0 {
			out[k] = slices.Clone(v)
		}
	}
	return out
}

// Rows returns the settings listed in the middle pane.
func (m Model) Rows() []schema.Setting {
	if m.searching || m.search != "" {
		return m.sch.Search(m.search)
	}
	if m.cats[m.cat] == ModifiedCategory {
		var out []schema.Setting
		for _, s := range m.sch.Settings {
			if len(m.vals[s.Key]) > 0 || m.dirtyKey(s.Key) {
				out = append(out, s)
			}
		}
		return out
	}
	return m.sch.InCategory(m.cats[m.cat])
}

// Current returns the highlighted setting.
func (m Model) Current() (schema.Setting, bool) {
	rows := m.Rows()
	if m.row < 0 || m.row >= len(rows) {
		return schema.Setting{}, false
	}
	return rows[m.row], true
}

// Values returns a copy of the staged values.
func (m Model) Values() map[string][]string { return clone(m.vals) }

// Editing reports whether an editor is open.
func (m Model) Editing() bool { return m.ed != nil }

// Message returns the status message.
func (m Model) Message() string { return m.msg }

func (m Model) scalar(key string) string {
	v := m.vals[key]
	if len(v) == 0 {
		return ""
	}
	return v[len(v)-1]
}

// effective is the value Ghostty would use, including an open edit.
func (m Model) effective(key string) string {
	if m.ed != nil && m.ed.st.Key == key {
		if v, ok := m.ed.live(); ok && v != "" {
			return v
		}
	}
	if v := m.scalar(key); v != "" {
		return v
	}
	if s, ok := m.sch.ByKey(key); ok {
		return s.Default
	}
	return ""
}

func (m Model) dirtyKey(key string) bool { return !slices.Equal(m.vals[key], m.orig[key]) }

// Dirty reports unsaved changes.
func (m Model) Dirty() bool { return len(m.changed()) > 0 }

func (m Model) changed() map[string][]string {
	out := map[string][]string{}
	for k := range m.vals {
		if m.dirtyKey(k) {
			out[k] = slices.Clone(m.vals[k])
		}
	}
	for k := range m.orig {
		if m.dirtyKey(k) {
			out[k] = slices.Clone(m.vals[k])
		}
	}
	return out
}

// set stages values for key. A scalar set back to Ghostty's default is
// dropped only when the file never had the key, so the config stays minimal
// while explicit pins (e.g. overriding a theme's color) are kept.
func (m *Model) set(key string, values []string) {
	st, _ := m.sch.ByKey(key)
	if !st.Repeatable && len(values) == 1 {
		if values[0] == "" || (values[0] == st.Default && len(m.orig[key]) == 0) {
			values = nil
		}
	}
	if len(values) == 0 {
		delete(m.vals, key)
		return
	}
	m.vals[key] = slices.Clone(values)
}

func (m *Model) note(s string)    { m.msg, m.msgErr = s, false }
func (m *Model) noteErr(s string) { m.msg, m.msgErr = s, true }

func (m *Model) requestTheme(name string) tea.Cmd {
	name = ghostty.ThemeForMode(name)
	if name == "" || m.themeAsked[name] {
		return nil
	}
	m.themeAsked[name] = true
	return resolveThemeCmd(name)
}

func (m *Model) clampRow() {
	n := len(m.Rows())
	if m.row >= n {
		m.row = n - 1
	}
	if m.row < 0 {
		m.row = 0
	}
	m.docOff = 0
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case BlinkMsg:
		m.blinkOn = !m.blinkOn
		return m, tickCmd()
	case ThemeResolvedMsg:
		if msg.OK {
			m.themeColors[ghostty.ThemeForMode(msg.Name)] = msg.Colors
		}
		return m, nil
	case FacesMsg:
		m.faces, m.facesOK, m.facesDone = msg.Lines, msg.OK, true
		return m, nil
	case FontImageMsg:
		return m.fontImage(msg)
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.PasteMsg:
		return m.paste(msg.Content), nil
	case tea.KeyPressMsg:
		// Some terminals and input bursts deliver Enter as a bare line feed.
		if s := msg.String(); (s == "ctrl+j" || s == "ctrl+m") && !m.recording() {
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		}
		res, cmd := m.key(msg)
		next := res.(Model)
		if c := next.syncIcon(); c != nil {
			return next, tea.Batch(cmd, c)
		}
		return next, cmd
	}
	return m, nil
}

// recording reports whether the keybind builder is capturing a raw key.
func (m Model) recording() bool {
	if m.ed == nil {
		return false
	}
	return m.ed.kbRecord > 0 || (m.ed.sub != nil && m.ed.sub.kbRecord > 0)
}

func (m Model) paste(s string) Model {
	// Terminals deliver pasted newlines as \r or \r\n.
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	switch {
	case m.dlg == dlgImport || m.dlg == dlgExportPath:
		m.dlgBuf += s
	case m.typing:
		m.typed += strings.ReplaceAll(s, "\n", " ")
	case m.ed != nil:
		m.ed.paste(s)
	}
	return m
}

func (m Model) key(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		return m, tea.Quit
	}
	if m.dlg != dlgNone {
		return m.dialogKey(msg)
	}
	if m.ed != nil {
		cmd := m.editKey(msg)
		return m, cmd
	}
	if m.typing {
		switch k {
		case "esc":
			m.typing = false
			m.note("")
		case "enter":
			m.history = append(m.history, m.typed)
			if len(m.history) > 2 {
				m.history = m.history[1:]
			}
			m.typed = ""
		case "backspace":
			m.typed = trimLast(m.typed)
		default:
			m.typed += msg.Text
		}
		return m, nil
	}
	if m.searching {
		switch k {
		case "esc":
			m.searching, m.search = false, ""
		case "enter", "down":
			m.searching = false
		case "backspace":
			m.search = trimLast(m.search)
		default:
			if t := msg.Text; t != "" {
				m.search += t
			}
		}
		m.clampRow()
		return m, nil
	}
	if k != m.armed {
		m.armed = ""
	}
	rows := m.Rows()
	switch k {
	case "q":
		if m.Dirty() && m.armed != "q" {
			m.armed = "q"
			m.noteErr("unsaved changes: q again to discard, s to save")
			return m, nil
		}
		return m, tea.Quit
	case "?":
		m.dlg = dlgHelp
	case "/":
		m.searching, m.search, m.row = true, "", 0
	case "esc":
		m.search, m.msg = "", ""
		m.clampRow()
	case "tab", "left", "h", "right", "l":
		if k == "left" || k == "h" {
			m.pane = paneCats
		} else if k == "right" || k == "l" {
			m.pane = paneSettings
		} else {
			m.pane = 1 - m.pane
		}
	case "down", "j":
		m.move(1)
	case "up", "k":
		m.move(-1)
	case "pgdown", "ctrl+d":
		m.docOff += 5
	case "pgup", "ctrl+u":
		m.docOff = max(0, m.docOff-5)
	case "g", "home":
		m.row = 0
	case "G", "end":
		m.row = max(0, len(rows)-1)
	case "p":
		m.showPreview = !m.showPreview
	case "t":
		m.typing, m.showPreview = true, true
		m.note("typing in the preview — esc to stop")
	case "enter", "e", "space", " ":
		if m.pane == paneCats {
			m.pane = paneSettings
			return m, nil
		}
		return m, m.startEdit(k == "space" || k == " ")
	case "s":
		return m.save(), nil
	case "u":
		if m.Dirty() && m.armed != "u" {
			m.armed = "u"
			m.noteErr("undo discards unsaved edits: u again to undo the last save")
			return m, nil
		}
		m.armed = ""
		return m.undo()
	case "r":
		if st, ok := m.Current(); ok {
			if v := m.orig[st.Key]; len(v) > 0 {
				m.vals[st.Key] = slices.Clone(v)
			} else {
				delete(m.vals, st.Key)
			}
			m.note("reverted " + st.Key + " to the file value")
		}
	case "R":
		m.vals = clone(m.orig)
		m.note("reverted all unsaved changes")
	case "d", "delete":
		if st, ok := m.Current(); ok {
			m.set(st.Key, nil)
			m.note(st.Key + " reset to Ghostty default (" + orNone(st.Default) + ")")
		}
	case "i":
		m.dlg, m.dlgBuf = dlgImport, ""
	case "x":
		m.dlg, m.dlgOff = dlgExport, 0
	case "f":
		m.dlg, m.faces, m.facesDone = dlgFonts, nil, false
		m.fontImg, m.fontErr = false, ""
		if m.opts.Graphics {
			return m, tea.Batch(m.facesCmd(), m.renderFontCmd())
		}
		return m, m.facesCmd()
	}
	return m, nil
}

func (m *Model) move(d int) {
	if m.pane == paneCats {
		m.cat = (m.cat + d + len(m.cats)) % len(m.cats)
		m.row, m.rowOff, m.search = 0, 0, ""
		m.docOff = 0
		return
	}
	n := len(m.Rows())
	if n == 0 {
		return
	}
	m.row = min(max(m.row+d, 0), n-1)
	m.docOff = 0
}

func (m *Model) startEdit(space bool) tea.Cmd {
	st, ok := m.Current()
	if !ok {
		return nil
	}
	if st.Kind == schema.Bool && !st.Repeatable {
		on, _ := schema.ParseBool(m.effective(st.Key))
		m.set(st.Key, []string{boolStr(!on)})
		m.note(st.Key + " = " + boolStr(!on))
		return nil
	}
	if space {
		return nil
	}
	m.ed = m.newEditor(st)
	m.note("editing " + st.Key)
	if st.Kind == schema.Theme {
		return m.requestTheme(m.ed.highlighted())
	}
	return nil
}

func (m Model) save() Model {
	changed := m.changed()
	if len(changed) == 0 {
		m.note("nothing to save")
		return m
	}
	if m.opts.OnSave == nil {
		m.noteErr("no save handler")
		return m
	}
	problems, err := m.opts.OnSave(changed)
	if err != nil {
		m.noteErr("save failed: " + err.Error())
		return m
	}
	m.orig = clone(m.vals)
	switch {
	case m.opts.DryRun:
		m.note("dry-run: " + plural(len(changed), "change") + " not written")
	case len(problems) > 0:
		m.noteErr("saved, but Ghostty reports: " + strings.Join(problems, "; "))
	default:
		m.note("saved " + plural(len(changed), "change") + " — reload Ghostty with ctrl+shift+r")
	}
	return m
}

func (m Model) undo() (Model, tea.Cmd) {
	if m.opts.OnUndo == nil {
		m.noteErr("no undo handler")
		return m, nil
	}
	restored, err := m.opts.OnUndo()
	if err != nil {
		m.noteErr("undo failed: " + err.Error())
		return m, nil
	}
	m.vals, m.orig = clone(restored), clone(restored)
	m.note("restored the file from before the last save")
	return m, m.requestTheme(m.scalar("theme"))
}

func (m Model) dialogKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	switch m.dlg {
	case dlgHelp, dlgFonts:
		m.dlg = dlgNone
	case dlgExport:
		switch k {
		case "esc", "q", "x":
			m.dlg = dlgNone
		case "c", "y":
			m.dlg = dlgNone
			m.note("copied minimal config to the clipboard")
			return m, tea.SetClipboard(m.ExportText())
		case "w":
			m.dlg, m.dlgBuf = dlgExportPath, "~/ghostty-config-export.ghostty"
		case "u":
			link, ok := m.ShareURL()
			if !ok {
				m.noteErr("config too large for a share link (over 1800 chars) — use c or w instead")
				return m, nil
			}
			m.dlg = dlgNone
			m.note("copied share link for ghostty.zerebos.com to the clipboard")
			return m, tea.SetClipboard(link)
		case "down", "j":
			m.dlgOff++
		case "up", "k":
			m.dlgOff = max(0, m.dlgOff-1)
		}
	case dlgExportPath, dlgImport:
		switch k {
		case "esc":
			m.dlg = dlgNone
		case "enter":
			if m.dlg == dlgImport {
				return m.doImport(), m.requestTheme(m.scalar("theme"))
			}
			path := expandHome(strings.TrimSpace(m.dlgBuf))
			if err := config.AtomicWrite(path, m.ExportText(), 0); err != nil {
				m.noteErr("export failed: " + err.Error())
			} else {
				m.note("exported to " + path)
			}
			m.dlg = dlgNone
		case "backspace":
			m.dlgBuf = trimLast(m.dlgBuf)
		case "ctrl+u":
			m.dlgBuf = ""
		default:
			m.dlgBuf += msg.Text
		}
	}
	return m, nil
}

// doImport merges a pasted config or a file path into the staged values.
func (m Model) doImport() Model {
	m.dlg = dlgNone
	src := m.dlgBuf
	if strings.Contains(src, "#share=") {
		text, err := DecodeShare(src)
		if err != nil {
			m.noteErr("import failed: " + err.Error())
			return m
		}
		src = text
	} else if t := strings.TrimSpace(src); t != "" && !strings.ContainsAny(t, "\n=") {
		b, err := os.ReadFile(expandHome(t))
		if err != nil {
			m.noteErr("import failed: " + err.Error())
			return m
		}
		src = string(b)
	}
	f := config.Parse(src)
	n, unknown, invalid := 0, 0, 0
	for _, key := range f.Keys() {
		st, ok := m.sch.ByKey(key)
		if !ok {
			unknown++
			continue
		}
		var vals []string
		for _, v := range f.GetAll(key) {
			if _, _, err := schema.Validate(st, v); err != nil {
				invalid++
				continue
			}
			vals = append(vals, v)
		}
		if len(vals) == 0 {
			continue
		}
		m.set(key, vals)
		n++
	}
	if n == 0 {
		m.noteErr("import: no Ghostty settings found")
		return m
	}
	msg := "imported " + plural(n, "setting") + " — review, then s to save"
	if unknown > 0 {
		msg += " (" + plural(unknown, "unknown key") + " skipped)"
	}
	if invalid > 0 {
		msg += " (" + plural(invalid, "invalid value") + " skipped)"
	}
	m.note(msg)
	return m
}

// ExportText renders the staged values as a minimal config grouped by
// category: only settings that differ from Ghostty's defaults.
func (m Model) ExportText() string {
	var sb strings.Builder
	for _, cat := range schema.Categories {
		first := true
		for _, st := range m.sch.InCategory(cat) {
			vals := m.vals[st.Key]
			if len(vals) == 0 {
				continue
			}
			if first {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString("# " + cat + "\n")
				first = false
			}
			for _, v := range vals {
				sb.WriteString(st.Key + " = " + v + "\n")
			}
		}
	}
	return sb.String()
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}

func trimLast(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func orNone(s string) string {
	if s == "" {
		return "unset"
	}
	return s
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	if strings.HasSuffix(word, "y") {
		return itoa(n) + " " + strings.TrimSuffix(word, "y") + "ies"
	}
	return itoa(n) + " " + word + "s"
}

// Run starts the Tea program.
func Run(opts Options) error {
	_, err := tea.NewProgram(New(opts)).Run()
	return err
}
