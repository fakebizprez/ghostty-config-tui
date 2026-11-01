// Package graphics draws real images in terminals that speak the Kitty
// graphics protocol (Ghostty, kitty) using Unicode placeholders, so images
// are ordinary text cells to the TUI renderer.
package graphics

import (
	"bytes"
	"image"
	"os"
	"strconv"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi/kitty"
)

// Supported reports whether the current terminal can show images. tmux and
// screen do not pass Kitty graphics through, so they fall back to text.
func Supported() bool { return supported(os.Getenv) }

func supported(env func(string) string) bool {
	if env("TMUX") != "" || strings.HasPrefix(env("TERM"), "screen") || strings.HasPrefix(env("TERM"), "tmux") {
		return false
	}
	t := env("TERM")
	return env("TERM_PROGRAM") == "ghostty" || strings.Contains(t, "ghostty") || strings.Contains(t, "kitty") || env("KITTY_WINDOW_ID") != ""
}

// Transmit uploads img as image id with a virtual placement of cols x rows
// cells. Sending the same id again replaces the image.
func Transmit(id int, img image.Image, cols, rows int) string {
	var b bytes.Buffer
	_ = kitty.EncodeGraphics(&b, img, &kitty.Options{
		Action:           kitty.TransmitAndPut,
		Transmission:     kitty.Direct,
		Format:           kitty.PNG,
		ID:               id,
		VirtualPlacement: true,
		Columns:          cols,
		Rows:             rows,
		Quiet:            2,
		Chunk:            true,
	})
	return b.String()
}

// Placeholder returns rows of placeholder cells that display image id. The
// id travels in the 256-color foreground (38;5;id), so keep ids in 16..255;
// lower ids would be emitted as basic ANSI colors.
func Placeholder(id, cols, rows int) []string {
	st := lipgloss.NewStyle().Foreground(lipgloss.Color(strconv.Itoa(id)))
	out := make([]string, rows)
	for r := 0; r < rows; r++ {
		var sb strings.Builder
		for c := 0; c < cols; c++ {
			sb.WriteRune(kitty.Placeholder)
			sb.WriteRune(kitty.Diacritic(r))
			sb.WriteRune(kitty.Diacritic(c))
		}
		out[r] = st.Render(sb.String())
	}
	return out
}
