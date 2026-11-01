// Package config handles Ghostty config path resolution, round-trip
// parsing, surgical writes, atomic saves, and one-level undo.
package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"ghostty-config-tui/internal/ghostty"
)

// Line is one source line: kv pairs are editable, everything else is raw.
type Line struct {
	Kind  string // blank, comment, kv, other
	Key   string
	Value string
	Raw   string
}

// File is a parsed config preserving line order and raw bytes.
type File struct {
	Lines       []Line
	TrailingNL  bool
	HasTrailing bool
}

// Parse parses raw config text, preserving every line.
func Parse(src string) *File {
	f := &File{}
	if src == "" {
		return f
	}
	f.TrailingNL = strings.HasSuffix(src, "\n")
	trimmed := strings.TrimSuffix(src, "\n")
	for _, raw := range strings.Split(trimmed, "\n") {
		f.Lines = append(f.Lines, parseLine(raw))
	}
	return f
}

func parseLine(raw string) Line {
	if strings.TrimSpace(raw) == "" {
		return Line{Kind: "blank", Raw: raw}
	}
	trimmed := strings.TrimLeft(raw, " \t")
	if strings.HasPrefix(trimmed, "#") {
		return Line{Kind: "comment", Raw: raw}
	}
	if i := strings.Index(raw, "="); i >= 0 {
		key := strings.TrimSpace(raw[:i])
		val := strings.TrimSpace(raw[i+1:])
		if key != "" && !strings.ContainsAny(key, " \t") {
			return Line{Kind: "kv", Key: key, Value: val, Raw: raw}
		}
	}
	return Line{Kind: "other", Raw: raw}
}

// ParseFile reads and parses a file; missing file yields an empty File.
func ParseFile(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{}, nil
		}
		return nil, err
	}
	return Parse(string(b)), nil
}

// Get returns the last value for a key.
func (f *File) Get(key string) string {
	got := ""
	for _, l := range f.Lines {
		if l.Kind == "kv" && l.Key == key {
			got = l.Value
		}
	}
	return got
}

// Set rewrites the last occurrence of key, drops earlier duplicates, or
// appends when absent. Unmanaged lines are never touched.
func (f *File) Set(key, value string) {
	last := -1
	for i, l := range f.Lines {
		if l.Kind == "kv" && l.Key == key {
			last = i
		}
	}
	if last >= 0 {
		kept := f.Lines[:0]
		for i, l := range f.Lines {
			if l.Kind == "kv" && l.Key == key && i != last {
				continue
			}
			kept = append(kept, l)
		}
		f.Lines = kept
		for i, l := range f.Lines {
			if l.Kind == "kv" && l.Key == key {
				f.Lines[i].Value = value
				f.Lines[i].Raw = l.Key + " = " + value
			}
		}
		return
	}
	f.Lines = append(f.Lines, Line{Kind: "kv", Key: key, Value: value, Raw: key + " = " + value})
}

// GetAll returns every value for a repeatable key, in file order.
func (f *File) GetAll(key string) []string {
	var out []string
	for _, l := range f.Lines {
		if l.Kind == "kv" && l.Key == key {
			out = append(out, l.Value)
		}
	}
	return out
}

// SetAll replaces every line for key with values, placed where the first
// existing line was (or appended). Empty values removes the key entirely.
// Lines for other keys are never touched.
func (f *File) SetAll(key string, values []string) {
	at := -1
	kept := make([]Line, 0, len(f.Lines)+len(values))
	for _, l := range f.Lines {
		if l.Kind == "kv" && l.Key == key {
			if at < 0 {
				at = len(kept)
			}
			continue
		}
		kept = append(kept, l)
	}
	if at < 0 {
		at = len(kept)
	}
	repl := make([]Line, 0, len(values))
	for _, v := range values {
		repl = append(repl, Line{Kind: "kv", Key: key, Value: v, Raw: key + " = " + v})
	}
	out := append(append(append([]Line{}, kept[:at]...), repl...), kept[at:]...)
	f.Lines = out
	if len(f.Lines) > 0 && !f.TrailingNL && at == len(kept) && len(values) > 0 {
		f.TrailingNL = true
	}
}

// Keys returns each distinct key present, in first-seen order.
func (f *File) Keys() []string {
	seen := map[string]bool{}
	var out []string
	for _, l := range f.Lines {
		if l.Kind == "kv" && !seen[l.Key] {
			seen[l.Key] = true
			out = append(out, l.Key)
		}
	}
	return out
}

// Marshal renders the file byte-for-byte except for managed edits.
func (f *File) Marshal() string {
	if len(f.Lines) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, l := range f.Lines {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(l.Raw)
	}
	if f.TrailingNL {
		sb.WriteString("\n")
	}
	return sb.String()
}

// Values returns managed-key values present in the file.
func (f *File) Values(keys []string) map[string]string {
	m := map[string]string{}
	for _, k := range keys {
		if v := f.Get(k); v != "" {
			m[k] = v
		}
	}
	return m
}

// ResolveConfigPath resolves per AGENTS.md order.
func ResolveConfigPath() string {
	return resolveWithEnv(runtime.GOOS, os.Getenv("XDG_CONFIG_HOME"), homeDir(), fileExists, true)
}

// ResolveConfigPathForVersion resolves per AGENTS.md order, preferring plain
// config for new files on Ghostty older than 1.2.3. Unknown versions (ok=false)
// prefer config.ghostty. An existing file always wins so no second file is
// created next to the one Ghostty actually reads.
func ResolveConfigPathForVersion(ok bool, major, minor, patch int) string {
	prefer := true
	if ok {
		prefer = ghostty.AtLeast123(major, minor, patch)
	}
	return resolveWithEnv(runtime.GOOS, os.Getenv("XDG_CONFIG_HOME"), homeDir(), fileExists, prefer)
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return "~"
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func resolveWithEnv(goos, xdg, home string, exists func(string) bool, preferGhostty bool) string {
	xdgBase := xdg
	if xdgBase == "" {
		if goos == "darwin" {
			xdgBase = ""
		} else {
			xdgBase = filepath.Join(home, ".config")
		}
	}
	var candidates []string
	if goos == "darwin" {
		macDir := filepath.Join(home, "Library", "Application Support", "com.mitchellh.ghostty")
		if xdg != "" {
			candidates = append(candidates,
				filepath.Join(xdgBase, "ghostty", "config.ghostty"),
				filepath.Join(xdgBase, "ghostty", "config"))
		}
		candidates = append(candidates,
			filepath.Join(macDir, "config.ghostty"),
			filepath.Join(macDir, "config"))
		candidates = append(candidates,
			filepath.Join(home, ".config", "ghostty", "config.ghostty"),
			filepath.Join(home, ".config", "ghostty", "config"))
	} else {
		candidates = append(candidates,
			filepath.Join(xdgBase, "ghostty", "config.ghostty"),
			filepath.Join(xdgBase, "ghostty", "config"))
	}
	for _, c := range candidates {
		if exists(c) {
			return c
		}
	}
	// Prefer config.ghostty for new files on recent Ghostty, else plain config.
	if !preferGhostty {
		return candidates[1]
	}
	return candidates[0]
}

// AtomicWrite writes via same-dir temp file + rename, preserving perms
// when perm is 0.
func AtomicWrite(path, content string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if perm == 0 {
		if fi, err := os.Stat(path); err == nil {
			perm = fi.Mode().Perm()
		} else {
			perm = 0o644
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// Backup captures one-level undo state for path.
type Backup struct {
	Path    string
	Data    []byte
	Existed bool
	Mode    os.FileMode
}

// Snapshot records current file bytes (missing file = remove on restore).
func Snapshot(path string) (*Backup, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Backup{Path: path, Existed: false}, nil
		}
		return nil, err
	}
	var mode os.FileMode
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	return &Backup{Path: path, Data: append([]byte(nil), b...), Existed: true, Mode: mode}, nil
}

// Restore puts the snapshot bytes back (or removes a missing-file path).
func (s *Backup) Restore() error {
	if !s.Existed {
		err := os.Remove(s.Path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	perm := s.Mode
	if perm == 0 {
		perm = 0o644
	}
	return AtomicWrite(s.Path, string(s.Data), perm)
}
