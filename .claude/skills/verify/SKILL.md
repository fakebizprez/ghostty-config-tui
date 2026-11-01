---
name: verify
description: Build and drive ghostty-config-tui in an isolated tmux server against a scratch config to verify changes at the terminal surface.
---

# Verify ghostty-config-tui

Build: `go build -o ghostty-config-tui .`

Always pass `--config <scratch file>` so your real Ghostty config is never touched.

## Drive it in tmux (isolated server)

```sh
tmux -L gctv new-session -d -s v -x 130 -y 38 "env COLORTERM=truecolor ./ghostty-config-tui --config /tmp/x/c.conf; echo EXIT=\$?; sleep 30"
tmux -L gctv send-keys -t v /            # one key per send-keys, ~0.35s apart
tmux -L gctv send-keys -t v -l -- "-calt" # literal text; `--` so tmux doesn't parse it as a flag
tmux -L gctv capture-pane -t v -p        # screen; add -e for SGR colors
tmux -L gctv capture-pane -t v -p | tail -2 | head -1   # status line
tmux -L gctv kill-server
```

Import paste: `tmux -L gctv load-buffer f.txt; tmux -L gctv paste-buffer -p -t v` (bracketed; newlines arrive as `\r`).

## Gotchas

- Send `Escape` alone and wait before the next key, or the terminal reads ESC+key as alt+key.
- Text and Enter sent in one burst can arrive as `ctrl+j`; the app maps it to enter.
- In zsh, don't put `tmux -L gctv` in a variable (`$T ...` is not word-split).
- tmux downsamples to 256 colors; compare palette fg codes (e.g. Dracula red = 38;5;203) rather than exact hex.

## Flows worth driving

- Edit each kind (bool space, enum picker, slider ←/→, color channels, flags, list editor a/K/x, keybind builder with ctrl+r), then `s` and diff the file: only changed keys move, unknown keys and comments stay.
- Theme picker: highlight themes and watch preview colors change; an explicit `background` must win over the theme.
- `u` after a save restores the file byte for byte (`diff` against the seed).
- Probes: `--dry-run` save, a read-only config (must refuse), a symlinked config (must stay a symlink), a new file in a missing dir, resize the pane mid-edit.
