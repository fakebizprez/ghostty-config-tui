package ui

import (
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"ghostty-config-tui/internal/schema"
)

// editor is the open widget for one setting. Repeatable settings get a list
// editor whose items are edited by a nested scalar editor (sub).
type editor struct {
	st    schema.Setting
	buf   string
	idx   int // picker match, flag, list item, or color channel
	flags map[string]bool
	err   string
	warn  string

	// list editor
	list  bool
	items []string
	sub   *editor
	subAt int // -1 appends

	// keybind builder
	kbField  int // 0 prefixes, 1 trigger, 2 action, 3 param
	kbPrefix map[string]bool
	kbPreIdx int
	kbTrig   string
	kbAct    string // action filter text
	kbParam  string
	kbRecord int // 0 off, 1 replace, 2 append a sequence step

	options []string // picker source
}

func (m *Model) newEditor(st schema.Setting) *editor {
	if st.Repeatable {
		return &editor{st: st, list: true, items: slices.Clone(m.vals[st.Key]), subAt: -1}
	}
	return m.scalarEditor(st, m.scalar(st.Key))
}

func (m *Model) scalarEditor(st schema.Setting, cur string) *editor {
	e := &editor{st: st}
	switch st.Kind {
	case schema.Theme:
		e.options = m.opts.Themes
	case schema.Font:
		e.options = m.opts.Fonts
	case schema.Enum:
		e.options = st.Options
	}
	switch st.Kind {
	case schema.Theme, schema.Font, schema.Enum:
		// Start unfiltered with the current value highlighted.
		want := cur
		if want == "" {
			want = st.Default
		}
		if i := slices.Index(e.options, want); i >= 0 {
			e.idx = i
		} else if cur != "" {
			e.buf, e.idx = cur, -1 // keep the custom value, not a fuzzy match
		}
	case schema.Flags:
		e.flags = map[string]bool{}
		for _, o := range st.Options {
			e.flags[o] = schema.FlagState(cur, st.Default, o)
		}
	case schema.Keybind:
		e.kbPrefix = map[string]bool{}
		e.kbField = 1
		if kb, err := schema.ParseKeybind(cur); err == nil && kb.Raw == "" {
			for _, p := range kb.Prefixes {
				e.kbPrefix[p] = true
			}
			e.kbTrig, e.kbAct, e.kbParam = kb.Trigger, kb.Action, kb.Param
		}
		e.options = m.sch.Actions
	default:
		e.buf = cur
		if e.buf == "" && st.Kind != schema.String {
			e.buf = st.Default
		}
	}
	return e
}

// itemSetting is the scalar form of a repeatable setting's entries.
func itemSetting(st schema.Setting) schema.Setting {
	st.Repeatable = false
	st.Default = ""
	return st
}

func (e *editor) picker() bool {
	switch e.st.Kind {
	case schema.Theme, schema.Font, schema.Enum:
		return len(e.options) > 0
	}
	return false
}

// matches filters picker options; prefix matches rank first.
func matches(opts []string, q string) []string {
	if q == "" {
		return opts
	}
	q = strings.ToLower(q)
	var pre, sub []string
	for _, o := range opts {
		lo := strings.ToLower(o)
		switch {
		case strings.HasPrefix(lo, q):
			pre = append(pre, o)
		case strings.Contains(lo, q):
			sub = append(sub, o)
		}
	}
	return append(pre, sub...)
}

func (e *editor) matches() []string {
	if e.st.Kind == schema.Keybind {
		return matches(e.options, e.kbAct)
	}
	return matches(e.options, e.buf)
}

// highlighted is the picker row under the cursor; idx -1 is the typed text.
func (e *editor) highlighted() string {
	ms := e.matches()
	if e.idx >= 0 && e.idx < len(ms) {
		return ms[e.idx]
	}
	if e.st.Kind == schema.Keybind {
		return e.kbAct
	}
	return e.buf
}

// value assembles what Enter would commit.
func (e *editor) value() string {
	switch {
	case e.st.Kind == schema.Keybind:
		kb := schema.Binding{Trigger: e.kbTrig, Action: e.highlighted(), Param: e.kbParam}
		for _, p := range schema.KeybindPrefixes {
			if e.kbPrefix[p] {
				kb.Prefixes = append(kb.Prefixes, p)
			}
		}
		return kb.String()
	case e.st.Kind == schema.Flags:
		var parts []string
		for _, o := range e.st.Options {
			if e.flags[o] != schema.FlagState("", e.st.Default, o) {
				if e.flags[o] {
					parts = append(parts, o)
				} else {
					parts = append(parts, "no-"+o)
				}
			}
		}
		return strings.Join(parts, ",")
	case e.picker():
		return e.highlighted()
	}
	return e.buf
}

// live returns the in-progress value when it is valid, for the preview.
func (e *editor) live() (string, bool) {
	if e.list {
		return "", false
	}
	v, _, err := schema.Validate(e.st, e.value())
	return v, err == nil
}

func (e *editor) paste(s string) {
	s = strings.ReplaceAll(strings.TrimRight(s, "\r\n"), "\n", " ")
	if e.sub != nil {
		e.sub.paste(s)
		return
	}
	switch {
	case e.st.Kind == schema.Keybind && e.kbField == 1:
		e.kbTrig += s
	case e.st.Kind == schema.Keybind && e.kbField == 2:
		e.kbAct += s
	case e.st.Kind == schema.Keybind && e.kbField == 3:
		e.kbParam += s
	default:
		e.buf += s
		e.idx = 0
	}
}

const (
	edContinue = iota
	edCancel
	edCommit
)

// editKey routes a key to the open editor and applies commits.
func (m *Model) editKey(msg tea.KeyPressMsg) tea.Cmd {
	e := m.ed
	if e.list {
		return m.listKey(msg)
	}
	switch m.scalarKey(e, msg) {
	case edCancel:
		m.ed = nil
		m.note("")
	case edCommit:
		v, warn, err := schema.Validate(e.st, e.value())
		if err != nil {
			e.err = err.Error()
			return nil
		}
		m.set(e.st.Key, []string{v})
		m.ed = nil
		shown := v
		if _, set := m.vals[e.st.Key]; !set {
			shown = "default (" + orNone(e.st.Default) + ")"
		}
		m.note(e.st.Key + " = " + shown + suffix(warn))
		if e.st.Kind == schema.Theme {
			return m.requestTheme(v)
		}
	default:
		if e.st.Kind == schema.Theme {
			return m.requestTheme(e.highlighted())
		}
	}
	return nil
}

func suffix(warn string) string {
	if warn == "" {
		return ""
	}
	return " (warning: " + warn + ")"
}

func (m *Model) listKey(msg tea.KeyPressMsg) tea.Cmd {
	e := m.ed
	if e.sub != nil {
		switch m.scalarKey(e.sub, msg) {
		case edCancel:
			e.sub = nil
		case edCommit:
			v, warn, err := schema.Validate(e.sub.st, e.sub.value())
			if err != nil {
				e.sub.err = err.Error()
				return nil
			}
			if v == "" {
				e.sub = nil
				return nil
			}
			if e.subAt < 0 {
				e.items = append(e.items, v)
				e.idx = len(e.items) - 1
			} else {
				e.items[e.subAt] = v
			}
			e.sub, e.warn = nil, warn
			m.set(e.st.Key, e.items)
		}
		return nil
	}
	k := msg.String()
	switch k {
	case "esc", "q":
		m.ed = nil
		m.note(e.st.Key + ": " + plural(len(e.items), "entry"))
	case "down", "j":
		e.idx = min(e.idx+1, max(len(e.items)-1, 0))
	case "up", "k":
		e.idx = max(e.idx-1, 0)
	case "a", "+", "o":
		e.sub, e.subAt = m.scalarEditor(itemSetting(e.st), ""), -1
	case "enter", "e":
		if len(e.items) == 0 {
			e.sub, e.subAt = m.scalarEditor(itemSetting(e.st), ""), -1
			return nil
		}
		e.sub, e.subAt = m.scalarEditor(itemSetting(e.st), e.items[e.idx]), e.idx
	case "x", "d", "delete", "backspace":
		if len(e.items) > 0 {
			e.items = slices.Delete(e.items, e.idx, e.idx+1)
			e.idx = min(e.idx, max(len(e.items)-1, 0))
			m.set(e.st.Key, e.items)
		}
	case "K", "shift+up":
		if e.idx > 0 {
			e.items[e.idx], e.items[e.idx-1] = e.items[e.idx-1], e.items[e.idx]
			e.idx--
			m.set(e.st.Key, e.items)
		}
	case "J", "shift+down":
		if e.idx < len(e.items)-1 {
			e.items[e.idx], e.items[e.idx+1] = e.items[e.idx+1], e.items[e.idx]
			e.idx++
			m.set(e.st.Key, e.items)
		}
	}
	return nil
}

// scalarKey handles one key for a scalar editor.
func (m *Model) scalarKey(e *editor, msg tea.KeyPressMsg) int {
	k := msg.String()
	e.err = ""
	if e.st.Kind == schema.Keybind {
		return e.keybindKey(msg)
	}
	switch k {
	case "esc":
		return edCancel
	case "enter":
		return edCommit
	case "ctrl+u":
		e.buf, e.idx = "", 0
		return edContinue
	case "backspace":
		e.buf = trimLast(e.buf)
		e.idx = e.firstRow()
		return edContinue
	}
	switch {
	case e.picker():
		n := len(e.matches())
		lo := 0
		if e.buf != "" {
			lo = -1 // row -1: use the typed text as-is
		}
		switch k {
		case "down", "tab", "ctrl+n":
			if e.idx < n-1 {
				e.idx++
			} else {
				e.idx = lo
			}
		case "up", "shift+tab", "ctrl+p":
			if e.idx > lo {
				e.idx--
			} else {
				e.idx = max(n-1, lo)
			}
		case "pgdown":
			e.idx = min(e.idx+10, max(n-1, 0))
		case "pgup":
			e.idx = max(e.idx-10, lo)
		default:
			if msg.Text != "" {
				e.buf += msg.Text
				e.idx = e.firstRow()
			}
		}
	case e.st.Kind == schema.Flags:
		switch k {
		case "down", "j", "tab":
			e.idx = min(e.idx+1, len(e.st.Options)-1)
		case "up", "k", "shift+tab":
			e.idx = max(e.idx-1, 0)
		case "space", " ", "x":
			o := e.st.Options[e.idx]
			e.flags[o] = !e.flags[o]
		}
	case e.st.Kind == schema.Int || e.st.Kind == schema.Float:
		step := e.st.Step
		if step == 0 {
			step = 1
		}
		switch k {
		case "right", "up", "l", "+", "=":
			e.buf = e.nudge(step)
		case "left", "down", "h", "-":
			if k == "-" && e.buf == "" {
				e.buf = "-"
				break
			}
			e.buf = e.nudge(-step)
		case "shift+right", "shift+up", "L":
			e.buf = e.nudge(step * 10)
		case "shift+left", "shift+down", "H":
			e.buf = e.nudge(-step * 10)
		default:
			if t := msg.Text; t != "" && strings.ContainsAny(t, "0123456789.-") {
				e.buf += t
			}
		}
	case e.st.Kind == schema.Color:
		switch k {
		case "up", "shift+tab":
			e.idx = (e.idx + 2) % 3
		case "down", "tab":
			e.idx = (e.idx + 1) % 3
		case "right":
			e.buf = e.channel(1)
		case "left":
			e.buf = e.channel(-1)
		case "shift+right":
			e.buf = e.channel(16)
		case "shift+left":
			e.buf = e.channel(-16)
		default:
			e.buf += msg.Text
		}
	default:
		e.buf += msg.Text
	}
	return edContinue
}

// firstRow highlights the best match after typing, or the typed text when
// nothing matches.
func (e *editor) firstRow() int {
	if len(e.matches()) == 0 {
		return -1
	}
	return 0
}

func (e *editor) nudge(d float64) string {
	f, ok := schema.ParseFloat(e.buf)
	if !ok {
		f, _ = schema.ParseFloat(e.st.Default)
	}
	f += d
	if e.st.HasRange {
		f = min(max(f, e.st.Min), e.st.Max)
	}
	if e.st.Kind == schema.Int {
		return strconv.Itoa(int(f))
	}
	// Round to the step's precision to avoid 0.30000000000000004.
	return schema.FormatNumber(float64(int64(f*1000+copysign(0.5, f))) / 1000)
}

func copysign(v, sign float64) float64 {
	if sign < 0 {
		return -v
	}
	return v
}

// RGB returns the color buffer's channels (zero when not hex).
func (e *editor) RGB() [3]int {
	c := schema.NormalizeColor(e.buf)
	if !schema.IsHex(c) {
		c = schema.NormalizeColor(e.st.Default)
	}
	var out [3]int
	if !schema.IsHex(c) {
		return out
	}
	for i := 0; i < 3; i++ {
		n, _ := strconv.ParseUint(c[1+2*i:3+2*i], 16, 8)
		out[i] = int(n)
	}
	return out
}

func (e *editor) channel(d int) string {
	rgb := e.RGB()
	rgb[e.idx] = min(max(rgb[e.idx]+d, 0), 255)
	return hex(rgb)
}

func hex(rgb [3]int) string {
	const digits = "0123456789abcdef"
	b := []byte{'#'}
	for _, v := range rgb {
		b = append(b, digits[v>>4], digits[v&15])
	}
	return string(b)
}

func (e *editor) keybindKey(msg tea.KeyPressMsg) int {
	k := msg.String()
	if e.kbRecord > 0 {
		trig := schema.TriggerFromKey(msg.Keystroke())
		if e.kbRecord == 2 && e.kbTrig != "" {
			e.kbTrig += ">" + trig
		} else {
			e.kbTrig = trig
		}
		e.kbRecord = 0
		return edContinue
	}
	switch k {
	case "esc":
		return edCancel
	case "enter":
		return edCommit
	case "tab":
		e.kbField = (e.kbField + 1) % 4
		return edContinue
	case "shift+tab":
		e.kbField = (e.kbField + 3) % 4
		return edContinue
	case "ctrl+r":
		e.kbRecord = 1
		return edContinue
	case "ctrl+t":
		e.kbRecord = 2
		return edContinue
	}
	field := map[int]*string{1: &e.kbTrig, 2: &e.kbAct, 3: &e.kbParam}[e.kbField]
	switch e.kbField {
	case 0:
		switch k {
		case "left", "h":
			e.kbPreIdx = max(e.kbPreIdx-1, 0)
		case "right", "l":
			e.kbPreIdx = min(e.kbPreIdx+1, len(schema.KeybindPrefixes)-1)
		case "space", " ", "x":
			p := schema.KeybindPrefixes[e.kbPreIdx]
			e.kbPrefix[p] = !e.kbPrefix[p]
		}
		return edContinue
	case 2:
		n := len(e.matches())
		switch k {
		case "down", "ctrl+n":
			if n > 0 {
				e.idx = (e.idx + 1) % n
			}
			return edContinue
		case "up", "ctrl+p":
			if n > 0 {
				e.idx = (e.idx - 1 + n) % n
			}
			return edContinue
		}
	}
	switch k {
	case "backspace":
		*field = trimLast(*field)
		e.idx = 0
	case "ctrl+u":
		*field = ""
		e.idx = 0
	default:
		if msg.Text != "" {
			*field += msg.Text
			e.idx = 0
		}
	}
	return edContinue
}

func itoa(n int) string { return strconv.Itoa(n) }
