package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"ghostty-config-tui/internal/ghostty"
)

type saver struct {
	calls []map[string][]string
}

func (s *saver) save(c map[string][]string) ([]string, error) {
	s.calls = append(s.calls, c)
	return nil, nil
}

func newModel(vals map[string][]string) (Model, *saver) {
	s := &saver{}
	m := New(Options{
		ConfigPath: "/tmp/config",
		Values:     vals,
		Themes:     []string{"Dracula", "Nord", "Gruvbox Dark"},
		Fonts:      []string{"JetBrains Mono", "Iosevka"},
		OnSave:     s.save,
	})
	return sized(m, 120, 40), s
}

func TestCategoriesCoverEverySetting(t *testing.T) {
	m, _ := newModel(nil)
	seen := 0
	for i := 1; i < len(m.cats); i++ {
		m.cat = i
		seen += len(m.Rows())
	}
	if seen != len(m.sch.Settings) {
		t.Fatalf("categories list %d of %d settings", seen, len(m.sch.Settings))
	}
}

func TestBoolTogglesInPlace(t *testing.T) {
	m, _ := newModel(nil)
	m = goTo(t, m, "maximize")
	m = press(m, "space")
	if got := m.Values()["maximize"]; len(got) != 1 || got[0] != "true" {
		t.Fatalf("maximize = %v", got)
	}
	m = press(m, "enter")
	if _, ok := m.Values()["maximize"]; ok {
		t.Fatal("toggling back to the default should drop the key")
	}
}

func TestEnumPickerCommitsHighlighted(t *testing.T) {
	m, _ := newModel(nil)
	m = goTo(t, m, "cursor-style")
	m = press(m, "enter", "down", "enter")
	if got := m.Values()["cursor-style"]; len(got) != 1 || got[0] != "bar" {
		t.Fatalf("cursor-style = %v", got)
	}
	m = press(m, "enter")
	m = typeText(m, "hol")
	m = press(m, "enter")
	if got := m.Values()["cursor-style"][0]; got != "block_hollow" {
		t.Fatalf("filtered pick = %q", got)
	}
}

func TestNumberSliderClampsToRange(t *testing.T) {
	m, _ := newModel(map[string][]string{"background-opacity": {"0.9"}})
	m = goTo(t, m, "background-opacity")
	m = press(m, "enter", "right", "right", "right", "enter")
	if got := m.Values()["background-opacity"]; len(got) != 1 || got[0] != "1" {
		t.Fatalf("0.9 + 3 steps clamps to 1; the file had the key so it stays explicit, got %v", got)
	}
	m = press(m, "enter", "left", "left", "enter")
	if got := m.Values()["background-opacity"][0]; got != "0.9" {
		t.Fatalf("opacity = %q, want 0.9 (no float drift)", got)
	}
}

func TestInvalidNumberStaysOpenWithError(t *testing.T) {
	m, _ := newModel(nil)
	m = goTo(t, m, "font-size")
	m = press(m, "enter")
	m = press(m, "backspace", "backspace")
	m = typeText(m, "999")
	m = press(m, "enter")
	if !m.Editing() || !strings.Contains(m.ed.err, "1 to 255") {
		t.Fatalf("editing=%v err=%q", m.Editing(), m.ed.err)
	}
	m = press(m, "esc")
	if m.Editing() || m.Values()["font-size"] != nil {
		t.Fatal("esc should cancel without staging")
	}
}

func TestColorChannels(t *testing.T) {
	m, _ := newModel(map[string][]string{"background": {"#102030"}})
	m = goTo(t, m, "background")
	m = press(m, "enter", "right", "down", "shift+right", "enter")
	if got := m.Values()["background"][0]; got != "#113030" {
		t.Fatalf("background = %q, want #113030", got)
	}
}

func TestFlagsWriteOnlyDifferences(t *testing.T) {
	m, _ := newModel(nil)
	m = goTo(t, m, "bell-features")
	// Options: system audio attention title border. Turn audio on.
	m = press(m, "enter", "down", "space", "enter")
	if got := m.Values()["bell-features"]; len(got) != 1 || got[0] != "audio" {
		t.Fatalf("bell-features = %v", got)
	}
}

func TestListEditorAddMoveDelete(t *testing.T) {
	m, _ := newModel(map[string][]string{"font-feature": {"-calt"}})
	m = goTo(t, m, "font-feature")
	m = press(m, "enter", "a")
	m = typeText(m, "+ss01")
	m = press(m, "enter")
	if got := strings.Join(m.Values()["font-feature"], ","); got != "-calt,+ss01" {
		t.Fatalf("after add = %s", got)
	}
	m = press(m, "K")
	if got := strings.Join(m.Values()["font-feature"], ","); got != "+ss01,-calt" {
		t.Fatalf("after move = %s", got)
	}
	m = press(m, "x", "esc")
	if got := strings.Join(m.Values()["font-feature"], ","); got != "-calt" || m.Editing() {
		t.Fatalf("after delete = %s editing=%v", got, m.Editing())
	}
}

func TestFontListUsesFontPicker(t *testing.T) {
	m, _ := newModel(nil)
	m = goTo(t, m, "font-family")
	m = press(m, "enter", "a")
	m = typeText(m, "ios")
	m = press(m, "enter")
	if got := m.Values()["font-family"]; len(got) != 1 || got[0] != "Iosevka" {
		t.Fatalf("font-family = %v", got)
	}
}

func TestKeybindBuilderRecordsAndValidates(t *testing.T) {
	m, _ := newModel(nil)
	m = goTo(t, m, "keybind")
	m = press(m, "enter", "a", "ctrl+r", "ctrl+shift+t", "tab")
	m = typeText(m, "new_wind")
	m = press(m, "shift+tab", "shift+tab", "right", "space", "enter")
	got := m.Values()["keybind"]
	if len(got) != 1 || got[0] != "all:ctrl+shift+t=new_window" {
		t.Fatalf("keybind = %v", got)
	}
	// An invalid modifier keeps the builder open with an error.
	m = press(m, "a")
	m = typeText(m, "hyper+x")
	m = press(m, "tab")
	m = typeText(m, "new_tab")
	m = press(m, "enter")
	if m.ed.sub == nil || !strings.Contains(m.ed.sub.err, "unknown modifier") {
		t.Fatalf("expected modifier error, sub=%+v", m.ed.sub)
	}
}

func TestKeybindBuilderFlagsDefaultOverride(t *testing.T) {
	m, _ := newModel(nil)
	m = goTo(t, m, "keybind")
	m = press(m, "enter", "a")
	m = typeText(m, "super+t")
	m = press(m, "tab")
	m = typeText(m, "new_window")
	lines := ansi.Strip(strings.Join(m.keybindLines(m.ed.sub, 60), "\n"))
	if !strings.Contains(lines, "overrides default") {
		t.Fatalf("expected default-conflict warning:\n%s", lines)
	}
}

func TestSaveSendsOnlyChangedKeysAndRemovals(t *testing.T) {
	m, s := newModel(map[string][]string{"font-size": {"12"}, "theme": {"Nord"}})
	m = goTo(t, m, "font-size")
	m = press(m, "d")
	m = goTo(t, m, "maximize")
	m = press(m, "space", "s")
	if len(s.calls) != 1 {
		t.Fatalf("save calls = %d", len(s.calls))
	}
	c := s.calls[0]
	if len(c) != 2 || c["font-size"] != nil || c["maximize"][0] != "true" {
		t.Fatalf("changed = %v", c)
	}
	if _, ok := c["theme"]; ok {
		t.Fatal("unchanged keys must not be rewritten")
	}
	if m.Dirty() {
		t.Fatal("clean after save")
	}
	m = press(m, "s")
	if len(s.calls) != 1 || !strings.Contains(m.Message(), "nothing to save") {
		t.Fatal("second save should be a no-op")
	}
}

func TestRevertAndQuitGuard(t *testing.T) {
	m, _ := newModel(map[string][]string{"font-size": {"12"}})
	m = goTo(t, m, "font-size")
	m = press(m, "d", "r")
	if m.Dirty() {
		t.Fatal("r should revert to the file value")
	}
	m = press(m, "d")
	mm, cmd := m.Update(key("q"))
	m = mm.(Model)
	if cmd != nil || !strings.Contains(m.Message(), "unsaved") {
		t.Fatal("first q with unsaved changes should warn, not quit")
	}
	if _, cmd = m.Update(key("q")); cmd == nil {
		t.Fatal("second q should quit")
	}
}

func TestImportPasteStagesKnownKeys(t *testing.T) {
	m, _ := newModel(nil)
	m = press(m, "i")
	mm, _ := m.Update(tea.PasteMsg{Content: "# shared\ntheme = Nord\nkeybind = a=b\nkeybind = c=d\nnot-a-key = 1\n"})
	m = press(mm.(Model), "enter")
	v := m.Values()
	if v["theme"][0] != "Nord" || len(v["keybind"]) != 2 {
		t.Fatalf("imported %v", v)
	}
	if !strings.Contains(m.Message(), "1 unknown key skipped") {
		t.Fatalf("message = %q", m.Message())
	}
}

func TestImportFromFileAndExportToFile(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.conf")
	os.WriteFile(in, []byte("cursor-style = bar\n"), 0o644)
	m, _ := newModel(nil)
	m = press(m, "i")
	m = typeText(m, in)
	m = press(m, "enter")
	if m.Values()["cursor-style"][0] != "bar" {
		t.Fatalf("import file: %v (%s)", m.Values(), m.Message())
	}
	out := filepath.Join(dir, "out.conf")
	m = press(m, "x", "w", "ctrl+u")
	m.dlgBuf = out
	m = press(m, "enter")
	b, err := os.ReadFile(out)
	if err != nil || string(b) != "# Cursor\ncursor-style = bar\n" {
		t.Fatalf("exported %q err=%v", b, err)
	}
}

func TestExportCopiesToClipboard(t *testing.T) {
	m, _ := newModel(map[string][]string{"theme": {"Nord"}})
	m = press(m, "x")
	if _, cmd := m.Update(key("c")); cmd == nil {
		t.Fatal("c should return a clipboard command")
	}
}

func TestSearchMatchesDocs(t *testing.T) {
	m, _ := newModel(nil)
	m = press(m, "/")
	m = typeText(m, "hollow")
	rows := m.Rows()
	if len(rows) == 0 || rows[0].Key != "cursor-style" {
		t.Fatalf("search rows = %v", rows)
	}
	m = press(m, "esc")
	if len(m.Rows()) == len(rows) && m.search != "" {
		t.Fatal("esc should clear the search")
	}
}

func TestModifiedCategoryListsSetKeys(t *testing.T) {
	m, _ := newModel(map[string][]string{"theme": {"Nord"}, "keybind": {"a=b"}})
	m.cat = 0
	if got := len(m.Rows()); got != 2 {
		t.Fatalf("modified rows = %d", got)
	}
}

func TestThemePickerPreviewsHighlightedTheme(t *testing.T) {
	old := resolveTheme
	defer func() { resolveTheme = old }()
	resolveTheme = func(name string) (ghostty.ThemeColors, bool) {
		return ghostty.ThemeColors{Background: map[string]string{"Dracula": "#282a36", "Nord": "#2e3440"}[name]}, true
	}
	m, _ := newModel(nil)
	m = goTo(t, m, "theme")
	mm, cmd := m.Update(key("enter"))
	m = mm.(Model)
	m = drain(m, cmd)
	if got := m.Colors().BG; got != "#282a36" {
		t.Fatalf("highlighted Dracula bg = %s", got)
	}
	mm, cmd = m.Update(key("down"))
	m = drain(mm.(Model), cmd)
	if got := m.Colors().BG; got != "#2e3440" {
		t.Fatalf("highlighted Nord bg = %s", got)
	}
	m = press(m, "esc")
	if got := m.Colors().BG; got != "#282c34" {
		t.Fatalf("cancel should restore the default bg, got %s", got)
	}
}

func drain(m Model, cmd tea.Cmd) Model {
	if cmd == nil {
		return m
	}
	if msg, ok := cmd().(ThemeResolvedMsg); ok {
		mm, _ := m.Update(msg)
		return mm.(Model)
	}
	return m
}

func TestPreviewExplicitColorsBeatTheme(t *testing.T) {
	m, _ := newModel(map[string][]string{"theme": {"X"}, "background": {"#010203"}, "palette": {"1=#ff0000"}, "cursor-color": {"cell-foreground"}})
	m.themeColors["X"] = ghostty.ThemeColors{Background: "#999999", Foreground: "#eeeeee"}
	pc := m.Colors()
	if pc.BG != "#010203" || pc.FG != "#eeeeee" || pc.Palette[1] != "#ff0000" || pc.Cursor != "#eeeeee" {
		t.Fatalf("colors = %+v", pc)
	}
}

func TestRenderFitsTerminal(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {200, 60}} {
		m, _ := newModel(map[string][]string{"keybind": {"ctrl+a=new_tab"}})
		m = sized(m, size[0], size[1])
		for _, keys := range [][]string{nil, {"enter"}, {"?"}, {"x"}, {"i"}} {
			got := press(m, keys...).Render()
			lines := strings.Split(got, "\n")
			if len(lines) != size[1] {
				t.Fatalf("%v %v: %d lines, want %d", size, keys, len(lines), size[1])
			}
			for i, l := range lines {
				if w := lipgloss.Width(l); w > size[0] {
					t.Fatalf("%v %v line %d is %d wide: %q", size, keys, i, w, ansi.Strip(l))
				}
			}
		}
	}
}

func TestLineFeedActsAsEnter(t *testing.T) {
	m, _ := newModel(nil)
	m = goTo(t, m, "cursor-style")
	mm, _ := m.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})
	if !mm.(Model).Editing() {
		t.Fatal("ctrl+j (line feed) should open the editor like enter")
	}
}

func TestExplicitDefaultKeptWhenFileHadIt(t *testing.T) {
	m, _ := newModel(map[string][]string{"theme": {"Dracula"}, "background": {"#282c34"}})
	m = goTo(t, m, "background")
	m = press(m, "enter", "enter")
	if m.Dirty() || m.Values()["background"][0] != "#282c34" {
		t.Fatalf("re-applying a pinned default must not delete it: %v", m.Values()["background"])
	}
}

func TestPickerKeepsCustomValue(t *testing.T) {
	m, _ := newModel(map[string][]string{"window-title-font-family": {"Iosev"}, "background-blur": {"2"}})
	m = goTo(t, m, "window-title-font-family")
	m = press(m, "enter", "enter")
	m = goTo(t, m, "background-blur")
	m = press(m, "enter", "enter")
	if m.Dirty() {
		t.Fatalf("open+enter rewrote custom values: %v", m.changed())
	}
	// Typed text that is a substring of an option is reachable via the typed row.
	m = goTo(t, m, "window-title-font-family")
	m = press(m, "enter", "ctrl+u")
	m = typeText(m, "Ios")
	m = press(m, "up", "enter")
	if got := m.Values()["window-title-font-family"][0]; got != "Ios" {
		t.Fatalf("typed custom value = %q", got)
	}
}

func TestUndoWithUnsavedEditsAsksFirst(t *testing.T) {
	m, _ := newModel(nil)
	called := 0
	m.opts.OnUndo = func() (map[string][]string, error) { called++; return map[string][]string{}, nil }
	m = goTo(t, m, "maximize")
	m = press(m, "space", "u")
	if called != 0 || !m.Dirty() {
		t.Fatal("first u with unsaved edits must not undo")
	}
	m = press(m, "u")
	if called != 1 {
		t.Fatal("second u should undo")
	}
}

func TestRecorderCapturesCtrlJ(t *testing.T) {
	m, _ := newModel(nil)
	m = goTo(t, m, "keybind")
	m = press(m, "enter", "a", "ctrl+r")
	mm, _ := m.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})
	m = mm.(Model)
	if got := m.ed.sub.kbTrig; got != "ctrl+j" {
		t.Fatalf("recorded %q, want ctrl+j", got)
	}
}
