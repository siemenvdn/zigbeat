package beats

import (
	"fmt"
	"strings"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/run/show"
)

func trackLine(t music.Track, e config.Effects) string {
	s := fmt.Sprintf("track %.1f BPM, ", t.BPM)
	colors := show.AlbumColors(t.Palette, e.AlbumColors)
	switch {
	case !e.AlbumPalette:
		s += "album colours off in the config; the palette plays"
	case colors == nil:
		s += "no album colours; the palette plays"
	default:
		s += "album colours " + strings.Join(colors, " ")
	}
	if len(t.Palette) > 0 {
		s += fmt.Sprintf(" (Music Assistant sent %d)", len(t.Palette))
	}
	return s
}
