package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"ghostty-config-tui/internal/ghostty"
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
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "space":
		return tea.KeyPressMsg{Text: " ", Code: ' '}
	default:
		return tea.KeyPressMsg{Text: s, Code: []rune(s)[0]}
	}
}

func moveTo(m Model, idx int) Model {
	for m.Cursor() != idx {
		updated, _ := m.Update(keyPress("j"))
		m = updated.(Model)
	}
	return m
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

func TestFilterBackspaceWidens(t *testing.T) {
	m := typeFilter(t, New(testOptions()), "fontx")
	if len(m.Visible()) != 0 {
		t.Fatalf("fontx should match nothing, got %v", visibleKeys(m))
	}
	updated, _ := m.Update(keyPress("backspace"))
	m = updated.(Model)
	if got := visibleKeys(m); len(got) != 2 {
		t.Fatalf("after backspace visible = %v, want 2 font keys", got)
	}
}

func TestEditEnumCycleAndCommit(t *testing.T) {
	m := moveTo(New(testOptions()), 6) // cursor-style
	updated, _ := m.Update(keyPress("enter"))
	m = updated.(Model)
	updated, _ = m.Update(keyPress("tab"))
	m = updated.(Model)
	updated, _ = m.Update(keyPress("enter"))
	m = updated.(Model)
	if v := m.Values()["cursor-style"]; v != "bar" {
		t.Fatalf("cursor-style = %q, want bar", v)
	}
	if !m.Dirty() {
		t.Fatal("committed edit should mark model dirty")
	}
}

func TestEditBoolToggleCommit(t *testing.T) {
	m := moveTo(New(testOptions()), 7) // cursor-style-blink
	updated, _ := m.Update(keyPress("enter"))
	m = updated.(Model)
	updated, _ = m.Update(keyPress("space"))
	m = updated.(Model)
	updated, _ = m.Update(keyPress("enter"))
	m = updated.(Model)
	if v := m.Values()["cursor-style-blink"]; v != "true" {
		t.Fatalf("cursor-style-blink = %q, want true", v)
	}
}

func TestEditIntClampCommit(t *testing.T) {
	m := moveTo(New(testOptions()), 2) // font-size, current "12"
	updated, _ := m.Update(keyPress("enter"))
	m = updated.(Model)
	for _, ch := range []string{"backspace", "backspace"} {
		updated, _ = m.Update(keyPress(ch))
		m = updated.(Model)
	}
	for _, ch := range []string{"2", "0", "0"} {
		updated, _ = m.Update(keyPress(ch))
		m = updated.(Model)
	}
	updated, _ = m.Update(keyPress("enter"))
	m = updated.(Model)
	if v := m.Values()["font-size"]; v != "72" {
		t.Fatalf("font-size = %q, want clamped 72", v)
	}
}

func TestEditInvalidStaysEditing(t *testing.T) {
	m := moveTo(New(testOptions()), 2) // font-size
	updated, _ := m.Update(keyPress("enter"))
	m = updated.(Model)
	for _, ch := range []string{"backspace", "backspace", "a", "b", "c"} {
		updated, _ = m.Update(keyPress(ch))
		m = updated.(Model)
	}
	updated, _ = m.Update(keyPress("enter"))
	m = updated.(Model)
	if !m.Editing() {
		t.Fatal("invalid value should keep edit mode open")
	}
	updated, _ = m.Update(keyPress("esc"))
	m = updated.(Model)
	if m.Editing() || m.Values()["font-size"] != "12" {
		t.Fatalf("esc should cancel edit, editing=%v values=%v", m.Editing(), m.Values())
	}
}

func TestEditColorRejectsBad(t *testing.T) {
	m := moveTo(New(testOptions()), 3) // background
	updated, _ := m.Update(keyPress("enter"))
	m = updated.(Model)
	for range 7 {
		updated, _ = m.Update(keyPress("backspace"))
		m = updated.(Model)
	}
	for _, ch := range []string{"#", "z", "z", "z", "z", "z", "z"} {
		updated, _ = m.Update(keyPress(ch))
		m = updated.(Model)
	}
	updated, _ = m.Update(keyPress("enter"))
	m = updated.(Model)
	if !m.Editing() {
		t.Fatal("bad color should keep edit mode open")
	}
}

func TestEditThemeUnknownWarns(t *testing.T) {
	opts := testOptions()
	opts.ThemeOptions = []string{"Dracula"}
	m := New(opts) // cursor 0 = theme
	updated, _ := m.Update(keyPress("enter"))
	m = updated.(Model)
	for _, ch := range []string{"C", "u", "s", "t", "o", "m"} {
		updated, _ = m.Update(keyPress(ch))
		m = updated.(Model)
	}
	updated, _ = m.Update(keyPress("enter"))
	m = updated.(Model)
	if v := m.Values()["theme"]; v != "Custom" {
		t.Fatalf("theme = %q, want Custom", v)
	}
	if !strings.Contains(m.StatusBar(), "not in") {
		t.Fatalf("unknown theme should warn, status = %q", m.StatusBar())
	}
}

func TestPickerSelectsTheme(t *testing.T) {
	opts := testOptions()
	opts.ThemeOptions = []string{"Dracula", "Nord"}
	m := New(opts) // cursor 0 = theme
	updated, _ := m.Update(keyPress("enter"))
	m = updated.(Model)
	for _, ch := range []string{"n", "o", "r"} {
		updated, _ = m.Update(keyPress(ch))
		m = updated.(Model)
	}
	if !strings.Contains(m.View().Content, "Nord") {
		t.Fatalf("picker should list matches:\n%s", m.View().Content)
	}
	updated, _ = m.Update(keyPress("enter"))
	m = updated.(Model)
	if v := m.Values()["theme"]; v != "Nord" {
		t.Fatalf("theme = %q, want picked Nord", v)
	}
	if strings.Contains(m.StatusBar(), "not in") {
		t.Fatalf("picked option should not warn: %q", m.StatusBar())
	}
}

func TestPickerNavigatesOptions(t *testing.T) {
	opts := testOptions()
	opts.ThemeOptions = []string{"Dracula", "Nord", "Gruvbox"}
	m := New(opts)
	updated, _ := m.Update(keyPress("enter"))
	m = updated.(Model)
	updated, _ = m.Update(keyPress("down"))
	m = updated.(Model)
	updated, _ = m.Update(keyPress("enter"))
	m = updated.(Model)
	if v := m.Values()["theme"]; v != "Nord" {
		t.Fatalf("theme = %q, want navigated Nord", v)
	}
}

func TestPickerCustomWhenNoMatch(t *testing.T) {
	opts := testOptions()
	opts.ThemeOptions = []string{"Dracula"}
	m := New(opts)
	updated, _ := m.Update(keyPress("enter"))
	m = updated.(Model)
	for _, ch := range []string{"x", "y", "z"} {
		updated, _ = m.Update(keyPress(ch))
		m = updated.(Model)
	}
	updated, _ = m.Update(keyPress("enter"))
	m = updated.(Model)
	if v := m.Values()["theme"]; v != "xyz" {
		t.Fatalf("theme = %q, want custom xyz", v)
	}
	if !strings.Contains(m.StatusBar(), "not in") {
		t.Fatalf("custom value should warn: %q", m.StatusBar())
	}
}

func TestPickerEscCancels(t *testing.T) {
	opts := testOptions()
	opts.ThemeOptions = []string{"Dracula"}
	m := New(opts)
	updated, _ := m.Update(keyPress("enter"))
	m = updated.(Model)
	updated, _ = m.Update(keyPress("d"))
	m = updated.(Model)
	updated, _ = m.Update(keyPress("esc"))
	m = updated.(Model)
	if m.Editing() || m.Values()["theme"] != "" {
		t.Fatalf("esc should cancel picker, editing=%v values=%v", m.Editing(), m.Values())
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

func themeResolvedMsg() ThemeResolvedMsg {
	var pal [16]string
	pal[1] = "#ff5555"
	return ThemeResolvedMsg{
		Name: "Dracula",
		Colors: ghostty.ThemeColors{
			Background: "#282a36",
			Foreground: "#f8f8f2",
			Palette:    pal,
		},
		OK: true,
	}
}

func TestPreviewUsesThemeColors(t *testing.T) {
	opts := testOptions()
	opts.Values = map[string]string{"theme": "Dracula"}
	m := New(opts)
	updated, _ := m.Update(themeResolvedMsg())
	m = updated.(Model)
	bg, fg, _, pal := resolvePreviewColors(m)
	if bg != "#282a36" || fg != "#f8f8f2" {
		t.Fatalf("preview colors = %q %q, want theme colors", bg, fg)
	}
	if pal[1] != "#ff5555" {
		t.Fatalf("preview palette[1] = %q, want theme color", pal[1])
	}
}

func TestResetFieldAndAll(t *testing.T) {
	m := moveTo(New(testOptions()), 2) // font-size, orig "12"
	m.SetValue("font-size", "14")
	m.SetValue("theme", "Dracula")
	updated, _ := m.Update(keyPress("r"))
	m = updated.(Model)
	if v := m.Values()["font-size"]; v != "12" {
		t.Fatalf("r should reset field, font-size = %q", v)
	}
	if v := m.Values()["theme"]; v != "Dracula" {
		t.Fatalf("r should leave other edits, theme = %q", v)
	}
	updated, _ = m.Update(keyPress("R"))
	m = updated.(Model)
	if m.Dirty() {
		t.Fatalf("R should reset all, values = %v", m.Values())
	}
}

func TestViewUsesAltScreen(t *testing.T) {
	if !New(testOptions()).View().AltScreen {
		t.Fatal("view should use the alternate screen buffer")
	}
}

func TestPreviewExplicitBeatsTheme(t *testing.T) {
	opts := testOptions()
	opts.Values = map[string]string{"theme": "Dracula", "background": "#000000"}
	m := New(opts)
	updated, _ := m.Update(themeResolvedMsg())
	m = updated.(Model)
	bg, _, _, _ := resolvePreviewColors(m)
	if bg != "#000000" {
		t.Fatalf("explicit background should win, got %q", bg)
	}
}

func TestPreviewCursorColor(t *testing.T) {
	opts := testOptions()
	opts.Values = map[string]string{"theme": "Dracula"}
	m := New(opts)
	msg := themeResolvedMsg()
	msg.Colors.Cursor = "#ff0000"
	updated, _ := m.Update(msg)
	m = updated.(Model)
	_, _, cursor, _ := resolvePreviewColors(m)
	if cursor != "#ff0000" {
		t.Fatalf("cursor color = %q, want theme cursor", cursor)
	}
}

func TestThemeCommitDispatchesResolve(t *testing.T) {
	m := New(testOptions())
	updated, cmd := m.Update(keyPress("enter"))
	m = updated.(Model)
	updated, cmd = m.Update(keyPress("enter"))
	m = updated.(Model)
	_ = updated
	if cmd == nil {
		t.Fatal("theme commit without options should still dispatch a resolve")
	}
	m2 := moveTo(New(testOptions()), 2)
	updated, cmd = m2.Update(keyPress("enter"))
	m2 = updated.(Model)
	updated, cmd = m2.Update(keyPress("enter"))
	_ = updated
	if cmd != nil {
		t.Fatal("non-theme commit should not dispatch a resolve")
	}
}
