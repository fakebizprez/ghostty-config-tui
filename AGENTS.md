# AGENTS.md — ghostty-config-tui

## Project

Fullscreen terminal UI for editing Ghostty terminal config with live preview and safe writes.
Single static binary. Runs wherever Ghostty runs (Linux + macOS).

## Stack (decided)

- Language: Go (recent stable, currently Go 1.23+)
- TUI: Bubble Tea v2 (Elm/MVU), Bubbles (pickers/lists), Lip Gloss v2 (styling), Huh (forms)
- No Zig, Swift, Objective-C, C++, or Rust for this tool (see Scope notes)
- Distribute as `ghostty-config-tui` single binary; no runtime required

### Why this stack

- Ghostty config is a flat `key = value` file. This app is forms + search + validation, which Huh covers.
- Go cross-compiles cleanly to Linux/macOS and starts fast.
- Charm ecosystem (Tea + Bubbles + Gloss + Huh) is the fastest path to a polished config editor.

## Inspiration (what to borrow)

### From `ajr-khll/ghostty-config-cli` (Ink/TS TUI)

Borrow the interaction model:

- Fullscreen paned layout: sidebar navigation + focused detail editor + live preview
- Alternate screen buffer (restore scrollback on exit)
- Live preview: mock terminal with theme colors, 16-color palette, cursor style/blink
- Safe surgical writes: only managed keys rewritten, everything else preserved byte-for-byte
- Atomic writes via temp file + rename; never leave a half-written config
- One-level undo (`u` restores exact previous file)
- Smart pickers over `ghostty +list-themes` and `ghostty +list-fonts`, type-to-filter
- Accept custom values with warnings for unknown themes or uninstalled fonts
- Degrade gracefully when `ghostty` is not on `PATH`
- CLI flags: `--print`, `--dry-run`, `--version`, `--help`
- Keys: `↑/↓` or `k/j` navigate, `Enter` edit, `Esc` finish/cancel, `s` save, `u` undo, `r`/`R` reset field/all, `q` quit

### From `zerebos/ghostty-config` (Svelte web generator)

Borrow the data model direction:

- Typed setting schema as source of truth (key, type, default, docs, platform, min version)
- Export/track only values that differ from Ghostty defaults to keep files minimal
- Categorized settings with inline docs per setting
- Per-setting badges: platform (macOS/Linux/GTK) and minimum Ghostty version
- Rich inputs per type: sliders + linked numbers, color pickers, durations, repeatable list editor
- Keybinding builder with validation (defer full builder to post-v1)
- Import/export: paste, upload, copy, download
- Sidebar search across all settings

## Scope

### v1 managed settings

`theme`, `font-family`, `font-size`, `background`, `foreground`, `background-opacity`, `cursor-style`, `cursor-style-blink`

Everything else in the file is preserved untouched.

### v1 screens

1. Sidebar list of managed settings with modified indicators + search filter
2. Detail editor for selected setting (Huh field)
3. Live preview pane (colors, palette, cursor)
4. Status bar: file path, dirty state, last save/undo, key hints

### Non-goals for v1

- Full keybinding builder (preserve keybinding lines verbatim in v1)
- Plugin system, network sync, URL sharing
- Native macOS preference pane (this stays a terminal app)

### Out-of-scope languages

Do not introduce Zig, Swift, Objective-C, C++, or Rust modules. Zig matches Ghostty internals but has no mature form TUI stack. Swift/ObjC split the app per OS. C++ slows UI iteration with no gain here. Rust + Ratatui is a valid alternative stack, but this repo has chosen Go + Tea; do not add a second stack.

## Config file handling (hard rules)

- Format: plain-text `key = value`, one per line, `#` comments, case-sensitive keys.
- Location resolution order:
  - Linux: `$XDG_CONFIG_HOME/ghostty/config.ghostty`, fallback `~/.config/ghostty/config.ghostty`, fallback `.../config` if that is what exists
  - macOS: `~/Library/Application Support/com.mitchellh.ghostty/config.ghostty` (honor `XDG_CONFIG_HOME` if set), fallback to `~/.config/ghostty/...`, prefer `config.ghostty` on Ghostty >= 1.2.3, use plain `config` if that is what exists
- Parser must round-trip: preserve comments, blank lines, ordering, and unmanaged keys byte-for-byte.
- Writer rewrites only managed keys. Atomic write only. Keep file permissions.
- After save, tell the user to reload Ghostty with `ctrl+shift+r`.

## Repo layout (expected)

- `main.go`: entrypoint, flag parsing, Tea program startup
- `internal/config/`: path resolution, parse, marshal, atomic write, undo backup
- `internal/schema/`: setting definitions (key, type, default, docs, platform, min version)
- `internal/ui/`: Tea model/update/view, sidebar, editor, preview, status bar
- `internal/ghostty/`: exec wrappers for `ghostty +list-themes`, `+list-fonts` with graceful fallback
- `testdata/`: golden config files for parser/writer round-trip tests

## Commands

```sh
go run . --help
go run . --print
go run . --dry-run
go build -o ghostty-config-tui .
go test ./...
go vet ./...
gofmt -l .
```

## Conventions

- Bubble Tea v2 Elm structure: `Model`, `Init`, `Update`, `View`. Keep messages typed per screen.
- Huh for form fields; Bubbles list/textinput for pickers and search; Lip Gloss for all styling (no raw ANSI).
- Every setting comes from `internal/schema`, not hardcoded in UI code.
- Handle `ghostty` binary missing: pickers fall back to manual input with a warning.
- Errors are shown in-app in the status bar, never panic to a broken alternate screen.

## Tests (required for changes)

- Parser round-trip test: unmanaged lines, comments, and ordering survive unchanged (golden files in `testdata/`).
- Writer test: changing one managed key does not touch other lines; atomic write path covered.
- Schema test: every v1 managed key has type, default, and help text.
- Run `go test ./...` and `go vet ./...` before committing.

## Git

Work on branches, never directly on `main`. Keep diffs focused; one behavior per commit.
