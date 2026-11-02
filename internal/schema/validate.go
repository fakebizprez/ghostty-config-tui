package schema

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
)

// IsHex reports whether s is #RRGGBB or #RRGGBBAA (renderable in previews).
func IsHex(s string) bool {
	s = strings.TrimSpace(s)
	if (len(s) != 7 && len(s) != 9) || s[0] != '#' {
		return false
	}
	_, err := strconv.ParseUint(s[1:], 16, 64)
	return err == nil
}

// ParseHex decodes the RGB of a #RRGGBB or #RRGGBBAA color (alpha ignored).
func ParseHex(s string) (color.RGBA, bool) {
	s = strings.TrimSpace(s)
	if !IsHex(s) {
		return color.RGBA{}, false
	}
	n, _ := strconv.ParseUint(s[1:7], 16, 32)
	return color.RGBA{uint8(n >> 16), uint8(n >> 8), uint8(n), 0xff}, true
}

// ValidateColor is kept for callers that need the strict hex form.
func ValidateColor(s string) bool { return IsHex(s) }

// NormalizeColor lowercases and adds the leading # to bare hex.
func NormalizeColor(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if hexRe.MatchString(s) && !strings.HasPrefix(s, "#") {
		s = "#" + s
	}
	if len(s) == 4 && s[0] == '#' && hexRe.MatchString(s) {
		s = "#" + string([]byte{s[1], s[1], s[2], s[2], s[3], s[3]})
	}
	return s
}

// ParseBool parses Ghostty bools.
func ParseBool(s string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes":
		return true, true
	case "false", "0", "no":
		return false, true
	}
	return false, false
}

// ParseFloat parses a float value.
func ParseFloat(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f, err == nil
}

// FormatNumber renders without trailing zeros.
func FormatNumber(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// Validate checks one value for a setting. It returns the normalized value,
// a non-fatal warning, or an error that blocks the commit.
func Validate(st Setting, v string) (string, string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", "", nil
	}
	switch st.Kind {
	case Bool:
		b, ok := ParseBool(v)
		if !ok {
			return "", "", fmt.Errorf("want true or false")
		}
		return strconv.FormatBool(b), "", nil
	case Int:
		n, err := strconv.Atoi(v)
		if err != nil {
			return "", "", fmt.Errorf("want a whole number")
		}
		if st.HasRange && (float64(n) < st.Min || float64(n) > st.Max) {
			return "", "", fmt.Errorf("want %s to %s", FormatNumber(st.Min), FormatNumber(st.Max))
		}
		return strconv.Itoa(n), "", nil
	case Float:
		f, ok := ParseFloat(v)
		if !ok {
			return "", "", fmt.Errorf("want a number")
		}
		if st.HasRange && (f < st.Min || f > st.Max) {
			return "", "", fmt.Errorf("want %s to %s", FormatNumber(st.Min), FormatNumber(st.Max))
		}
		return FormatNumber(f), "", nil
	case Color:
		c := NormalizeColor(v)
		if hexRe.MatchString(c) || contains(st.Options, v) || v == "cell-foreground" || v == "cell-background" {
			return c, "", nil
		}
		if isWord(v) {
			return v, "treated as an X11 color name", nil
		}
		return "", "", fmt.Errorf("want #RRGGBB or a color name")
	case Duration:
		if !durationRe.MatchString(v) {
			return "", "", fmt.Errorf("want a duration like 500ms, 5s, 1h30m")
		}
		return v, "", nil
	case Enum:
		if !contains(st.Options, v) {
			return v, "not a documented value, used as-is", nil
		}
		return v, "", nil
	case Flags:
		for _, p := range strings.Split(v, ",") {
			if !contains(st.Options, strings.TrimPrefix(strings.TrimSpace(p), "no-")) {
				return v, "unknown flag " + strings.TrimSpace(p), nil
			}
		}
		return v, "", nil
	case Palette:
		n, c, ok := strings.Cut(v, "=")
		idx, err := strconv.Atoi(strings.TrimSpace(n))
		if !ok || err != nil || idx < 0 || idx > 255 {
			return "", "", fmt.Errorf("want N=#RRGGBB with N from 0 to 255")
		}
		cc := NormalizeColor(c)
		if !hexRe.MatchString(cc) && !isWord(strings.TrimSpace(c)) {
			return "", "", fmt.Errorf("want N=#RRGGBB")
		}
		return strconv.Itoa(idx) + "=" + cc, "", nil
	case Keybind:
		kb, err := ParseKeybind(v)
		if err != nil {
			return "", "", err
		}
		if w := kb.Warning(active.Actions); w != "" {
			return kb.String(), w, nil
		}
		return kb.String(), "", nil
	}
	return v, "", nil
}

func isWord(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == ' ' || r == '-') {
			return false
		}
	}
	r := s[0]
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

// FlagState reports a flag's effective on/off in a flags value, falling back
// to the default value when unset.
func FlagState(value, def, flag string) bool {
	on := false
	for _, src := range []string{def, value} {
		for _, p := range strings.Split(src, ",") {
			switch strings.TrimSpace(p) {
			case flag, "true":
				on = true
			case "no-" + flag, "false":
				on = false
			}
		}
	}
	return on
}

// SetFlags renders every option explicitly on or off.
func SetFlags(options []string, on map[string]bool) string {
	parts := make([]string, 0, len(options))
	for _, o := range options {
		if on[o] {
			parts = append(parts, o)
		} else {
			parts = append(parts, "no-"+o)
		}
	}
	return strings.Join(parts, ",")
}

// KeybindPrefixes are the documented trigger prefixes.
var KeybindPrefixes = []string{"global", "all", "unconsumed", "performable"}

// Modifiers are the canonical trigger modifiers.
var Modifiers = []string{"ctrl", "alt", "shift", "super"}

var modAliases = map[string]string{
	"ctrl": "ctrl", "control": "ctrl",
	"alt": "alt", "opt": "alt", "option": "alt",
	"shift": "shift",
	"super": "super", "cmd": "super", "command": "super",
}

// Binding is one `keybind = ...` entry broken into builder fields.
type Binding struct {
	Prefixes []string
	Trigger  string // e.g. ctrl+shift+t or ctrl+a>n
	Action   string
	Param    string
	Raw      string // set for `clear`
}

// ParseKeybind parses `[prefix:]...trigger=action[:param]`.
func ParseKeybind(v string) (Binding, error) {
	v = strings.TrimSpace(v)
	if v == "clear" {
		return Binding{Raw: v}, nil
	}
	trig, act, ok := strings.Cut(v, "=")
	if !ok {
		return Binding{}, fmt.Errorf("want trigger=action, e.g. ctrl+shift+t=new_tab")
	}
	kb := Binding{}
	for {
		p, rest, found := strings.Cut(trig, ":")
		if !found || !contains(KeybindPrefixes, p) {
			break
		}
		kb.Prefixes = append(kb.Prefixes, p)
		trig = rest
	}
	kb.Trigger = strings.TrimSpace(trig)
	if kb.Trigger == "" {
		return Binding{}, fmt.Errorf("missing trigger")
	}
	if kb.Trigger != "chain" {
		for _, step := range strings.Split(kb.Trigger, ">") {
			if err := checkStep(step); err != nil {
				return Binding{}, err
			}
		}
	}
	kb.Action, kb.Param, _ = strings.Cut(strings.TrimSpace(act), ":")
	if kb.Action == "" {
		return Binding{}, fmt.Errorf("missing action")
	}
	return kb, nil
}

func checkStep(step string) error {
	parts := strings.Split(step, "+")
	key := parts[len(parts)-1]
	if strings.HasSuffix(step, "++") || step == "+" { // the plus key itself
		key = "+"
		parts = strings.Split(strings.TrimSuffix(step, "++"), "+")
		if step != "+" {
			parts = append(parts, "+")
		}
	}
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("trigger step %q has no key", step)
	}
	for _, m := range parts[:len(parts)-1] {
		if _, ok := modAliases[strings.ToLower(m)]; !ok {
			return fmt.Errorf("unknown modifier %q (use ctrl, alt, shift, super)", m)
		}
	}
	return nil
}

// Warning flags an action Ghostty does not list.
func (k Binding) Warning(actions []string) string {
	if k.Raw != "" || len(actions) == 0 || contains(actions, k.Action) {
		return ""
	}
	return "unknown action " + k.Action
}

// String renders the keybind back to config syntax.
func (k Binding) String() string {
	if k.Raw != "" {
		return k.Raw
	}
	var sb strings.Builder
	for _, p := range k.Prefixes {
		sb.WriteString(p + ":")
	}
	sb.WriteString(k.Trigger + "=" + k.Action)
	if k.Param != "" {
		sb.WriteString(":" + k.Param)
	}
	return sb.String()
}

// TriggerFromKey converts a Bubble Tea keystroke like "ctrl+shift+t" to
// Ghostty trigger syntax.
func TriggerFromKey(keystroke string) string {
	parts := strings.Split(keystroke, "+")
	if strings.HasSuffix(keystroke, "++") {
		parts = append(strings.Split(strings.TrimSuffix(keystroke, "++"), "+"), "plus")
	}
	key := parts[len(parts)-1]
	var mods []string
	for _, m := range parts[:len(parts)-1] {
		switch m {
		case "meta", "super":
			m = "super"
		}
		if canon, ok := modAliases[m]; ok && !contains(mods, canon) {
			mods = append(mods, canon)
		}
	}
	switch key {
	case "space":
		key = "space"
	case "pgup":
		key = "page_up"
	case "pgdown":
		key = "page_down"
	case "esc":
		key = "escape"
	}
	return strings.Join(append(mods, key), "+")
}
