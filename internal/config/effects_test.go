package config

import (
	"strings"
	"testing"
)

func TestAlbumAndVariant(t *testing.T) {
	c, err := Parse([]byte(minimal+"effects: {album_palette: false, album_colors: 6, variant: swap}\n"), env(nil), ForShow)
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
		_, err := Parse([]byte(minimal+"effects: "+c.effects+"\n"), env(nil), ForShow)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.effects, err, c.want)
		}
	}
}

func TestPaceAndBreatheVariant(t *testing.T) {
	for _, c := range []struct{ effects, want string }{
		{"{pace: 1}", ""},
		{"{pace: 16}", ""},
		{"{pace: 0}", "effects.pace must be 1, 2, 4, 8 or 16 beats"},
		{"{pace: 3}", "effects.pace must be 1, 2, 4, 8 or 16 beats"},
		{"{pace: 32}", "effects.pace must be 1, 2, 4, 8 or 16 beats"},
		{"{breathe_variant: flip}", ""},
		{"{breathe_variant: breathe}", "effects.breathe_variant must be rotation, pulse or flip"},
	} {
		_, err := Parse([]byte(minimal+"effects: "+c.effects+"\n"), env(nil), ForShow)
		if c.want == "" && err != nil || c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)) {
			t.Errorf("%s: %v, want %q", c.effects, err, c.want)
		}
	}
}

// TestCadenceIsGone: a config from before the pace in beats says what to set instead.
func TestCadenceIsGone(t *testing.T) {
	_, err := Parse([]byte(minimal+"effects: {cadence: 2s}\n"), env(nil), ForShow)
	if err == nil || err.Error() != "config: effects.cadence is gone: set effects.pace in beats, such as pace: 2" {
		t.Errorf("got %v", err)
	}
}

// TestBoundsAreTheChecks pins the exported bounds, which Home Assistant's
// sliders and list show, to what Validate accepts.
func TestBoundsAreTheChecks(t *testing.T) {
	for i, c := range []struct{ in, out func(*Effects) }{
		{func(e *Effects) { e.Pace = Paces[0] }, func(e *Effects) { e.Pace = 0 }},
		{func(e *Effects) { e.Pace = Paces[len(Paces)-1] }, func(e *Effects) { e.Pace = 2 * Paces[len(Paces)-1] }},
		{func(e *Effects) { e.BreatheVariant = BreatheVariants[len(BreatheVariants)-1] }, func(e *Effects) { e.BreatheVariant = "breathe" }},
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
