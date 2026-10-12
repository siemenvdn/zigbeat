package config

import (
	"strings"
	"testing"
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
