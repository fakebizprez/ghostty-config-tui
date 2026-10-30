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

func typeFilter(t *testing.T, m Model, s string) Model {
	t.Helper()
	updated, _ := m.Update(keyPress("/"))
	m = updated.(Model)
	for _, r := range s {
		updated, _ = m.Update(keyPress(string(r)))
		m = updated.(Model)
	}
	return m
}

func visibleKeys(m Model) []string {
	names := []string{}
	for _, s := range m.Visible() {
		names = append(names, s.Key)
	}
	return names
}

func TestFilterNarrowsSidebar(t *testing.T) {
	m := typeFilter(t, New(testOptions()), "font")
	got := visibleKeys(m)
	if len(got) != 2 || got[0] != "font-family" || got[1] != "font-size" {
		t.Fatalf("filter font visible = %v, want [font-family font-size]", got)
	}
}

func TestFilterEscClears(t *testing.T) {
	m := typeFilter(t, New(testOptions()), "font")
	updated, _ := m.Update(keyPress("esc"))
	m = updated.(Model)
	if m.Filter() != "" || len(m.Visible()) != 8 {
		t.Fatalf("esc should clear filter, got %q with %d visible", m.Filter(), len(m.Visible()))
	}
}

func TestFilterEnterKeeps(t *testing.T) {
	m := typeFilter(t, New(testOptions()), "cursor")
	updated, _ := m.Update(keyPress("enter"))
	m = updated.(Model)
	if m.Filter() == "" || len(m.Visible()) != 2 {
		t.Fatalf("enter should keep filter, got %q with %d visible", m.Filter(), len(m.Visible()))
	}
}

func TestPreviewCursorGlyphs(t *testing.T) {
	m := New(testOptions())
	if !strings.Contains(previewBlock(m), "█") {
		t.Fatalf("default block cursor missing:\n%s", previewBlock(m))
	}
	m.SetValue("cursor-style", "bar")
	if !strings.Contains(previewBlock(m), "▏") {
		t.Fatalf("bar cursor missing:\n%s", previewBlock(m))
	}
	m.SetValue("cursor-style", "underline")
	if !strings.Contains(previewBlock(m), "▁") {
		t.Fatalf("underline cursor missing:\n%s", previewBlock(m))
	}
}

func TestPreviewShowsPalette(t *testing.T) {
	m := New(testOptions())
	if !strings.Contains(previewBlock(m), "palette") {
		t.Fatalf("palette row missing:\n%s", previewBlock(m))
	}
}

func TestBlinkTickFlipsCursor(t *testing.T) {
	m := New(testOptions())
	m.SetValue("cursor-style", "block")
	m.SetValue("cursor-style-blink", "true")
	if !strings.Contains(previewBlock(m), "█") {
		t.Fatalf("cursor should start visible:\n%s", previewBlock(m))
	}
	updated, _ := m.Update(BlinkMsg{})
	m = updated.(Model)
	if strings.Contains(previewBlock(m), "█") {
		t.Fatalf("cursor should hide on blink phase:\n%s", previewBlock(m))
	}
	updated, _ = m.Update(BlinkMsg{})
	m = updated.(Model)
	if !strings.Contains(previewBlock(m), "█") {
		t.Fatalf("cursor should return on next phase:\n%s", previewBlock(m))
	}
}
