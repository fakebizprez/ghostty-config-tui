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
