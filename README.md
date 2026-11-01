# ghostty-config-tui

A fullscreen terminal editor for **every** [Ghostty](https://ghostty.org) setting, with a live preview and safe writes. It's a single static Go binary for macOS and Linux.

## What it does

- **All settings.** The setting list comes from your installed Ghostty's own docs (`ghostty +show-config --default --docs`), so it always matches your version. A bundled snapshot is used when `ghostty` isn't on `PATH`.
- **Categories, search, and docs.** Settings are grouped into categories, and there's a "Modified" view of everything you've set. Search covers both keys and docs. The full Ghostty docs for each setting are shown inline, with platform badges (macOS, Linux, GTK) and the minimum Ghostty version.
- **An editor for each type of setting:**
  - Toggles for true/false settings.
  - Sliders for numbers with known ranges.
  - An RGB color picker that also accepts hex and X11 color names.
  - Filterable pickers for options, themes (`+list-themes`), and fonts (`+list-fonts`).
  - Checkbox lists for comma-separated flag settings. Only flags that differ from the default are written.
  - A list editor for settings that repeat, like `font-family` fallbacks, `font-feature`, `palette`, and `env`.
- **Keybinding builder.** Set prefixes (`global:`, `all:` and the rest), record a trigger by pressing the keys, build key sequences, and pick an action from `+list-actions`. You get live validation and a warning when you override one of Ghostty's default bindings.
- **Live preview.** A mock terminal draws your theme and explicit colors, the 16-color palette, selection colors, and the cursor's style and blink. While you browse themes, the preview follows the highlighted one.
- **Safe writes.** Only keys you changed are rewritten. Comments, unknown keys, and every other line stay byte-for-byte. Writes are atomic and keep file permissions. `u` restores the file exactly as it was before your last save. After each save, Ghostty's `+validate-config` checks the file and reports any problems.
- **Import and export.** Paste a config or load one from a file; the values are staged until you save. Export a minimal config (only values that differ from the defaults) to the clipboard (OSC 52) or to a file.

## Usage

```sh
go build -o ghostty-config-tui .
./ghostty-config-tui                 # edit your Ghostty config
./ghostty-config-tui --config FILE   # edit a specific file
./ghostty-config-tui --dry-run       # edit without writing
./ghostty-config-tui --print         # print set values and exit
./ghostty-config-tui --export        # print a minimal config and exit
```

| Key | Action |
| --- | --- |
| `↑/↓` `j/k` | move |
| `←/→` `h/l` `tab` | switch between categories and settings |
| `enter` | edit (toggles true/false settings, opens the list editor for repeating ones) |
| `space` | toggle a true/false setting |
| `/` | search keys and docs |
| `s` / `u` | save / undo last save |
| `r` / `R` | revert field / all unsaved changes |
| `d` | reset to Ghostty default (removes the line) |
| `i` / `x` | import / export |
| `p` | toggle preview |
| `?` | help |
| `q` | quit (asks again if there are unsaved changes) |

After saving, reload Ghostty with `ctrl+shift+r`.

Config location: `$XDG_CONFIG_HOME/ghostty/config.ghostty` on Linux (or `~/.config/ghostty/…`), and `~/Library/Application Support/com.mitchellh.ghostty/config.ghostty` on macOS (`XDG_CONFIG_HOME` is honored). A plain `config` file is used if that's what exists.
