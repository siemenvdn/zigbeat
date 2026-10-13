package show

import (
	"math"
	"slices"
	"strings"

	colorful "github.com/lucasb-eyer/go-colorful"

	"github.com/siemenvdn/zigbeat/internal/music"
)

type palette struct {
	colors   []string
	i        int
	fallback []string
	album    bool
	n        int
	art      []music.RGB
}

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
	minValue       = 0.15
	greySaturation = 0.25
	hueSpacing     = 30
	minSaturation  = 0.75
	fullValue      = 1
	fullCircle     = 360
)

var fillSteps = []float64{hueSpacing, -hueSpacing, 2 * hueSpacing, -2 * hueSpacing, 3 * hueSpacing, -3 * hueSpacing}

type hueSat struct{ h, s float64 }

// AlbumColors turns the colours of an album's art into n hex colours for colour bulbs.
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

func near(kept []hueSat, h float64) bool {
	for _, k := range kept {
		d := math.Abs(k.h - h)
		if min(d, fullCircle-d) < hueSpacing {
			return true
		}
	}
	return false
}
