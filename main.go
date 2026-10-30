// Command ghostty-config-tui is a fullscreen TUI for editing Ghostty config.
package main

import (
	"fmt"
	"io"
	"os"

	"ghostty-config-tui/internal/config"
	"ghostty-config-tui/internal/ghostty"
	"ghostty-config-tui/internal/schema"
	"ghostty-config-tui/internal/ui"
)

const version = "0.1.0"

type flags struct {
	print   bool
	dryRun  bool
	version bool
	help    bool
}

func parseFlags(args []string) (flags, []string, error) {
	var f flags
	var rest []string
	for _, a := range args {
		switch a {
		case "--print":
			f.print = true
		case "--dry-run":
			f.dryRun = true
		case "--version":
			f.version = true
		case "--help", "-h":
			f.help = true
		case "--":
			continue
		default:
			if len(a) > 1 && a[0] == '-' {
				return f, nil, fmt.Errorf("unknown flag %q", a)
			}
			rest = append(rest, a)
		}
	}
	return f, rest, nil
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "ghostty-config-tui — fullscreen TUI for Ghostty config")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage: ghostty-config-tui [--print] [--dry-run] [--version] [--help]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Keys: ↑/↓ or k/j navigate, Enter edit, Esc finish/cancel,")
	fmt.Fprintln(w, "      s save, u undo, r/R reset field/all, q quit")
	fmt.Fprintln(w, "After save, reload Ghostty with ctrl+shift+r.")
}

// printConfig prints only managed keys set in the file, in schema order.
func printConfig(w io.Writer, path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	f, err := config.ParseFile(path)
	if err != nil {
		return err
	}
	for _, s := range schema.All() {
		if v := f.Get(s.Key); v != "" {
			fmt.Fprintf(w, "%s = %s\n", s.Key, v)
		}
	}
	return nil
}

// startProgram is stubbed in tests.
var startProgram = func(opts ui.Options) error {
	return ui.Run(opts)
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
	if f.print {
		path := config.ResolveConfigPath()
		if err := printConfig(stdout, path); err != nil {
			fmt.Fprintln(stderr, "print: "+err.Error())
			return 1
		}
		return 0
	}

	path := config.ResolveConfigPath()

	// Dry-run without a TTY degrades to printing what would be edited.
	if f.dryRun && !isTTY {
		if err := printConfig(stdout, path); err != nil {
			fmt.Fprintln(stderr, "dry-run: "+err.Error())
			return 1
		}
		fmt.Fprintln(stdout, "# dry-run: no changes written")
		return 0
	}
	if !isTTY {
		fmt.Fprintln(stderr, "no TTY: refusing to start fullscreen editor")
		return 1
	}

	cf, err := config.ParseFile(path)
	if err != nil {
		fmt.Fprintln(stderr, "load: "+err.Error())
		return 1
	}
	values := cf.Values(managedKeys())

	themes, _ := ghostty.ListThemes()
	fonts, _ := ghostty.ListFonts()

	backup, _ := config.Snapshot(path)

	onSave := func(next map[string]string) error {
		for _, s := range schema.All() {
			if v, ok := next[s.Key]; ok {
				cf.Set(s.Key, v)
			} else {
				// Leave absent keys absent; only rewrite managed keys present.
				_ = s
			}
		}
		if f.dryRun {
			return nil
		}
		return config.AtomicWrite(path, cf.Marshal(), 0)
	}
	onUndo := func() (map[string]string, error) {
		if backup == nil {
			return values, nil
		}
		if err := backup.Restore(); err != nil {
			return nil, err
		}
		fresh, err := config.ParseFile(path)
		if err != nil {
			return nil, err
		}
		*cf = *fresh
		return cf.Values(managedKeys()), nil
	}

	if err := startProgram(ui.Options{
		ConfigPath:   path,
		Values:       values,
		ThemeOptions: themes,
		FontOptions:  fonts,
		OnSave:       onSave,
		OnUndo:       onUndo,
		DryRun:       f.dryRun,
	}); err != nil {
		fmt.Fprintln(stderr, "editor: "+err.Error())
		return 1
	}
	return 0
}

func managedKeys() []string {
	keys := []string{}
	for _, s := range schema.All() {
		keys = append(keys, s.Key)
	}
	return keys
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
