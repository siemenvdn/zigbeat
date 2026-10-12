package show

import (
	"math"

	colorful "github.com/lucasb-eyer/go-colorful"

	"github.com/siemenvdn/zigbeat/internal/music"
)

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
