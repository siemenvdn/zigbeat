package probe

import (
	"math"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

type command struct {
	key     int
	payload map[string]any
}

// generator hands out commands whose echo can be told apart: no key repeats
// for a device until the whole palette has been used.
type generator struct {
	mode   string
	colors []z2m.XY
	levels []int
	next   map[string]int
}

func newGenerator(mode string) *generator {
	g := &generator{mode: mode, next: map[string]int{}}
	seen := map[int]bool{}
	for i := 0; i < 72 && len(g.colors) < 36; i++ {
		xy := hueToXY(float64(i) * 5)
		k := xKey(xy.X)
		if seen[k] || seen[k-1] || seen[k+1] {
			continue
		}
		seen[k] = true
		g.colors = append(g.colors, xy)
	}
	for b := 40; b <= 250; b += 6 {
		g.levels = append(g.levels, b)
	}
	return g
}

func (g *generator) paletteSize() int {
	if g.mode == "brightness" {
		return len(g.levels)
	}
	return len(g.colors)
}

func (g *generator) nextFor(dev string) command {
	i := g.next[dev]
	g.next[dev] = i + 1
	switch g.mode {
	case "brightness":
		b := g.levels[i%len(g.levels)]
		return command{key: b, payload: map[string]any{"brightness": b, "transition": 0}}
	case "combo":
		c := g.colors[i%len(g.colors)]
		b := 140
		if i%2 == 0 {
			b = 254
		}
		return command{key: xKey(c.X), payload: map[string]any{"color": c, "brightness": b, "transition": 0}}
	default:
		c := g.colors[i%len(g.colors)]
		return command{key: xKey(c.X), payload: map[string]any{"color": c, "transition": 0}}
	}
}

func keyFunc(mode string) func(z2m.State) (int, bool) {
	if mode == "brightness" {
		return func(s z2m.State) (int, bool) {
			if s.Brightness == nil {
				return 0, false
			}
			return *s.Brightness, true
		}
	}
	return func(s z2m.State) (int, bool) {
		if s.Color == nil {
			return 0, false
		}
		return xKey(s.Color.X), true
	}
}

func xKey(x float64) int { return int(math.Round(x * 1000)) }

// hueToXY converts a fully saturated sRGB hue (degrees) to CIE xy, rounded to 4 decimals.
func hueToXY(h float64) z2m.XY {
	r, g, b := hsvToRGB(h, 1, 1)
	lin := func(c float64) float64 {
		if c <= 0.04045 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	r, g, b = lin(r), lin(g), lin(b)
	X := r*0.4124 + g*0.3576 + b*0.1805
	Y := r*0.2126 + g*0.7152 + b*0.0722
	Z := r*0.0193 + g*0.1192 + b*0.9505
	sum := X + Y + Z
	round := func(v float64) float64 { return math.Round(v*10000) / 10000 }
	return z2m.XY{X: round(X / sum), Y: round(Y / sum)}
}

func hsvToRGB(h, s, v float64) (float64, float64, float64) {
	h = math.Mod(h, 360) / 60
	i := math.Floor(h)
	f := h - i
	p, q, t := v*(1-s), v*(1-s*f), v*(1-s*(1-f))
	switch int(i) {
	case 0:
		return v, t, p
	case 1:
		return q, v, p
	case 2:
		return p, v, t
	case 3:
		return p, q, v
	case 4:
		return t, p, v
	default:
		return v, p, q
	}
}
