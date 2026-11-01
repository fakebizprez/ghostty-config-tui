// Command ghostty-config-tui is a fullscreen TUI for editing Ghostty config.
package main

import (
	"fmt"
	"io"
	"os"
	"sort"

	"ghostty-config-tui/internal/config"
	"ghostty-config-tui/internal/ghostty"
	"ghostty-config-tui/internal/schema"
	"ghostty-config-tui/internal/ui"
)

const version = "0.2.0"

type flags struct {
	print   bool
	export  bool
	share   bool
	dryRun  bool
	version bool
	help    bool
	config  string
}

func parseFlags(args []string) (flags, []string, error) {
	var f flags
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--print":
			f.print = true
		case a == "--export":
			f.export = true
		case a == "--share":
			f.share = true
		case a == "--dry-run":
			f.dryRun = true
		case a == "--version":
			f.version = true
		case a == "--help" || a == "-h":
			f.help = true
		case a == "--config":
			if i+1 >= len(args) {
				return f, nil, fmt.Errorf("--config needs a path")
			}
			i++
			f.config = args[i]
		case len(a) > 9 && a[:9] == "--config=":
			f.config = a[9:]
		case a == "--":
		case len(a) > 1 && a[0] == '-':
			return f, nil, fmt.Errorf("unknown flag %q", a)
		default:
			rest = append(rest, a)
		}
	}
	return f, rest, nil
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "ghostty-config-tui — fullscreen TUI for every Ghostty setting")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage: ghostty-config-tui [--config PATH] [--print] [--export] [--share] [--dry-run] [--version] [--help]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  --config PATH  edit this file instead of the resolved Ghostty config")
	fmt.Fprintln(w, "  --print        print every Ghostty setting set in the file and exit")
	fmt.Fprintln(w, "  --export       print a minimal config grouped by category and exit")
	fmt.Fprintln(w, "  --share        print a ghostty.zerebos.com link that opens this config and exit")
	fmt.Fprintln(w, "  --dry-run      edit without writing; saves are previewed only")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Keys: ↑/↓ move, ←/→ pane, enter edit, / search, s save, u undo,")
	fmt.Fprintln(w, "      r/R revert, d default, i import, x export, ? help, q quit")
	fmt.Fprintln(w, "After saving, reload Ghostty with ctrl+shift+r.")
}

// loadValues returns every known Ghostty key present in the file.
func loadValues(f *config.File) map[string][]string {
	out := map[string][]string{}
	for _, k := range f.Keys() {
		if _, ok := schema.ByKey(k); ok {
			out[k] = f.GetAll(k)
		}
	}
	return out
}

// printConfig prints known keys set in the file, in schema order.
func printConfig(w io.Writer, path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	f, err := config.ParseFile(path)
	if err != nil {
		return err
	}
	vals := loadValues(f)
	for _, s := range schema.All() {
		for _, v := range vals[s.Key] {
			fmt.Fprintf(w, "%s = %s\n", s.Key, v)
		}
	}
	return nil
}

// startProgram is stubbed in tests.
var startProgram = func(opts ui.Options) error { return ui.Run(opts) }

// loadSchema swaps in the installed Ghostty's own option docs when possible.
var loadSchema = func() string {
	docs, ok := ghostty.DefaultDocs()
	if !ok {
		return "ghostty not found — using the bundled Ghostty 1.4 option list; pickers accept typed values"
	}
	actions, _ := ghostty.Actions()
	schema.Use(docs, actions)
	return ""
}

func resolvePath(override string) string {
	if override != "" {
		return override
	}
	maj, min, pat, ok := ghostty.Version()
	return config.ResolveConfigPathForVersion(ok, maj, min, pat)
}

// newSession wires save/undo to one config file. Each save snapshots the
// file first so undo restores exactly what was there before that save.
func newSession(path string, dryRun bool) (ui.Options, error) {
	cf, err := config.ParseFile(path)
	if err != nil {
		return ui.Options{}, err
	}
	var backup *config.Backup
	opts := ui.Options{ConfigPath: path, Values: loadValues(cf), DryRun: dryRun}
	opts.OnSave = func(changed map[string][]string) ([]string, error) {
		keys := make([]string, 0, len(changed))
		for k := range changed {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		next := &config.File{Lines: append([]config.Line(nil), cf.Lines...), TrailingNL: cf.TrailingNL}
		for _, k := range keys {
			next.SetAll(k, changed[k])
		}
		if dryRun {
			*cf = *next
			return nil, nil
		}
		snap, err := config.Snapshot(path)
		if err != nil {
			return nil, err
		}
		if err := config.AtomicWrite(path, next.Marshal(), 0); err != nil {
			return nil, err // cf untouched: a later save cannot resurrect this change
		}
		*cf = *next
		backup = snap
		problems, _ := ghostty.Validate(path)
		return problems, nil
	}
	opts.OnUndo = func() (map[string][]string, error) {
		if backup == nil {
			return nil, fmt.Errorf("no save to undo yet")
		}
		if err := backup.Restore(); err != nil {
			return nil, err
		}
		backup = nil
		fresh, err := config.ParseFile(path)
		if err != nil {
			return nil, err
		}
		*cf = *fresh
		return loadValues(cf), nil
	}
	return opts, nil
}

func run(stdout, stderr io.Writer, args []string, isTTY bool) int {
	f, _, err := parseFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		usage(stderr)
		return 2
	}
	if f.help {
		usage(stdout)
		return 0
	}
	if f.version {
		fmt.Fprintln(stdout, "ghostty-config-tui "+version)
		return 0
	}
	warning := loadSchema()
	path := resolvePath(f.config)
	if f.print || (f.dryRun && !isTTY) {
		if err := printConfig(stdout, path); err != nil {
			fmt.Fprintln(stderr, "print: "+err.Error())
			return 1
		}
		if !f.print {
			fmt.Fprintln(stdout, "# dry-run: no changes written")
		}
		return 0
	}
	if f.share {
		opts, err := newSession(path, true)
		if err != nil {
			fmt.Fprintln(stderr, "share: "+err.Error())
			return 1
		}
		link, ok := ui.New(opts).ShareURL()
		if !ok {
			fmt.Fprintln(stderr, "share: config too large for a share link; use --export")
			return 1
		}
		fmt.Fprintln(stdout, link)
		return 0
	}
	if f.export {
		opts, err := newSession(path, true)
		if err != nil {
			fmt.Fprintln(stderr, "export: "+err.Error())
			return 1
		}
		fmt.Fprint(stdout, ui.New(opts).ExportText())
		return 0
	}
	if !isTTY {
		fmt.Fprintln(stderr, "no TTY: refusing to start fullscreen editor (try --print or --export)")
		return 1
	}
	opts, err := newSession(path, f.dryRun)
	if err != nil {
		fmt.Fprintln(stderr, "load: "+err.Error())
		return 1
	}
	opts.Themes, _ = ghostty.ListThemes()
	opts.Fonts, _ = ghostty.ListFonts()
	opts.Warning = warning
	if err := startProgram(opts); err != nil {
		fmt.Fprintln(stderr, "editor: "+err.Error())
		return 1
	}
	return 0
}

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func main() {
	os.Exit(run(os.Stdout, os.Stderr, os.Args[1:], isTerminal()))
}
