// Package ui is the Bubble Tea fullscreen TUI: sidebar + preview + status bar.
package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"ghostty-config-tui/internal/schema"
)

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
		width:        80,
		height:       24,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

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
	return fmt.Sprintf("%s | %s%s | s save u undo r/R reset q quit%s", m.configPath, dirty, dry, msg)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyPressMsg:
		key := msg.String()
		text := msg.Text
		switch key {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "down", "j":
			if m.cursor < len(m.keys)-1 {
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
			if len(m.keys) > 0 {
				k := m.keys[m.cursor].Key
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
			if len(m.keys) > 0 {
				k := m.keys[m.cursor].Key
				m.message = k + " = " + m.values[k]
			}
			return m, nil
		case "esc":
			m.message = ""
			return m, nil
		default:
			_ = text
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
	for i, s := range m.keys {
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

func previewBlock(m Model) string {
	bg := firstNonEmpty(m.values["background"], schema.Defaults()["background"])
	fg := firstNonEmpty(m.values["foreground"], schema.Defaults()["foreground"])
	cursor := firstNonEmpty(m.values["cursor-style"], "block")
	_ = cursor
	box := lipgloss.NewStyle().
		Background(lipgloss.Color(bg)).
		Foreground(lipgloss.Color(fg)).
		Render("  AaBbCc 0123  ")
	return "preview: " + box
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
