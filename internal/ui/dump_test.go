package ui

import (
	"fmt"
	"os"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderStates(t *testing.T) {
	if os.Getenv("UI_DUMP") == "" {
		t.Skip()
	}
	m := sized(New(Options{ConfigPath: "cfg", Themes: []string{"Dracula", "Nord", "GruvboxDark"}, Values: map[string][]string{"keybind": {"ctrl+a=new_tab"}}}), 120, 34)
	show := func(label string) { fmt.Println("=====", label); fmt.Println(ansi.Strip(m.Render())) }
	m = goTo(t, m, "cursor-color")
	m = press(m, "enter")
	m = press(m, "down", "right", "right")
	show("color")
	m = press(m, "esc")
	m = goTo(t, m, "bell-features")
	m = press(m, "enter")
	show("flags")
	m = press(m, "esc")
	m = goTo(t, m, "keybind")
	m = press(m, "enter", "a")
	m = typeText(m, "ctrl+shift+t")
	m = press(m, "tab")
	m = typeText(m, "new_t")
	show("keybind")
	m = press(m, "esc", "esc")
	m = goTo(t, m, "theme")
	m = press(m, "enter", "down")
	show("theme")
	m = press(m, "esc", "x")
	show("export")
}
