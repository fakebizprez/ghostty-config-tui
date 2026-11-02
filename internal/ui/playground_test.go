package ui

import (
	"encoding/base64"
	"errors"
	"image"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"ghostty-config-tui/internal/ghostty"
)

// stubFonts replaces the font lookups for one test.
func stubFonts(t *testing.T, face func(string, []string) ([]string, error), render func(string, []string, string, string) (image.Image, string, error)) {
	t.Helper()
	oldFace, oldRender := showFace, renderFont
	t.Cleanup(func() { showFace, renderFont = oldFace, oldRender })
	showFace, renderFont = face, render
}

// runBatch delivers every message of a batched command to m.
func runBatch(m Model, cmd tea.Cmd) Model {
	msg := cmd()
	msgs := []tea.Msg{msg}
	if b, ok := msg.(tea.BatchMsg); ok {
		msgs = nil
		for _, c := range b {
			if c != nil {
				msgs = append(msgs, c())
			}
		}
	}
	for _, msg := range msgs {
		mm, _ := m.Update(msg)
		m = mm.(Model)
	}
	return m
}

// Ghostty strips quotes when it reads the file but not on the command line,
// and +show-face must see only the staged font settings, not other config.
func TestPlaygroundPassesUnquotedFontSettings(t *testing.T) {
	var got []string
	stubFonts(t, func(_ string, s []string) ([]string, error) { got = s; return nil, nil }, nil)
	m, _ := newModel(map[string][]string{
		"font-family":        {`""`, `"Menlo"`},
		"font-codepoint-map": {`U+E000-U+E0FF=Symbols Nerd Font`},
		"theme":              {"Nord"},
	})
	mm, cmd := m.Update(key("f"))
	runBatch(mm.(Model), cmd)
	joined := strings.Join(got, "|")
	if !strings.Contains(joined, "font-family=|font-family=Menlo") || !strings.Contains(joined, "font-codepoint-map=U+E000-U+E0FF=Symbols Nerd Font") {
		t.Fatalf("show-face settings = %q", got)
	}
	if strings.Contains(joined, "theme") || strings.Contains(joined, `"`) {
		t.Fatalf("show-face settings should be unquoted font keys only: %q", got)
	}
}

func TestPlaygroundRendersFirstUnquotedFamily(t *testing.T) {
	var family string
	stubFonts(t, func(string, []string) ([]string, error) { return nil, nil },
		func(f string, _ []string, _, _ string) (image.Image, string, error) {
			family = f
			return nil, "", errors.New("skip")
		})
	m, _ := newModel(map[string][]string{"font-family": {`""`, `"Menlo"`}})
	m.opts.Graphics = true
	mm, cmd := m.Update(key("f"))
	runBatch(mm.(Model), cmd)
	if family != "Menlo" {
		t.Fatalf("rendered family %q, want Menlo", family)
	}
}

func TestPlaygroundDropsStaleResults(t *testing.T) {
	stubFonts(t, func(_ string, s []string) ([]string, error) {
		return []string{"U+61 « a » found in face “" + strings.TrimPrefix(s[0], "font-family=") + "”."}, nil
	}, func(f string, _ []string, _, _ string) (image.Image, string, error) {
		return image.NewRGBA(image.Rect(0, 0, 400, 100)), "/fonts/" + f + ".ttf", nil
	})
	m, _ := newModel(map[string][]string{"font-family": {"Old"}})
	m.opts.Graphics = true
	mm, oldCmd := m.Update(key("f"))
	m = press(mm.(Model), "esc")
	m.vals["font-family"] = []string{"New"}
	mm, newCmd := m.Update(key("f"))
	m = runBatch(mm.(Model), newCmd)
	m = runBatch(m, oldCmd) // the first request answers last
	out := ansi.Strip(m.Render())
	if strings.Contains(out, "Old") || !strings.Contains(out, "New.ttf") || !strings.Contains(out, "face “New”") {
		t.Fatalf("stale results replaced the current ones:\n%s", out)
	}
}

func TestPlaygroundShowFaceErrors(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{ghostty.ErrNotInstalled, "install"},
		{errors.New("ghostty +show-face: unknown action"), "unknown action"},
	} {
		stubFonts(t, func(string, []string) ([]string, error) { return nil, tc.err }, nil)
		m, _ := newModel(nil)
		mm, cmd := m.Update(key("f"))
		out := ansi.Strip(runBatch(mm.(Model), cmd).Render())
		if !strings.Contains(out, tc.want) {
			t.Fatalf("%v: want %q in:\n%s", tc.err, tc.want, out)
		}
		if tc.want != "install" && strings.Contains(out, "install it") {
			t.Fatalf("a failing command must not say ghostty is missing:\n%s", out)
		}
	}
}

// On 80x24 the rendered sample must leave room for the face list, and the
// rest must be reachable by scrolling instead of closing the dialog.
func TestPlaygroundFitsSmallTerminal(t *testing.T) {
	faces := []string{}
	for _, c := range "abcdefghijkl" {
		faces = append(faces, "U+00 « "+string(c)+" » found in face “F"+string(c)+"”.")
	}
	stubFonts(t, func(string, []string) ([]string, error) { return faces, nil },
		func(string, []string, string, string) (image.Image, string, error) {
			return image.NewRGBA(image.Rect(0, 0, 564, 280)), "/fonts/x.ttf", nil
		})
	m, _ := newModel(nil)
	m = sized(m, 80, 24)
	m.opts.Graphics = true
	mm, cmd := m.Update(key("f"))
	m = runBatch(mm.(Model), cmd)
	if out := ansi.Strip(m.Render()); !strings.Contains(out, "Which face Ghostty picks") {
		t.Fatalf("face list heading hidden at 80x24:\n%s", out)
	}
	for range 20 {
		m = press(m, "j")
	}
	if m.dlg != dlgFonts {
		t.Fatal("j should scroll the playground, not close it")
	}
	if out := ansi.Strip(m.Render()); !strings.Contains(out, "Fl") || !strings.Contains(out, "esc close") {
		t.Fatalf("scrolling should reach the end:\n%s", out)
	}
	if m = press(m, "esc"); m.dlg != dlgNone {
		t.Fatal("esc should close the playground")
	}
}

// Ghostty accepts X11 names, and a gradient for the screen color.
func TestIconColorsAcceptNamesAndGradients(t *testing.T) {
	m, _ := newModel(map[string][]string{"macos-icon": {"custom-style"}, "macos-icon-ghost-color": {"red"}, "macos-icon-screen-color": {"#000080,#ff0000"}})
	ic := m.IconColors()
	if ic.Ghost != "#ff0000" || ic.Screen != "#000080" {
		t.Fatalf("icon colors = %+v", ic)
	}
}

func TestShareImportWarnsWhenLinkLooksCut(t *testing.T) {
	link := ShareBase + "#share=" + base64.RawURLEncoding.EncodeToString([]byte("theme = Nord\nfont-size = 1"))
	m, _ := newModel(nil)
	m = press(m, "i")
	mm, _ := m.Update(tea.PasteMsg{Content: link})
	m = press(mm.(Model), "enter")
	if !strings.Contains(m.Message(), "cut off") {
		t.Fatalf("truncated share link imported silently: %q", m.Message())
	}
}
