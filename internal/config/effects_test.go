package config

import (
	"strings"
	"testing"
	"time"
)

func TestAlbumAndVariant(t *testing.T) {
	c, err := Parse([]byte(minimal+"effects: {album_palette: false, album_colors: 6, variant: swap}\n"), env(nil))
	if err != nil || c.Effects.AlbumPalette || c.Effects.AlbumColors != 6 || c.Effects.Variant != "swap" {
		t.Errorf("effects %+v, %v", c.Effects, err)
	}
}

func TestAlbumAndVariantErrors(t *testing.T) {
	for _, c := range []struct{ effects, want string }{
		{"{album_colors: 1}", "effects.album_colors must be 2 to 6"},
		{"{album_colors: 7}", "effects.album_colors must be 2 to 6"},
		{"{variant: random}", "effects.variant must be rotation, walk, shuffle, swap or build"},
	} {
		_, err := Parse([]byte(minimal+"effects: "+c.effects+"\n"), env(nil))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.effects, err, c.want)
		}
	}
}

func TestCadenceBounds(t *testing.T) {
	for _, c := range []struct {
		cadence string
		ok      bool
	}{{"1s", true}, {"16s", true}, {"500ms", false}, {"17s", false}} {
		_, err := Parse([]byte(minimal+"effects: {cadence: "+c.cadence+"}\n"), env(nil))
		if c.ok != (err == nil) || err != nil && !strings.Contains(err.Error(), "effects.cadence must be 1s to 16s") {
			t.Errorf("cadence %s: %v", c.cadence, err)
		}
	}
}

// TestBoundsAreTheChecks pins the exported bounds, which Home Assistant's
// sliders and list show, to what Validate accepts.
func TestBoundsAreTheChecks(t *testing.T) {
	for i, c := range []struct{ in, out func(*Effects) }{
		{func(e *Effects) { e.Cadence = MinCadence }, func(e *Effects) { e.Cadence = MinCadence - time.Millisecond }},
		{func(e *Effects) { e.Cadence = MaxCadence }, func(e *Effects) { e.Cadence = MaxCadence + time.Millisecond }},
		{func(e *Effects) { e.Brightness = MinPercent }, func(e *Effects) { e.Brightness = MinPercent - 1 }},
		{func(e *Effects) { e.Brightness = MaxPercent }, func(e *Effects) { e.Brightness = MaxPercent + 1 }},
		{func(e *Effects) { e.AlbumColors = MinAlbumColors }, func(e *Effects) { e.AlbumColors = MinAlbumColors - 1 }},
		{func(e *Effects) { e.AlbumColors = MaxAlbumColors }, func(e *Effects) { e.AlbumColors = MaxAlbumColors + 1 }},
		{func(e *Effects) { e.Variant = Variants[len(Variants)-1] }, func(e *Effects) { e.Variant = "random" }},
	} {
		e := DefaultEffects
		e.Palette = DefaultPalette
		c.in(&e)
		if err := e.Validate(); err != nil {
			t.Errorf("case %d, on the bound: %v", i, err)
		}
		e = DefaultEffects
		e.Palette = DefaultPalette
		c.out(&e)
		if e.Validate() == nil {
			t.Errorf("case %d, past the bound: no error", i)
		}
	}
}
