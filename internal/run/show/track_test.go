package show

import (
	"slices"
	"strings"
	"testing"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
)

func TestTrackBringsAlbumColours(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	p.Track(music.Track{BPM: 124, Palette: navyCover})
	want := []string{"#406cff", "#ff943d", "#7340ff"}
	if !slices.Equal(p.palette, want) {
		t.Fatalf("palette %v, want %v", p.palette, want)
	}
	if n := p.TakeNotes(); len(n) != 1 || n[0] != "album colours #406cff #ff943d #7340ff" {
		t.Errorf("notes %q", n)
	}
	p.Track(music.Track{BPM: 124, Palette: navyCover})
	if n := p.TakeNotes(); n != nil {
		t.Errorf("the same palette again: %q", n)
	}
	for _, m := range p.Beat(moment(0), 0) {
		if c, ok := m.Payload["color"].(map[string]string); ok && !slices.Contains(want, c["hex"]) {
			t.Errorf("%s got %s, not an album colour", m.Light, c["hex"])
		}
	}
	p.Track(music.Track{BPM: 124})
	if !slices.Equal(p.palette, config.DefaultPalette) {
		t.Errorf("without album colours the palette plays: %v", p.palette)
	}
	if n := p.TakeNotes(); len(n) != 1 || n[0] != "no album colours; the palette plays" {
		t.Errorf("notes %q", n)
	}
}

func TestAlbumPaletteOff(t *testing.T) {
	c := testConfig("separate")
	c.Effects.AlbumPalette = false
	p := NewPlanner(c)
	p.Track(music.Track{BPM: 90, Palette: oneRed})
	if !slices.Equal(p.palette, config.DefaultPalette) || p.TakeNotes() != nil || p.bpm != 90 {
		t.Errorf("album colours off: palette %v at %g BPM", p.palette, p.bpm)
	}
}

func TestAlbumColorsFromTheConfig(t *testing.T) {
	c := testConfig("separate")
	c.Effects.AlbumColors = 2
	p := NewPlanner(c)
	p.Track(music.Track{BPM: 124, Palette: brownWood})
	if got := strings.Join(p.palette, " "); got != "#ff9340 #fff240" {
		t.Errorf("two colours: %s", got)
	}
}
