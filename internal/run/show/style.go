package show

import (
	"slices"
	"strings"

	"github.com/siemenvdn/zigbeat/internal/config"
)

// SetEffects takes the style of the show from the next moment on: the album
// colours, the variant, the brightness and the pace. The same style again
// changes nothing.
func (p *Planner) SetEffects(e config.Effects) {
	p.fallback, p.album, p.albumColors = e.Palette, e.AlbumPalette, e.AlbumColors
	p.setting, p.cadence = e.Variant, e.Cadence
	p.base = level(e.Brightness)
	p.kick = level(min(e.Brightness+kickPoints, config.MaxPercent))
	p.choose()
}

// choose plays the album colours of the last track when they are on and the
// track has them, else effects.palette. A new palette restarts the palette
// index; lights keep their colour until their turn.
func (p *Planner) choose() {
	pal, note := p.fallback, "album colours off; the palette plays"
	if p.album {
		note = "no album colours; the palette plays"
		if c := AlbumColors(p.art, p.albumColors); c != nil {
			pal, note = c, "album colours "+strings.Join(c, " ")
		}
	}
	if slices.Equal(pal, p.palette) {
		return
	}
	p.palette, p.pi = pal, 0
	p.notes = append(p.notes, note)
}
