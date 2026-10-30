package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func testOptions() Options {
	return Options{
		ConfigPath: "/tmp/test-config",
		Values:     map[string]string{"font-size": "12"},
		OnSave:     func(map[string]string) error { return nil },
		OnUndo:     func() (map[string]string, error) { return map[string]string{}, nil },
	}
}

func keyPress(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	default:
		return tea.KeyPressMsg{Text: s, Code: []rune(s)[0]}
	}
}

func TestNavigateDownUp(t *testing.T) {
	m := New(testOptions())
	if m.Cursor() != 0 {
		t.Fatalf("initial cursor = %d", m.Cursor())
	}
	updated, _ := m.Update(keyPress("j"))
	m = updated.(Model)
	if m.Cursor() != 1 {
		t.Fatalf("after j cursor = %d, want 1", m.Cursor())
	}
	updated, _ = m.Update(keyPress("k"))
	m = updated.(Model)
	if m.Cursor() != 0 {
		t.Fatalf("after k cursor = %d, want 0", m.Cursor())
	}
}

func TestSaveCallsOnSave(t *testing.T) {
	called := false
	opts := testOptions()
	opts.OnSave = func(map[string]string) error { called = true; return nil }
	m := New(opts)
	updated, _ := m.Update(keyPress("s"))
	m = updated.(Model)
	if !called {
		t.Fatal("s did not call OnSave")
	}
	if !strings.Contains(m.StatusBar(), "ctrl+shift+r") {
		t.Fatalf("status bar missing reload hint: %q", m.StatusBar())
	}
}

func TestUndoCallsOnUndo(t *testing.T) {
	called := false
	opts := testOptions()
	opts.OnUndo = func() (map[string]string, error) { called = true; return map[string]string{}, nil }
	m := New(opts)
	updated, _ := m.Update(keyPress("u"))
	m = updated.(Model)
	if !called {
		t.Fatal("u did not call OnUndo")
	}
}

func TestDirtyDot(t *testing.T) {
	m := New(testOptions())
	if m.Dirty() {
		t.Fatal("fresh model should not be dirty")
	}
	m.SetValue("font-size", "14")
	if !m.Dirty() {
		t.Fatal("edited model should be dirty")
	}
	view := m.View()
	if !strings.Contains(view.Content, "●") && !strings.Contains(view.Content, "•") {
		t.Fatalf("sidebar should show modified dot, got:\n%s", view.Content)
	}
}

func TestQuit(t *testing.T) {
	m := New(testOptions())
	_, cmd := m.Update(keyPress("q"))
	if cmd == nil {
		t.Fatal("q should return a command")
	}
}

func TestStatusBarPath(t *testing.T) {
	m := New(testOptions())
	if !strings.Contains(m.StatusBar(), "/tmp/test-config") {
		t.Fatalf("status bar = %q", m.StatusBar())
	}
}
