package ui

import (
	"os"
	"testing"
)

func TestLiveThemeColors(t *testing.T) {
	if os.Getenv("UI_LIVE") == "" {
		t.Skip()
	}
	m, _ := newModel(map[string][]string{"theme": {"Nord"}})
	m = drain(m, resolveThemeCmd("Nord"))
	t.Logf("%+v", m.Colors())
}
