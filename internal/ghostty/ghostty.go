// Package ghostty wraps `ghostty +list-themes` / `+list-fonts` with
// graceful fallback when the binary is missing.
package ghostty

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// DefaultTimeout bounds ghostty subprocess calls.
const DefaultTimeout = 10 * time.Second

// Status reports whether live ghostty data was available.
type Status struct {
	Degraded bool
	Warning  string
}

var (
	lookPath   = exec.LookPath
	runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, name, args...).Output()
	}
)

// ParseThemes parses `Name (source)` lines, handling names with parens,
// trimming, deduping, first-seen order.
func ParseThemes(out string) []string {
	var themes []string
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name := line
		if i := strings.LastIndex(line, " ("); i >= 0 && strings.HasSuffix(line, ")") {
			name = strings.TrimSpace(line[:i])
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		themes = append(themes, name)
	}
	return themes
}

// ParseFontFamilies parses family headers with indented variant faces.
func ParseFontFamilies(out string) []string {
	var fams []string
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		// Indented lines are faces, not families.
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		name := strings.TrimSpace(line)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		fams = append(fams, name)
	}
	return fams
}

func runList(args ...string) ([]byte, Status, error) {
	if _, err := lookPath("ghostty"); err != nil {
		return nil, Status{Degraded: true, Warning: "ghostty not on PATH — type values manually"}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	out, err := runCommand(ctx, "ghostty", args...)
	if err != nil {
		return nil, Status{Degraded: true, Warning: "ghostty list failed — type values manually"}, err
	}
	return out, Status{}, nil
}

// ListThemes returns installed theme names (empty + degraded when missing).
func ListThemes() ([]string, Status) {
	out, st, err := runList("+list-themes")
	if err != nil || st.Degraded {
		return nil, st
	}
	return ParseThemes(string(out)), st
}

// ListFonts returns installed font families (empty + degraded when missing).
func ListFonts() ([]string, Status) {
	out, st, err := runList("+list-fonts")
	if err != nil || st.Degraded {
		return nil, st
	}
	return ParseFontFamilies(string(out)), st
}
