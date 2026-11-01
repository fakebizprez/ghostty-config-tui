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

func TestImportPasteWithCarriageReturns(t *testing.T) {
	m, _ := newModel(nil)
	m = press(m, "i")
	// Terminals deliver pasted newlines as \r.
	mm, _ := m.Update(tea.PasteMsg{Content: "cursor-style = underline\rfont-size = 15\r\nwindow-padding-x = 12\r"})
	m = press(mm.(Model), "enter")
	v := m.Values()
	if len(v["cursor-style"]) != 1 || v["cursor-style"][0] != "underline" || v["font-size"][0] != "15" || v["window-padding-x"][0] != "12" {
		t.Fatalf("imported %v", v)
	}
}

func TestImportSkipsInvalidValues(t *testing.T) {
	m, _ := newModel(nil)
	m = press(m, "i")
	mm, _ := m.Update(tea.PasteMsg{Content: "font-size = huge\ncursor-style = bar\n"})
	m = press(mm.(Model), "enter")
	if _, ok := m.Values()["font-size"]; ok {
		t.Fatal("invalid font-size must not be staged")
	}
	if !strings.Contains(m.Message(), "1 invalid value skipped") {
		t.Fatalf("message = %q", m.Message())
	}
}

func TestPluralEntries(t *testing.T) {
	if got := plural(2, "entry"); got != "2 entries" {
		t.Fatalf("plural = %q", got)
	}
}

func TestTypeInPreview(t *testing.T) {
	m, s := newModel(map[string][]string{"font-size": {"12"}})
	m = goTo(t, m, "font-size")
	m = press(m, "d", "t")
	m = typeText(m, "ls -la")
	if s := ansi.Strip(m.Render()); !strings.Contains(s, "❯ ls -la") {
		t.Fatalf("typed text missing from preview:\n%s", s)
	}
	m = press(m, "s", "q")
	if len(s.calls) != 0 || m.Dirty() == false {
		t.Fatal("keys typed into the preview must not trigger save/quit")
	}
	m = press(m, "enter")
	out := ansi.Strip(m.Render())
	if !strings.Contains(out, "❯ ls -lasq") || strings.Count(out, "❯ ls -lasq") != 1 {
		t.Fatalf("enter should move the command into the preview history:\n%s", out)
	}
	m = press(m, "esc")
	if m.typing {
		t.Fatal("esc should leave preview typing mode")
	}
}

func TestShareURLRoundTrip(t *testing.T) {
	m, _ := newModel(map[string][]string{"theme": {"Nord"}, "font-size": {"15"}})
	u, ok := m.ShareURL()
	if !ok || !strings.HasPrefix(u, "https://ghostty.zerebos.com/app/import-export#share=") {
		t.Fatalf("share url = %q ok=%v", u, ok)
	}
	text, err := DecodeShare(u)
	if err != nil || text != "font-size = 15\ntheme = Nord\n" {
		t.Fatalf("decoded %q err=%v", text, err)
	}
	// Importing a share link stages its values.
	fresh, _ := newModel(nil)
	fresh = press(fresh, "i")
	mm, _ := fresh.Update(tea.PasteMsg{Content: u})
	fresh = press(mm.(Model), "enter")
	if fresh.Values()["theme"][0] != "Nord" || fresh.Values()["font-size"][0] != "15" {
		t.Fatalf("imported share link: %v (%s)", fresh.Values(), fresh.Message())
	}
}

func TestShareURLTooLong(t *testing.T) {
	var binds []string
	for i := 0; i < 80; i++ {
		binds = append(binds, "ctrl+shift+f"+itoa(i)+"=goto_tab:"+itoa(i))
	}
	m, _ := newModel(map[string][]string{"keybind": binds})
	if _, ok := m.ShareURL(); ok {
		t.Fatal("share URL over 1800 chars should be refused like the web app does")
	}
	m = press(m, "x")
	if _, cmd := m.Update(key("u")); cmd != nil {
		t.Fatal("too-long share must not copy anything")
	}
}

func TestExportCopiesShareURL(t *testing.T) {
	m, _ := newModel(map[string][]string{"theme": {"Nord"}})
	m = press(m, "x")
	mm, cmd := m.Update(key("u"))
	if cmd == nil || !strings.Contains(mm.(Model).Message(), "share link") {
		t.Fatalf("u should copy the share link, msg=%q", mm.(Model).Message())
	}
}

func TestFontPlaygroundUsesStagedFonts(t *testing.T) {
	old := showFace
	defer func() { showFace = old }()
	var gotFamilies []string
	showFace = func(sample string, families []string) ([]string, bool) {
		gotFamilies = families
		return []string{"U+2192 « → » found in face “Iosevka”."}, true
	}
	m, _ := newModel(map[string][]string{"font-family": {"Iosevka", "Symbols Nerd Font"}})
	mm, cmd := m.Update(key("f"))
	m = mm.(Model)
	if cmd == nil {
		t.Fatal("f should start a face lookup")
	}
	mm, _ = m.Update(cmd())
	out := ansi.Strip(mm.(Model).Render())
	if !strings.Contains(out, "Font playground") || !strings.Contains(out, "-> => != <=") || !strings.Contains(out, "found in face “Iosevka”") {
		t.Fatalf("playground render:\n%s", out)
	}
	if strings.Join(gotFamilies, ",") != "Iosevka,Symbols Nerd Font" {
		t.Fatalf("show-face families = %v", gotFamilies)
	}
}

func TestAppIconPreview(t *testing.T) {
	m, _ := newModel(map[string][]string{"macos-icon": {"custom-style"}, "macos-icon-frame": {"beige"}, "macos-icon-ghost-color": {"#ff0000"}, "macos-icon-screen-color": {"#000080"}})
	ic := m.IconColors()
	if ic.Ghost != "#ff0000" || ic.Screen != "#000080" || ic.Frame != frameColors["beige"] {
		t.Fatalf("icon colors = %+v", ic)
	}
	m = goTo(t, m, "macos-icon-frame")
	if out := ansi.Strip(m.Render()); !strings.Contains(out, "App icon") {
		t.Fatalf("icon preview missing for macos-icon settings:\n%s", out)
	}
	m = goTo(t, m, "font-size")
	if out := ansi.Strip(m.Render()); strings.Contains(out, "App icon") {
		t.Fatal("icon preview should only show for macos-icon settings")
	}
}

func TestPlaygroundGroupsByFace(t *testing.T) {
	got := groupFaces([]string{
		"U+54 « T » found in face “Menlo”.",
		"U+68 « h » found in face “Menlo”.",
		"U+250C « ┌ » is handled by Ghostty's internal sprites.",
		"U+2500 « ─ » is handled by Ghostty's internal sprites.",
		"U+1F642 « 🙂 » found in face “Apple Color Emoji”.",
	})
	want := []string{
		"found in face “Menlo”: T h",
		"is handled by Ghostty's internal sprites: ┌ ─",
		"found in face “Apple Color Emoji”: 🙂",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("groupFaces =\n%s", strings.Join(got, "\n"))
	}
}
