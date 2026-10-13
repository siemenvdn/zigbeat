package config

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Mode is how a color fixture changes.
type Mode string

// The modes of a color fixture.
const (
	Alternate Mode = "alternate"
	Separate  Mode = "separate"
	Together  Mode = "together"
)

// Variant is how the colour lights change at a moment.
type Variant string

// The values of effects.variant.
const (
	Rotation Variant = "rotation"
	Walk     Variant = "walk"
	Shuffle  Variant = "shuffle"
	Swap     Variant = "swap"
	Build    Variant = "build"
)

// Variants are the values of effects.variant.
var Variants = []Variant{Rotation, Walk, Shuffle, Swap, Build}

// BreatheVariant is how the breathe lights move at a moment.
type BreatheVariant string

// The values of effects.breathe_variant.
const (
	BreatheRotation BreatheVariant = "rotation"
	Pulse           BreatheVariant = "pulse"
	Flip            BreatheVariant = "flip"
)

// Paces are the values of effects.pace, in beats between moments.
var Paces = []int{1, 2, 4, 8, 16}

// BreatheVariants are the values of effects.breathe_variant.
var BreatheVariants = []BreatheVariant{BreatheRotation, Pulse, Flip}

// The bounds of the effects.
const (
	MinPercent = 1
	MaxPercent = 100
	minPalette = 2

	MinAlbumColors = minPalette

	MaxAlbumColors = 6
)

// Validate checks the effects block.
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

func oneOf[T any](vs []T) string {
	s := make([]string, len(vs))
	for i, v := range vs {
		s[i] = fmt.Sprint(v)
	}
	last := len(s) - 1
	return strings.Join(s[:last], ", ") + " or " + s[last]
}
