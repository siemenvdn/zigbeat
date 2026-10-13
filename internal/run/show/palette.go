package show

import (
	"math"
	"slices"
	"strings"

	colorful "github.com/lucasb-eyer/go-colorful"

	"github.com/siemenvdn/zigbeat/internal/music"
)

// palette is the colours that play and whose turn is next.
type palette struct {
	colors   []string    // the colours that play
	i        int         // the next colour to try
	fallback []string    // effects.palette, without album colours
	album    bool        // effects.album_palette
	n        int         // effects.album_colors
	art      []music.RGB // the last track's album colours, as the source sent them
}

// choose plays the album colours of the last track when they are on and the
// track has them, else effects.palette. A new palette restarts the index;
// lights keep their colour until their turn. It returns a note when the palette changed.
func (pl *palette) choose() string {
	colors, note := pl.fallback, "album colours off; the palette plays"
	if pl.album {
		note = "no album colours; the palette plays"
		if c := AlbumColors(pl.art, pl.n); c != nil {
			colors, note = c, "album colours "+strings.Join(c, " ")
		}
	}
	if slices.Equal(colors, pl.colors) {
		return ""
	}
	pl.colors, pl.i = colors, 0
	return note
}

// take returns the next colour that is not in avoid, and moves past it.
func (pl *palette) take(avoid map[string]bool) (string, bool) {
	for i := range pl.colors {
		c := pl.colors[(pl.i+i)%len(pl.colors)]
		if !avoid[c] {
			pl.i = (pl.i + i + 1) % len(pl.colors)
			return c, true
		}
	}
	return "", false
}

const (
	minValue       = 0.15 // below it the hue of a cover is noise: JPEG artefacts, text outlines
	greySaturation = 0.25 // below it white, grey or black; Music Assistant's Hue plugin draws the line here
	hueSpacing     = 30   // degrees: closer hues look like one colour on a bulb
	minSaturation  = 0.75 // vivid: below it a bulb shows a pastel tint of white
	fullValue      = 1    // Zigbee2MQTT sets brightness apart; a hex colour only brings hue and saturation
	fullCircle     = 360  // degrees of hue
)

// fillSteps are the neighbours of the dominant hue that fill up a short list, nearest first.
var fillSteps = []float64{hueSpacing, -hueSpacing, 2 * hueSpacing, -2 * hueSpacing, 3 * hueSpacing, -3 * hueSpacing}

type hueSat struct{ h, s float64 }

// AlbumColors turns the colours of an album's art into n hex colours for
// colour bulbs: greys and near-black dropped, hues at least hueSpacing apart,
// saturation lifted to minSaturation, filled up with neighbours of the most
// dominant hue. It returns nil when no colour is left.
func AlbumColors(p []music.RGB, n int) []string {
	var kept []hueSat
	for _, c := range p {
		h, s, v := colorful.Color{R: float64(c[0]) / math.MaxUint8, G: float64(c[1]) / math.MaxUint8, B: float64(c[2]) / math.MaxUint8}.Hsv()
		h = math.Mod(math.Round(h), fullCircle)
		if len(kept) == n {
			break
		}
		if v >= minValue && s >= greySaturation && !near(kept, h) {
			kept = append(kept, hueSat{h, max(s, minSaturation)})
		}
	}
	if len(kept) == 0 {
		return nil
	}
	dominant := kept[0]
	for _, step := range fillSteps {
		if h := math.Mod(dominant.h+step+fullCircle, fullCircle); len(kept) < n && !near(kept, h) {
			kept = append(kept, hueSat{h, dominant.s})
		}
	}
	out := make([]string, len(kept))
	for i, k := range kept {
		out[i] = colorful.Hsv(k.h, k.s, fullValue).Hex()
	}
	return out
}

// near reports whether a kept hue lies within hueSpacing of h.
func near(kept []hueSat, h float64) bool {
	for _, k := range kept {
		d := math.Abs(k.h - h)
		if min(d, fullCircle-d) < hueSpacing {
			return true
		}
	}
	return false
}
