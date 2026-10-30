package schema

import (
	"strings"
	"testing"
)

var v1Keys = []string{
	"theme",
	"font-family",
	"font-size",
	"background",
	"foreground",
	"background-opacity",
	"cursor-style",
	"cursor-style-blink",
}

func TestEveryV1KeyHasTypeDefaultAndHelp(t *testing.T) {
	for _, k := range v1Keys {
		s, ok := ByKey(k)
		if !ok {
			t.Fatalf("missing schema for key %q", k)
		}
		if s.Key == "" || s.Title == "" {
			t.Errorf("key %q missing Key/Title", k)
		}
		if s.Type == "" {
			t.Errorf("key %q missing Type", k)
		}
		if s.Help == "" {
			t.Errorf("key %q missing Help", k)
		}
	}
	if got := len(All()); got != len(v1Keys) {
		t.Fatalf("All() = %d settings, want %d", got, len(v1Keys))
	}
}

func TestV1KeySetExact(t *testing.T) {
	all := All()
	seen := map[string]bool{}
	for _, s := range all {
		seen[s.Key] = true
	}
	for _, k := range v1Keys {
		if !seen[k] {
			t.Errorf("All() missing %q", k)
		}
	}
}

func TestDefaultsUsable(t *testing.T) {
	d := Defaults()
	for _, k := range v1Keys {
		if _, ok := d[k]; !ok {
			t.Errorf("Defaults() missing %q", k)
		}
	}
}

func TestSearchTokenAND(t *testing.T) {
	got := Search("cursor blink")
	if len(got) != 1 || got[0].Key != "cursor-style-blink" {
		names := []string{}
		for _, s := range got {
			names = append(names, s.Key)
		}
		t.Fatalf("Search(cursor blink) = %v, want [cursor-style-blink]", names)
	}
	got = Search("FONT")
	found := false
	for _, s := range got {
		if strings.HasPrefix(s.Key, "font-") {
			found = true
		}
	}
	if !found {
		t.Fatalf("case-insensitive Search(FONT) found no font keys")
	}
}

func TestValidators(t *testing.T) {
	if ClampFontSize(2) != 4 || ClampFontSize(200) != 72 || ClampFontSize(13) != 13 {
		t.Fatal("ClampFontSize bounds wrong")
	}
	if ClampOpacity(-1) != 0 || ClampOpacity(9) != 1 || ClampOpacity(0.5) != 0.5 {
		t.Fatal("ClampOpacity bounds wrong")
	}
	if NormalizeColor("#FFAA00") != "#ffaa00" {
		t.Fatal("NormalizeColor should lowercase")
	}
	if !ValidateColor("#ffaa00") || !ValidateColor("#ffaa00cc") || ValidateColor("red") || ValidateColor("#xyz") {
		t.Fatal("ValidateColor wrong")
	}
}
