// Package ui is the Bubble Tea fullscreen TUI: sidebar + preview + status bar.
package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"ghostty-config-tui/internal/schema"
)

// BlinkMsg flips the cursor blink phase on every tick.
type BlinkMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(530*time.Millisecond, func(time.Time) tea.Msg {
		return BlinkMsg{}
	})
}

// Options wires the model to storage and picker data.
type Options struct {
	ConfigPath   string
	Values       map[string]string
	ThemeOptions []string
	FontOptions  []string
	OnSave       func(values map[string]string) error
	OnUndo       func() (map[string]string, error)
	DryRun       bool
}

// Model is the Tea Elm model.
type Model struct {
	keys         []schema.Setting
	cursor       int
	values       map[string]string
	orig         map[string]string
	configPath   string
	themeOptions []string
	fontOptions  []string
	onSave       func(map[string]string) error
	onUndo       func() (map[string]string, error)
	dryRun       bool
	filter       string
	filtering    bool
	editing      bool
	editKey      string
	editBuf      string
	pickIdx      int
	blinkPhase   bool
	message      string
	quitting     bool
	width        int
	height       int
}

// New builds a Model from Options. Every setting comes from internal/schema.
func New(opts Options) Model {
	keys := schema.All()
	values := map[string]string{}
	for k, v := range opts.Values {
		values[k] = v
	}
	orig := map[string]string{}
	for k, v := range values {
		orig[k] = v
	}
	// Fill defaults for display without marking dirty: orig tracks file state.
	return Model{
		keys:         keys,
		values:       values,
		orig:         orig,
		configPath:   opts.ConfigPath,
		themeOptions: opts.ThemeOptions,
		fontOptions:  opts.FontOptions,
		onSave:       opts.OnSave,
		onUndo:       opts.OnUndo,
		dryRun:       opts.DryRun,
		blinkPhase:   true,
		width:        80,
		height:       24,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return tickCmd() }

// visible returns the sidebar settings matching the current filter.
func (m Model) visible() []schema.Setting {
	if m.filter == "" {
		return m.keys
	}
	return schema.Search(m.filter)
}

// Visible returns the sidebar settings matching the current filter.
func (m Model) Visible() []schema.Setting { return m.visible() }

// Filter returns the current sidebar search text.
func (m Model) Filter() string { return m.filter }

func (m *Model) clampCursor() {
	if vis := m.visible(); len(vis) == 0 {
		m.cursor = 0
	} else if m.cursor >= len(vis) {
		m.cursor = len(vis) - 1
	}
}

// Cursor returns the sidebar cursor.
func (m Model) Cursor() int { return m.cursor }

// Values returns a copy of the current managed values.
func (m Model) Values() map[string]string {
	out := map[string]string{}
	for k, v := range m.values {
		out[k] = v
	}
	return out
}

// Editing reports whether a detail edit is in progress.
func (m Model) Editing() bool { return m.editing }

// editSetting returns the schema entry for the row being edited.
func (m Model) editSetting() schema.Setting {
	s, _ := schema.ByKey(m.editKey)
	return s
}

// effectiveValue returns the current value or the schema default for display.
func (m Model) effectiveValue(key string) string {
	if v := m.values[key]; v != "" {
		return v
	}
	if s, ok := schema.ByKey(key); ok {
		return s.Default
	}
	return ""
}

// startEdit opens edit mode for the row under the cursor.
func (m *Model) startEdit() {
	vis := m.visible()
	if len(vis) == 0 {
		return
	}
	key := vis[m.cursor].Key
	m.editing = true
	m.editKey = key
	m.editBuf = m.effectiveValue(key)
	m.pickIdx = 0
	m.message = "editing " + key + " (enter commits, esc cancels)"
}

// picking reports whether the current edit shows an option picker.
func (m Model) picking() bool {
	if !m.editing {
		return false
	}
	switch m.editKey {
	case "theme":
		return len(m.themeOptions) > 0
	case "font-family":
		return len(m.fontOptions) > 0
	}
	return false
}

func (m Model) pickerOptions() []string {
	switch m.editKey {
	case "theme":
		return m.themeOptions
	case "font-family":
		return m.fontOptions
	}
	return nil
}

// matches returns picker options containing the buffer text.
func (m Model) matches() []string {
	opts := m.pickerOptions()
	if m.editBuf == "" {
		return opts
	}
	q := strings.ToLower(m.editBuf)
	var out []string
	for _, o := range opts {
		if strings.Contains(strings.ToLower(o), q) {
			out = append(out, o)
		}
	}
	return out
}

// pickEnter commits the highlighted match, or the custom buffer on no match.
func (m *Model) pickEnter() {
	ms := m.matches()
	if len(ms) == 0 {
		m.commitEdit()
		return
	}
	at := m.pickIdx
	if at < 0 || at >= len(ms) {
		at = 0
	}
	m.finishEdit(ms[at], "")
}

// cycleEnum moves the edit buffer through the setting's options.
func (m *Model) cycleEnum(forward bool) {
	s := m.editSetting()
	if len(s.Options) == 0 {
		return
	}
	at := 0
	for i, o := range s.Options {
		if strings.EqualFold(o, m.editBuf) {
			at = i
			break
		}
	}
	if forward {
		at = (at + 1) % len(s.Options)
	} else {
		at = (at - 1 + len(s.Options)) % len(s.Options)
	}
	m.editBuf = s.Options[at]
}

// toggleBool flips a true/false edit buffer.
func (m *Model) toggleBool() {
	on, _ := schema.ParseBool(m.editBuf)
	if on {
		m.editBuf = "false"
	} else {
		m.editBuf = "true"
	}
}

// commitEdit validates the buffer and stores it, or keeps edit mode open.
func (m *Model) commitEdit() {
	s := m.editSetting()
	v := strings.TrimSpace(m.editBuf)
	switch s.Type {
	case "int":
		n, err := strconv.Atoi(v)
		if err != nil {
			m.message = "invalid " + s.Key + ": want a number (esc cancels)"
			return
		}
		m.finishEdit(strconv.Itoa(schema.ClampFontSize(n)), "")
	case "float":
		f, ok := schema.ParseFloat(v)
		if !ok {
			m.message = "invalid " + s.Key + ": want a number (esc cancels)"
			return
		}
		m.finishEdit(strconv.FormatFloat(schema.ClampOpacity(f), 'f', -1, 64), "")
	case "color":
		c := schema.NormalizeColor(v)
		if !schema.ValidateColor(c) {
			m.message = "invalid " + s.Key + ": want #RRGGBB (esc cancels)"
			return
		}
		m.finishEdit(c, "")
	case "enum":
		matched := ""
		for _, o := range s.Options {
			if strings.EqualFold(o, v) {
				matched = o
			}
		}
		if matched == "" {
			m.message = "invalid " + s.Key + ": want one of " + strings.Join(s.Options, "/") + " (esc cancels)"
			return
		}
		m.finishEdit(matched, "")
	case "bool":
		on, ok := schema.ParseBool(v)
		if !ok {
			m.message = "invalid " + s.Key + ": want true/false (esc cancels)"
			return
		}
		if on {
			m.finishEdit("true", "")
		} else {
			m.finishEdit("false", "")
		}
	default:
		m.finishEdit(v, s.Key)
	}
}

// finishEdit stores a validated value, warning on unknown theme/font names.
func (m *Model) finishEdit(stored, warnedKey string) {
	var options []string
	switch warnedKey {
	case "theme":
		options = m.themeOptions
	case "font-family":
		options = m.fontOptions
	}
	m.SetValue(m.editKey, stored)
	m.editing = false
	m.message = m.editKey + " = " + stored
	if len(options) > 0 && warnedKey != "" {
		known := false
		for _, o := range options {
			if o == stored {
				known = true
			}
		}
		if !known {
			m.message += " (warning: not in list — used as-is)"
		}
	}
}

// Dirty reports unsaved changes.
func (m Model) Dirty() bool {
	if len(m.values) != len(m.orig) {
		return true
	}
	for k, v := range m.values {
		if m.orig[k] != v {
			return true
		}
	}
	return false
}

// SetValue edits a managed key.
func (m *Model) SetValue(key, value string) {
	if m.values == nil {
		m.values = map[string]string{}
	}
	m.values[key] = value
}

// StatusBar renders path, dirty state, save/undo notes, and key hints.
func (m Model) StatusBar() string {
	dirty := "clean"
	if m.Dirty() {
		dirty = "dirty ●"
	}
	dry := ""
	if m.dryRun {
		dry = " [dry-run]"
	}
	msg := ""
	if m.message != "" {
		msg = " — " + m.message
	}
	return fmt.Sprintf("%s | %s%s | s save u undo r/R reset / filter q quit%s", m.configPath, dirty, dry, msg)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case BlinkMsg:
		m.blinkPhase = !m.blinkPhase
		return m, tickCmd()
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyPressMsg:
		key := msg.String()
		if m.editing && m.picking() {
			switch key {
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "esc":
				m.editing = false
				m.editBuf = ""
				m.message = ""
				return m, nil
			case "enter":
				m.pickEnter()
				return m, nil
			case "backspace":
				if len(m.editBuf) > 0 {
					m.editBuf = m.editBuf[:len(m.editBuf)-1]
				}
				m.pickIdx = 0
				return m, nil
			case "up":
				if m.pickIdx > 0 {
					m.pickIdx--
				}
				return m, nil
			case "down":
				if m.pickIdx < len(m.matches())-1 {
					m.pickIdx++
				}
				return m, nil
			case "tab", "right":
				if ms := m.matches(); len(ms) > 0 {
					m.pickIdx = (m.pickIdx + 1) % len(ms)
				}
				return m, nil
			case "left":
				if ms := m.matches(); len(ms) > 0 {
					m.pickIdx = (m.pickIdx - 1 + len(ms)) % len(ms)
				}
				return m, nil
			default:
				t := msg.Text
				if t == "" {
					if len(key) != 1 {
						return m, nil
					}
					t = key
				}
				if len(t) != 1 {
					return m, nil
				}
				m.editBuf += t
				m.pickIdx = 0
				return m, nil
			}
		}
		if m.editing {
			switch key {
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "esc":
				m.editing = false
				m.editBuf = ""
				m.message = ""
				return m, nil
			case "enter":
				m.commitEdit()
				return m, nil
			case "backspace":
				if len(m.editBuf) > 0 {
					m.editBuf = m.editBuf[:len(m.editBuf)-1]
				}
				return m, nil
			case "tab", "right":
				if typ := m.editSetting().Type; typ == "enum" {
					m.cycleEnum(true)
				} else if typ == "bool" {
					m.toggleBool()
				}
				return m, nil
			case "left":
				if typ := m.editSetting().Type; typ == "enum" {
					m.cycleEnum(false)
				} else if typ == "bool" {
					m.toggleBool()
				}
				return m, nil
			case "up", "down", " ", "space":
				if m.editSetting().Type == "bool" {
					m.toggleBool()
				}
				return m, nil
			default:
				t := msg.Text
				if t == "" {
					if len(key) != 1 {
						return m, nil
					}
					t = key
				}
				if len(t) != 1 {
					return m, nil
				}
				m.editBuf += t
				return m, nil
			}
		}
		if m.filtering {
			switch key {
			case "esc":
				m.filtering = false
				m.filter = ""
				m.clampCursor()
				return m, nil
			case "enter":
				m.filtering = false
				m.clampCursor()
				return m, nil
			case "backspace":
				if len(m.filter) > 0 {
					m.filter = m.filter[:len(m.filter)-1]
				}
				m.clampCursor()
				return m, nil
			default:
				t := msg.Text
				if t == "" {
					if len(key) != 1 {
						return m, nil
					}
					t = key
				}
				m.filter += t
				m.clampCursor()
				return m, nil
			}
		}
		switch key {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "/":
			m.filtering = true
			return m, nil
		case "down", "j":
			if m.cursor < len(m.visible())-1 {
				m.cursor++
			}
			return m, nil
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "s":
			if m.onSave == nil {
				m.message = "no save handler"
				return m, nil
			}
			cp := map[string]string{}
			for k, v := range m.values {
				cp[k] = v
			}
			if err := m.onSave(cp); err != nil {
				m.message = "save error: " + err.Error()
				return m, nil
			}
			m.orig = cp
			if m.dryRun {
				m.message = "dry-run: would write file"
			} else {
				m.message = "saved — reload Ghostty with ctrl+shift+r"
			}
			return m, nil
		case "u":
			if m.onUndo == nil {
				m.message = "no undo handler"
				return m, nil
			}
			restored, err := m.onUndo()
			if err != nil {
				m.message = "undo error: " + err.Error()
				return m, nil
			}
			m.values = restored
			m.orig = restored
			m.message = "undo restored"
			return m, nil
		case "r":
			if vis := m.visible(); len(vis) > 0 {
				k := vis[m.cursor].Key
				if v, ok := m.orig[k]; ok {
					m.values[k] = v
				} else {
					delete(m.values, k)
				}
				m.message = "reset " + k
			}
			return m, nil
		case "R":
			m.values = map[string]string{}
			for k, v := range m.orig {
				m.values[k] = v
			}
			m.message = "reset all"
			return m, nil
		case "enter":
			m.startEdit()
			return m, nil
		case "esc":
			m.message = ""
			return m, nil
		default:
			return m, nil
		}
	}
	return m, nil
}

// View implements tea.Model with the alternate screen buffer.
func (m Model) View() tea.View {
	var sb strings.Builder
	title := lipgloss.NewStyle().Bold(true).Render("ghostty-config-tui")
	fmt.Fprintf(&sb, "%s\n\n", title)
	// Sidebar.
	names := make([]string, 0, len(m.keys))
	_ = names
	_ = sort.StringSlice{}
	if m.filtering {
		fmt.Fprintf(&sb, "/%s\n", m.filter)
	} else if m.filter != "" {
		fmt.Fprintf(&sb, "filter: %s (esc clears)\n", m.filter)
	}
	for i, s := range m.visible() {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		dot := " "
		if m.orig[s.Key] != m.values[s.Key] {
			dot = "●"
		}
		val := m.values[s.Key]
		if val == "" {
			val = s.Default
		}
		if m.editing && s.Key == m.editKey {
			fmt.Fprintf(&sb, "%s%s %s = %s\n", cursor, dot, s.Key, m.editBuf+"▌")
			if m.picking() {
				ms := m.matches()
				shown := ms
				if len(shown) > 8 {
					shown = shown[:8]
				}
				for i, o := range shown {
					mark := "  "
					if i == m.pickIdx {
						mark = "> "
					}
					fmt.Fprintf(&sb, "    %s%s\n", mark, o)
				}
				if len(ms) == 0 {
					fmt.Fprintf(&sb, "    (no match — enter uses custom value)\n")
				}
			}
			continue
		}
		fmt.Fprintf(&sb, "%s%s %s = %s\n", cursor, dot, s.Key, val)
	}
	sb.WriteString("\n")
	sb.WriteString(previewBlock(m))
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Faint(true).Render(m.StatusBar()))
	v := tea.NewView(sb.String())
	v.AltScreen = true
	return v
}

// defaultPalette is the fallback 16-color swatch row for the preview.
var defaultPalette = []string{
	"#000000", "#800000", "#008000", "#808000",
	"#000080", "#800080", "#008080", "#c0c0c0",
	"#808080", "#ff0000", "#00ff00", "#ffff00",
	"#0000ff", "#ff00ff", "#00ffff", "#ffffff",
}

func cursorGlyph(style string) string {
	switch style {
	case "bar":
		return "▏"
	case "underline":
		return "▁"
	default:
		return "█"
	}
}

func blinkEnabled(m Model) bool {
	v := firstNonEmpty(m.values["cursor-style-blink"], schema.Defaults()["cursor-style-blink"])
	on, _ := schema.ParseBool(v)
	return on
}

func previewBlock(m Model) string {
	bg := firstNonEmpty(m.values["background"], schema.Defaults()["background"])
	if !schema.ValidateColor(bg) {
		bg = schema.Defaults()["background"]
	}
	fg := firstNonEmpty(m.values["foreground"], schema.Defaults()["foreground"])
	if !schema.ValidateColor(fg) {
		fg = schema.Defaults()["foreground"]
	}
	box := lipgloss.NewStyle().
		Background(lipgloss.Color(bg)).
		Foreground(lipgloss.Color(fg)).
		Render("  AaBbCc 0123  ")
	var pal strings.Builder
	for _, c := range defaultPalette {
		pal.WriteString(lipgloss.NewStyle().Background(lipgloss.Color(c)).Render("  "))
	}
	glyph := cursorGlyph(firstNonEmpty(m.values["cursor-style"], "block"))
	blink := "off"
	if blinkEnabled(m) {
		blink = "on"
		if !m.blinkPhase {
			glyph = " "
		}
	}
	return "preview: " + box + "\n" + "palette: " + pal.String() + "\n" + "cursor: " + glyph + " (blink " + blink + ")"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// Run starts the Tea program.
func Run(opts Options) error {
	p := tea.NewProgram(New(opts))
	_, err := p.Run()
	return err
}
