package config

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Variants are the values of effects.variant: rotation, or one variant all the time.
var Variants = []string{"rotation", "walk", "shuffle", "swap", "build"}

// Paces are the values of effects.pace, in beats between moments: powers of two
// keep moments on bar lines, and 16 beats are four bars.
var Paces = []int{1, 2, 4, 8, 16}

// BreatheVariants are the values of effects.breathe_variant: rotation, or one all the time.
var BreatheVariants = []string{"rotation", "pulse", "flip"}

// The bounds of the effects. Home Assistant's sliders show the same ones.
const (
	MinPercent = 1 // effects are in percent; 0 would be off
	MaxPercent = 100
	minPalette = 2 // alternating needs two colours
	// MinAlbumColors: as for the palette, two colours to alternate.
	MinAlbumColors = minPalette
	// MaxAlbumColors: Music Assistant sends at most 6 colours; more would only be fill-ins.
	MaxAlbumColors = 6
)

// Validate checks the effects block. The choices made in Home Assistant go
// through it too, so they meet the config's rules and messages.
func (e Effects) Validate() error {
	switch {
	case !slices.Contains(Paces, e.Pace):
		return fmt.Errorf("effects.pace must be %s beats", oneOf(Paces))
	case e.Brightness < MinPercent || e.Brightness > MaxPercent:
		return fmt.Errorf("effects.brightness must be %d to %d", MinPercent, MaxPercent)
	case e.BreatheRange[0] < MinPercent || e.BreatheRange[1] > MaxPercent || e.BreatheRange[0] >= e.BreatheRange[1]:
		return fmt.Errorf("effects.breathe_range must be [low, high] within %d to %d with low below high", MinPercent, MaxPercent)
	case !slices.Contains(BreatheVariants, e.BreatheVariant):
		return fmt.Errorf("effects.breathe_variant must be %s", oneOf(BreatheVariants))
	case len(e.Palette) < minPalette:
		return fmt.Errorf("effects.palette needs at least %d colours", minPalette)
	case e.AlbumColors < MinAlbumColors || e.AlbumColors > MaxAlbumColors:
		return fmt.Errorf("effects.album_colors must be %d to %d", MinAlbumColors, MaxAlbumColors)
	case !slices.Contains(Variants, e.Variant):
		return fmt.Errorf("effects.variant must be %s", oneOf(Variants))
	}
	for _, p := range e.Palette {
		if !hexColor.MatchString(p) {
			return fmt.Errorf("effects.palette: %q is not a #rrggbb colour", p)
		}
	}
	return nil
}

// oneOf lists values for a message: a, b or c.
func oneOf[T any](vs []T) string {
	s := make([]string, len(vs))
	for i, v := range vs {
		s[i] = fmt.Sprint(v)
	}
	last := len(s) - 1
	return strings.Join(s[:last], ", ") + " or " + s[last]
}
