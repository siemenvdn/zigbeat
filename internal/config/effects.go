package config

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Variants are the values of effects.variant: rotation, or one variant all the time.
var Variants = []string{"rotation", "walk", "shuffle", "swap", "build"}

// The bounds of the effects. Home Assistant's sliders show the same ones.
const (
	// MinCadence: faster moments outrun a budget of a few frames per second.
	MinCadence = time.Second
	// MaxCadence: the longest interval, 16 beats, at 60 BPM.
	MaxCadence = 16 * time.Second
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
	case e.Cadence < MinCadence || e.Cadence > MaxCadence:
		return fmt.Errorf("effects.cadence must be %v to %v", MinCadence, MaxCadence)
	case e.Brightness < MinPercent || e.Brightness > MaxPercent:
		return fmt.Errorf("effects.brightness must be %d to %d", MinPercent, MaxPercent)
	case e.BreatheRange[0] < MinPercent || e.BreatheRange[1] > MaxPercent || e.BreatheRange[0] >= e.BreatheRange[1]:
		return fmt.Errorf("effects.breathe_range must be [low, high] within %d to %d with low below high", MinPercent, MaxPercent)
	case len(e.Palette) < minPalette:
		return fmt.Errorf("effects.palette needs at least %d colours", minPalette)
	case e.AlbumColors < MinAlbumColors || e.AlbumColors > MaxAlbumColors:
		return fmt.Errorf("effects.album_colors must be %d to %d", MinAlbumColors, MaxAlbumColors)
	case !slices.Contains(Variants, e.Variant):
		last := len(Variants) - 1
		return fmt.Errorf("effects.variant must be %s or %s", strings.Join(Variants[:last], ", "), Variants[last])
	}
	for _, p := range e.Palette {
		if !hexColor.MatchString(p) {
			return fmt.Errorf("effects.palette: %q is not a #rrggbb colour", p)
		}
	}
	return nil
}
