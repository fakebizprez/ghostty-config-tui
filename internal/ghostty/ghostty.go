// Package ghostty wraps `ghostty +list-themes` / `+list-fonts` with
// graceful fallback when the binary is missing.
package ghostty

import (
	"context"
	"os/exec"
	"strconv"
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

// ParseVersion extracts major.minor.patch from `ghostty --version` output
// such as "Ghostty 1.4.1-main+9781c0afc". Missing parts default to zero.
func ParseVersion(out string) (major, minor, patch int, ok bool) {
	for _, f := range strings.Fields(out) {
		f = strings.TrimPrefix(f, "v")
		if i := strings.IndexAny(f, "-+"); i >= 0 {
			f = f[:i]
		}
		parts := strings.Split(f, ".")
		if len(parts) < 2 || len(parts) > 3 {
			continue
		}
		var nums []int
		valid := true
		for _, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil || n < 0 {
				valid = false
				break
			}
			nums = append(nums, n)
		}
		if !valid {
			continue
		}
		for len(nums) < 3 {
			nums = append(nums, 0)
		}
		return nums[0], nums[1], nums[2], true
	}
	return 0, 0, 0, false
}

// AtLeast123 reports whether a Ghostty version is at least 1.2.3.
func AtLeast123(major, minor, patch int) bool {
	if major != 1 {
		return major > 1
	}
	if minor != 2 {
		return minor > 2
	}
	return patch >= 3
}

// Version runs `ghostty --version`. ok is false when the binary is missing,
// the call fails, or the output is unparseable.
func Version() (major, minor, patch int, ok bool) {
	if _, err := lookPath("ghostty"); err != nil {
		return 0, 0, 0, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	out, err := runCommand(ctx, "ghostty", "--version")
	if err != nil {
		return 0, 0, 0, false
	}
	return ParseVersion(string(out))
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

func runGhostty(args ...string) (string, bool) {
	if _, err := lookPath("ghostty"); err != nil {
		return "", false
	}
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	out, err := runCommand(ctx, "ghostty", args...)
	if err != nil {
		return "", false
	}
	return string(out), true
}

// DefaultDocs returns `ghostty +show-config --default --docs`, the source of
// the setting schema for the installed version.
func DefaultDocs() (string, bool) { return runGhostty("+show-config", "--default", "--docs") }

// Actions returns `ghostty +list-actions` for the keybind builder.
func Actions() (string, bool) { return runGhostty("+list-actions") }

// Validate runs `ghostty +validate-config` against a file and returns its
// diagnostics. ok is false when Ghostty is unavailable.
func Validate(path string) (problems []string, ok bool) {
	if _, err := lookPath("ghostty"); err != nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	out, err := validateCommand(ctx, path)
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l = strings.TrimPrefix(strings.TrimSpace(l), path+":"); l != "" {
			problems = append(problems, l)
		}
	}
	if err != nil && len(problems) == 0 {
		problems = []string{err.Error()}
	}
	return problems, true
}

var validateCommand = func(ctx context.Context, path string) ([]byte, error) {
	return exec.CommandContext(ctx, "ghostty", "+validate-config", "--config-file="+path).CombinedOutput()
}

// ShowFace runs `ghostty +show-face --string=sample` with the given
// font-family overrides and returns one line per codepoint.
func ShowFace(sample string, families []string) ([]string, bool) {
	args := []string{"+show-face", "--string=" + sample}
	for _, f := range families {
		args = append(args, "--font-family="+f)
	}
	out, ok := runGhostty(args...)
	if !ok {
		return nil, false
	}
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	return lines, true
}
