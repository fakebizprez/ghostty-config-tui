package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func key(s string) tea.KeyPressMsg {
	codes := map[string]rune{"enter": tea.KeyEnter, "esc": tea.KeyEscape, "up": tea.KeyUp, "down": tea.KeyDown,
		"left": tea.KeyLeft, "right": tea.KeyRight, "tab": tea.KeyTab, "backspace": tea.KeyBackspace, "space": tea.KeySpace}
	if c, ok := codes[s]; ok {
		k := tea.KeyPressMsg{Code: c}
		if s == "space" {
			k.Text = " "
		}
		return k
	}
	switch s {
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "shift+right":
		return tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift}
	case "ctrl+u":
		return tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
	case "ctrl+r":
		return tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}
	case "ctrl+t":
		return tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl}
	case "ctrl+shift+t":
		return tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl | tea.ModShift}
	}
	r := []rune(s)
	return tea.KeyPressMsg{Code: r[0], Text: s}
}

func press(m Model, keys ...string) Model {
	for _, k := range keys {
		mm, _ := m.Update(key(k))
		m = mm.(Model)
	}
	return m
}

func typeText(m Model, s string) Model {
	for _, r := range s {
		mm, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = mm.(Model)
	}
	return m
}

func sized(m Model, w, h int) Model {
	mm, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return mm.(Model)
}

// goTo searches for key and selects its exact row.
func goTo(t *testing.T, m Model, k string) Model {
	t.Helper()
	m = press(m, "esc", "/")
	m = typeText(m, k)
	m = press(m, "enter")
	for i, s := range m.Rows() {
		if s.Key == k {
			m.row = i
			return m
		}
	}
	t.Fatalf("goTo %s: not found", k)
	return m
}
