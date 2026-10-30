// Package ui is the Bubble Tea fullscreen TUI: sidebar + preview + status bar.
package ui

import (
	"fmt"
	"sort"
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
			if vis := m.visible(); len(vis) > 0 {
				k := vis[m.cursor].Key
				m.message = k + " = " + m.values[k]
			}
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
