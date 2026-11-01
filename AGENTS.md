# AGENTS.md — ghostty-config-tui

## Project

Fullscreen terminal UI for editing Ghostty terminal config with live preview and safe writes.
Single static binary. Runs wherever Ghostty runs (Linux + macOS).

## Stack (decided)

- Language: Go (recent stable, currently Go 1.23+)
- TUI: Bubble Tea v2 (Elm/MVU) and Lip Gloss v2 (styling); editors are small hand-built widgets in `internal/ui/editor.go`
- No Zig, Swift, Objective-C, C++, or Rust for this tool (see Scope notes)
- Distribute as `ghostty-config-tui` single binary; no runtime required

### Why this stack

- Ghostty config is a flat `key = value` file. This app is typed fields + search + validation; per-kind widgets on Tea cover it without a forms library.
- Go cross-compiles cleanly to Linux/macOS and starts fast.
- Charm ecosystem (Tea + Lip Gloss) is the fastest path to a polished config editor.

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
- Keys: `↑/↓` or `k/j` navigate, `Enter` edit, `Esc` finish/cancel, `s` save, `u` undo, `r`/`R` revert field/all, `d` reset to default, `/` search, `i`/`x` import/export, `q` quit

### From `zerebos/ghostty-config` (Svelte web generator)

Borrow the data model direction:

- Typed setting schema as source of truth (key, type, default, docs, platform, min version)
- Export/track only values that differ from Ghostty defaults to keep files minimal
- Categorized settings with inline docs per setting
- Per-setting badges: platform (macOS/Linux/GTK) and minimum Ghostty version
- Rich inputs per type: sliders + linked numbers, color pickers, durations, repeatable list editor
- Keybinding builder with validation (prefixes, recorded triggers, sequences, action picker)
- Import/export: paste, upload, copy, download
- Sidebar search across all settings

## Scope

### Managed settings: all of them

The schema is generated from Ghostty itself (`ghostty +show-config --default --docs`),
so every option the installed Ghostty knows is editable, with its default, full docs,
platform badge (macOS / Linux / GTK), and "Available since" version. A snapshot
(`internal/schema/ghostty-docs.txt`, `ghostty-actions.txt`) is embedded for when
Ghostty is not on `PATH`. Refresh it with the commands at the top of `internal/schema/schema.go`.

Kinds are inferred from defaults and docs, with small override tables for the few
options Ghostty only describes in prose: bool, int, float (sliders for known ranges),
color (RGB picker, hex, X11 names, special values), enum (filterable picker), flags
(checkbox list, writes only differences from the default), duration, theme picker,
font picker, palette, keybind (builder), and repeatable lists (list editor).

Keys Ghostty does not know are preserved untouched.

### Screens

1. Categories pane (with a "Modified" view) and a settings pane with search across keys and docs
2. Detail pane: badges, value vs. default, the type-specific editor, wrapped docs
3. Live preview: mock terminal using theme + explicit colors, palette, selection, cursor style and blink; the theme picker previews the highlighted theme
4. Status bar: dirty count, messages, Ghostty validation results, key hints
5. Dialogs: help, import (paste, path, or share link), export (clipboard via OSC 52, file, or a ghostty.zerebos.com share link), font playground (`ghostty +show-face` against staged fonts)
6. App icon preview on `macos-icon*` settings (real icon images via Kitty graphics placeholders in Ghostty/kitty, block art elsewhere); type-into preview prompt (`t`)

### Non-goals

- Plugin system, network sync
- Per-glyph fallback inside the playground image (it draws one font file); the `+show-face` table covers fallback. Shaping uses go-text/typesetting, which misses some variable-font ligatures (e.g. Cascadia Code) that HarfBuzz applies
- Native macOS preference pane (this stays a terminal app)

### Out-of-scope languages

Do not introduce Zig, Swift, Objective-C, C++, or Rust modules. Zig matches Ghostty internals but has no mature form TUI stack. Swift/ObjC split the app per OS. C++ slows UI iteration with no gain here. Rust + Ratatui is a valid alternative stack, but this repo has chosen Go + Tea; do not add a second stack.

## Config file handling (hard rules)

- Format: plain-text `key = value`, one per line, `#` comments, case-sensitive keys.
- Location resolution order:
  - Linux: `$XDG_CONFIG_HOME/ghostty/config.ghostty`, fallback `~/.config/ghostty/config.ghostty`, fallback `.../config` if that is what exists
  - macOS: `~/Library/Application Support/com.mitchellh.ghostty/config.ghostty` (honor `XDG_CONFIG_HOME` if set), fallback to `~/.config/ghostty/...`, prefer `config.ghostty` on Ghostty >= 1.2.3, use plain `config` if that is what exists
- Parser must round-trip: preserve comments, blank lines, ordering, and unmanaged keys byte-for-byte.
- Writer rewrites only keys changed in the session (repeatable keys stay where their first line was). Atomic write only. Keep file permissions. Undo restores the bytes from before the last save.
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
go run . --export
go run . --config ./testdata/full.conf
go build -o ghostty-config-tui .
go test ./...
go vet ./...
gofmt -l .
```

## Conventions

- Bubble Tea v2 Elm structure: `Model`, `Init`, `Update`, `View`. Keep messages typed per screen.
- Editors live in `internal/ui/editor.go`, one widget per schema kind; Lip Gloss for all styling (no raw ANSI).
- Every setting comes from `internal/schema`, not hardcoded in UI code.
- Handle `ghostty` binary missing: pickers fall back to manual input with a warning.
- Errors are shown in-app in the status bar, never panic to a broken alternate screen.
- Images go through `internal/graphics` (Kitty graphics protocol with Unicode placeholders, so they are plain cells to the renderer); disabled under tmux/screen with a text fallback.

## Tests (required for changes)

- Parser round-trip test: unmanaged lines, comments, and ordering survive unchanged (golden files in `testdata/`).
- Schema test: every parsed setting has a kind, category, and docs; spot-check inferred kinds after refreshing the snapshot (`GHOSTTY_SCHEMA_DUMP=1 go test ./internal/schema -run TestDump -v`).
- Writer test: changing one managed key does not touch other lines; atomic write path covered.
- Run `go test ./...` and `go vet ./...` before committing.

## Git

Work on branches, never directly on `main`. Keep diffs focused; one behavior per commit.
