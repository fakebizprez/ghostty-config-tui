// Package schema is the typed setting registry, generated from Ghostty's own
// `ghostty +show-config --default --docs` output. A snapshot is embedded so
// the app works without Ghostty on PATH; a live dump replaces it at startup
// so keys, defaults, and docs match the installed version.
package schema

import (
	_ "embed"
	"regexp"
	"strconv"
	"strings"
)

// Refresh the snapshots with:
//
//	ghostty +show-config --default --docs > internal/schema/ghostty-docs.txt
//	ghostty +list-actions > internal/schema/ghostty-actions.txt
//
//go:embed ghostty-docs.txt
var embeddedDocs string

//go:embed ghostty-actions.txt
var embeddedActions string

// Kind is the editor widget type of a setting.
type Kind string

// Setting kinds.
const (
	String   Kind = "string"
	Bool     Kind = "bool"
	Int      Kind = "int"
	Float    Kind = "float"
	Color    Kind = "color"
	Enum     Kind = "enum"
	Flags    Kind = "flags"
	Duration Kind = "duration"
	Theme    Kind = "theme"
	Font     Kind = "font"
	Keybind  Kind = "keybind"
	Palette  Kind = "palette"
)

// Setting describes one Ghostty config key.
type Setting struct {
	Key        string
	Title      string
	Category   string
	Kind       Kind
	Default    string   // last default line (what Ghostty uses for scalars)
	Defaults   []string // every default line (repeatable keys)
	Repeatable bool
	Options    []string // enum choices, or flag names for Flags
	Docs       string
	Platforms  []string // macOS, Linux, GTK
	Since      string   // minimum Ghostty version, when documented
	Min, Max   float64
	Step       float64
	HasRange   bool
}

// Summary is the first sentence of the docs.
func (s Setting) Summary() string {
	d := strings.Join(strings.Fields(s.Docs), " ")
	if i := strings.Index(d, ". "); i >= 0 {
		return d[:i+1]
	}
	return d
}

// Schema is an ordered set of settings.
type Schema struct {
	Settings []Setting
	byKey    map[string]int
	Actions  []string
}

// Categories in sidebar order. Rules are matched top to bottom.
var Categories = []string{
	"Fonts", "Theme & Colors", "Background", "Cursor", "Window", "Tabs & Splits",
	"Mouse & Links", "Clipboard", "Keybindings", "Shell & Command", "Terminal",
	"Notifications", "Quick Terminal", "macOS", "Linux & GTK", "Shaders",
	"Config & Updates", "Advanced",
}

var categoryRules = []struct {
	cat      string
	prefixes []string
}{
	{"Keybindings", []string{"keybind", "key-remap", "command-palette-entry"}},
	{"Quick Terminal", []string{"quick-terminal-", "gtk-quick-terminal-"}},
	{"macOS", []string{"macos-"}},
	{"Linux & GTK", []string{"gtk-", "linux-", "x11-", "language", "async-backend"}},
	{"Fonts", []string{"font-", "adjust-", "grapheme-", "freetype-", "alpha-blending", "window-title-font-family"}},
	{"Cursor", []string{"cursor-"}},
	{"Background", []string{"background-", "unfocused-split-"}},
	{"Theme & Colors", []string{"theme", "background", "foreground", "palette", "selection-", "minimum-contrast", "bold-color", "faint-opacity", "search-", "split-divider-color", "window-padding-color", "osc-color-report-format"}},
	{"Tabs & Splits", []string{"tab-", "split-", "window-new-tab-position", "window-show-tab-bar"}},
	{"Mouse & Links", []string{"mouse-", "click-", "focus-follows-mouse", "copy-on-select", "right-click-action", "middle-click-action", "link-", "drag-handle"}},
	{"Clipboard", []string{"clipboard-"}},
	{"Notifications", []string{"bell-", "app-notifications", "notify-on-command-", "desktop-notifications", "progress-style"}},
	{"Shell & Command", []string{"command", "initial-command", "shell-integration", "working-directory", "env", "input", "wait-after-command", "abnormal-command-exit-runtime", "term", "enquiry-response"}},
	{"Terminal", []string{"scrollback-", "scroll-", "scrollbar", "image-storage-limit", "vt-", "title-report", "selection-word-chars"}},
	{"Window", []string{"window-", "maximize", "fullscreen", "title", "class", "resize-overlay", "initial-window", "confirm-close-surface", "quit-after-", "undo-timeout"}},
	{"Shaders", []string{"custom-shader"}},
	{"Config & Updates", []string{"config-", "auto-update"}},
}

func categoryFor(key string) string {
	for _, r := range categoryRules {
		for _, p := range r.prefixes {
			base := strings.TrimSuffix(p, "-")
			if key == base || strings.HasPrefix(key, base+"-") {
				return r.cat
			}
		}
	}
	return "Advanced"
}

// Hand-tuned ranges for slider-style numeric editing.
var ranges = map[string][3]float64{
	"font-size":                {1, 255, 0.5},
	"background-opacity":       {0, 1, 0.05},
	"background-image-opacity": {0, 1, 0.05},
	"unfocused-split-opacity":  {0.15, 1, 0.05},
	"cursor-opacity":           {0, 1, 0.05},
	"faint-opacity":            {0, 1, 0.05},
	"bell-audio-volume":        {0, 1, 0.05},
	"minimum-contrast":         {1, 21, 0.5},
	"font-thicken-strength":    {0, 255, 5},
}

// Kind overrides where the default value alone misleads inference.
var kindOverrides = map[string]Kind{
	"font-size":            Float,
	"window-padding-x":     String, // accepts "2" or "2,4"
	"window-padding-y":     String,
	"background-blur":      Enum, // false/true/radius/macos-glass-*
	"command":              String,
	"macos-titlebar-style": Enum,
	"gtk-tabs-location":    Enum,
	"gtk-titlebar-style":   Enum,
	"gtk-single-instance":  Enum,
	"window-show-tab-bar":  Enum,
}

var extraOptions = map[string][]string{
	"background-blur":      {"false", "true", "20", "macos-glass-regular", "macos-glass-clear"},
	"cursor-style-blink":   {"true", "false"},
	"macos-titlebar-style": {"native", "transparent", "tabs", "hidden"},
	"gtk-tabs-location":    {"top", "bottom", "hidden"},
	"gtk-titlebar-style":   {"native", "tabs"},
	"gtk-single-instance":  {"detect", "true", "false"},
	"window-show-tab-bar":  {"always", "auto", "never"},
}

var (
	bulletRe   = regexp.MustCompile("^(\\s*)\\* `([A-Za-z0-9_.:+-]+)`")
	tickRe     = regexp.MustCompile("`([A-Za-z0-9_.:+-]+)`")
	sinceRe    = regexp.MustCompile(`^Available since:? ([0-9]+\.[0-9]+(\.[0-9]+)?)`)
	durationRe = regexp.MustCompile(`^([0-9]+(\.[0-9]+)?(y|w|d|h|m|s|ms|us|µs|ns)\s*)+$`)
	hexRe      = regexp.MustCompile(`^#?([0-9a-fA-F]{6}|[0-9a-fA-F]{8}|[0-9a-fA-F]{3})$`)
	flagRe     = regexp.MustCompile(`^[a-z0-9-]+$`)
)

// Parse builds a schema from `ghostty +show-config --default --docs` text.
func Parse(docs, actions string) *Schema {
	s := &Schema{byKey: map[string]int{}}
	var comment []string
	flush := false
	for _, line := range strings.Split(docs, "\n") {
		if strings.HasPrefix(line, "#") {
			if flush {
				comment, flush = nil, false
			}
			c := strings.TrimPrefix(line, "#")
			c = strings.TrimPrefix(c, " ")
			comment = append(comment, c)
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		if i, seen := s.byKey[key]; seen {
			st := &s.Settings[i]
			st.Defaults = append(st.Defaults, val)
			st.Default = val
			st.Repeatable = true
			continue
		}
		s.byKey[key] = len(s.Settings)
		s.Settings = append(s.Settings, Setting{
			Key:      key,
			Default:  val,
			Defaults: []string{val},
			Docs:     strings.TrimSpace(strings.Join(comment, "\n")),
		})
		flush = true
	}
	for i := range s.Settings {
		finish(&s.Settings[i])
	}
	for _, a := range strings.Split(actions, "\n") {
		if a = strings.TrimSpace(a); a != "" {
			s.Actions = append(s.Actions, a)
		}
	}
	return s
}

func finish(st *Setting) {
	st.Title = titleCase(st.Key)
	st.Category = categoryFor(st.Key)
	if st.Defaults[0] == "" && len(st.Defaults) == 1 {
		st.Defaults = nil
	}
	{ // Curated: the docs word this inconsistently ("repeated", "repeatable", ...).
		switch st.Key {
		case "font-family", "font-family-bold", "font-family-italic", "font-family-bold-italic",
			"font-feature", "font-variation", "font-variation-bold", "font-variation-italic",
			"font-variation-bold-italic", "font-codepoint-map", "clipboard-codepoint-map",
			"config-file", "env", "input", "key-remap", "keybind", "palette", "command-palette-entry",
			"custom-shader":
			st.Repeatable = true
		}
	}
	st.Platforms = platforms(st.Key, st.Docs)
	st.Since = since(st.Docs)
	bullets := topBullets(st.Docs)
	st.Kind = inferKind(st, bullets)
	if r, ok := ranges[st.Key]; ok {
		st.Min, st.Max, st.Step, st.HasRange = r[0], r[1], r[2], true
		if st.Step < 1 {
			st.Kind = Float
		}
	} else if st.Kind == Int {
		st.Step = 1
	} else if st.Kind == Float {
		st.Step = 0.1
	}
}

func inferKind(st *Setting, bullets []string) Kind {
	k, d := st.Key, st.Default
	if o, ok := kindOverrides[k]; ok {
		if o == Enum {
			st.Options = extraOptions[k]
		}
		return o
	}
	switch {
	case k == "theme":
		return Theme
	case k == "keybind":
		return Keybind
	case k == "palette":
		return Palette
	case strings.HasPrefix(k, "font-family") || k == "window-title-font-family":
		return Font
	case d == "true" || d == "false":
		return Bool
	case k == "cursor-style-blink":
		st.Options = extraOptions[k]
		return Bool
	}
	if hexRe.MatchString(d) && strings.HasPrefix(d, "#") || isColorDoc(st) {
		st.Options = bullets // special values like cell-foreground
		return Color
	}
	if d != "" && durationRe.MatchString(d) && !isNumber(d) {
		return Duration
	}
	if d == "" && strings.Contains(strings.ToLower(st.Docs), "duration") && strings.Contains(st.Docs, "ms") {
		return Duration
	}
	if _, err := strconv.Atoi(d); err == nil && !st.Repeatable {
		return Int
	}
	if isNumber(d) && !st.Repeatable {
		return Float
	}
	if strings.Contains(d, ",") && !st.Repeatable && allFlags(d) {
		st.Options = flagNames(d, bullets)
		return Flags
	}
	if !st.Repeatable && len(bullets) >= 2 && (d == "" || contains(bullets, d)) {
		st.Options = bullets
		return Enum
	}
	return String
}

func isColorDoc(st *Setting) bool {
	if st.Repeatable {
		return false
	}
	return strings.Contains(st.Docs, "#RRGGBB") || strings.Contains(st.Docs, "X11 color")
}

func isNumber(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

func allFlags(d string) bool {
	for _, p := range strings.Split(d, ",") {
		if !flagRe.MatchString(strings.TrimSpace(p)) {
			return false
		}
	}
	return true
}

func flagNames(d string, bullets []string) []string {
	var out []string
	for _, p := range strings.Split(d, ",") {
		n := strings.TrimPrefix(strings.TrimSpace(p), "no-")
		if !contains(out, n) {
			out = append(out, n)
		}
	}
	for _, b := range bullets {
		b = strings.TrimPrefix(b, "no-")
		if !contains(out, b) && flagRe.MatchString(b) {
			out = append(out, b)
		}
	}
	return out
}

// topBullets returns "* `value`" options at the shallowest bullet indent.
func topBullets(docs string) []string {
	minIndent := -1
	type b struct {
		indent int
		val    string
	}
	var all []b
	for _, l := range strings.Split(docs, "\n") {
		m := bulletRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		head, _, _ := strings.Cut(l, " - ")
		for _, t := range tickRe.FindAllStringSubmatch(head, -1) {
			all = append(all, b{len(m[1]), t[1]})
		}
		if minIndent < 0 || len(m[1]) < minIndent {
			minIndent = len(m[1])
		}
	}
	var out []string
	for _, x := range all {
		if x.indent == minIndent && !contains(out, x.val) {
			out = append(out, x.val)
		}
	}
	return out
}

func platforms(key, docs string) []string {
	var p []string
	add := func(s string) {
		if !contains(p, s) {
			p = append(p, s)
		}
	}
	switch {
	case strings.HasPrefix(key, "macos-"):
		add("macOS")
	case strings.HasPrefix(key, "gtk-"):
		add("GTK")
	case strings.HasPrefix(key, "linux-"), strings.HasPrefix(key, "x11-"):
		add("Linux")
	}
	flat := strings.Join(strings.Fields(docs), " ")
	if strings.Contains(flat, "macOS and Linux") {
		return p
	}
	for _, pat := range []struct{ needle, plat string }{
		{"macOS only", "macOS"}, {"only supported on macOS", "macOS"}, {"only works on macOS", "macOS"},
		{"GTK only", "GTK"}, {"only supported on GTK", "GTK"},
		{"Linux only", "Linux"}, {"only supported on Linux", "Linux"},
	} {
		if strings.Contains(flat, pat.needle) {
			add(pat.plat)
		}
	}
	return p
}

func since(docs string) string {
	got := ""
	for _, l := range strings.Split(docs, "\n") {
		if m := sinceRe.FindStringSubmatch(l); m != nil {
			got = m[1] // unindented line: applies to the whole setting
		}
	}
	return got
}

func titleCase(key string) string {
	parts := strings.Split(key, "-")
	for i, p := range parts {
		switch p {
		case "macos":
			parts[i] = "macOS"
		case "gtk", "osc", "vt", "x11", "url", "osc8", "xt":
			parts[i] = strings.ToUpper(p)
		default:
			if p != "" {
				parts[i] = strings.ToUpper(p[:1]) + p[1:]
			}
		}
	}
	return strings.Join(parts, " ")
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// ByKey looks up a setting.
func (s *Schema) ByKey(key string) (Setting, bool) {
	i, ok := s.byKey[key]
	if !ok {
		return Setting{}, false
	}
	return s.Settings[i], true
}

// InCategory returns settings of one category in Ghostty order.
func (s *Schema) InCategory(cat string) []Setting {
	var out []Setting
	for _, st := range s.Settings {
		if st.Category == cat {
			out = append(out, st)
		}
	}
	return out
}

// Search ranks key/title matches above docs-only matches. Tokens are ANDed.
func (s *Schema) Search(q string) []Setting {
	toks := strings.Fields(strings.ToLower(q))
	if len(toks) == 0 {
		return nil
	}
	var keyHits, docHits []Setting
	for _, st := range s.Settings {
		key := strings.ToLower(st.Key + " " + st.Title)
		docs := strings.ToLower(st.Docs)
		inKey, inAny := true, true
		for _, t := range toks {
			if !strings.Contains(key, t) {
				inKey = false
			}
			if !strings.Contains(key, t) && !strings.Contains(docs, t) {
				inAny = false
			}
		}
		switch {
		case inKey:
			keyHits = append(keyHits, st)
		case inAny:
			docHits = append(docHits, st)
		}
	}
	return append(keyHits, docHits...)
}

var active = Parse(embeddedDocs, embeddedActions)

// Use replaces the active schema with a live Ghostty dump. Empty or
// unparseable input keeps the embedded snapshot.
func Use(docs, actions string) {
	if actions == "" {
		actions = embeddedActions
	}
	if s := Parse(docs, actions); len(s.Settings) > 50 {
		active = s
	}
}

// Active returns the schema in use.
func Active() *Schema { return active }

// All returns every setting in Ghostty order.
func All() []Setting { return active.Settings }

// ByKey looks up a setting in the active schema.
func ByKey(key string) (Setting, bool) { return active.ByKey(key) }

// Defaults returns the scalar default per key.
func Defaults() map[string]string {
	m := make(map[string]string, len(active.Settings))
	for _, s := range active.Settings {
		m[s.Key] = s.Default
	}
	return m
}
