package palette

import (
	"math"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// xyScale rounds xy to 4 decimals, as Zigbee2MQTT reports it.
const xyScale = 10000

// hueToXY converts a fully saturated sRGB hue (degrees) to CIE xy, rounded by xyScale.
// The constants are those of sRGB (IEC 61966-2-1) with its D65 white.
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
	round := func(v float64) float64 { return math.Round(v*xyScale) / xyScale }
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
