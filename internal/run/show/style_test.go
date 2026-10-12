package show

import (
	"slices"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
)

// style is the test config's effects with change applied, as Home Assistant sends them.
func style(change func(*config.Effects)) config.Effects {
	e := testConfig("separate").Effects
	change(&e)
	return e
}

func TestAlbumColoursSwitchMidTrack(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	p.Track(music.Track{BPM: 124, Palette: navyCover})
	p.TakeNotes()
	p.SetEffects(style(func(e *config.Effects) { e.AlbumPalette = false }))
	if !slices.Equal(p.palette, config.DefaultPalette) {
		t.Errorf("album colours off: %v", p.palette)
	}
	if n := p.TakeNotes(); len(n) != 1 || n[0] != "album colours off; the palette plays" {
		t.Errorf("notes %q", n)
	}
	p.SetEffects(style(func(e *config.Effects) { e.AlbumColors = 2 }))
	if n := p.TakeNotes(); len(n) != 1 || n[0] != "album colours #406cff #ff943d" {
		t.Errorf("back on, two colours from the same track: %q", n)
	}
	p.SetEffects(style(func(e *config.Effects) { e.AlbumColors = 2 }))
	if n := p.TakeNotes(); n != nil {
		t.Errorf("the same style again: %q", n)
	}
}

func TestRotationChosenMidTrack(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	p.Start(t0)
	p.Beat(moment(156), 0)
	p.TakeNotes()
	p.SetEffects(style(func(e *config.Effects) { e.Variant = "rotation" }))
	p.Beat(moment(160), 0)
	if n := p.TakeNotes(); len(n) != 1 || n[0] != "variant shuffle from bar 40" {
		t.Errorf("rotation chosen in bar 40 plays that phrase's variant: %q", n)
	}
}

func TestBrightnessFromTheNextMoment(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	p.Start(t0)
	p.SetEffects(style(func(e *config.Effects) { e.Brightness = 30 }))
	kicks, returns := 0, 0
	for _, m := range p.Beat(moment(4), 0) {
		switch {
		case m.Light == "lamp-4":
		case m.Payload["color"] != nil:
			kicks++
			if m.Payload["brightness"] != level(30+kickPoints) {
				t.Errorf("%s kicks to %v", m.Light, m.Payload["brightness"])
			}
		default:
			returns++
			if m.Payload["brightness"] != level(30) {
				t.Errorf("%s returns to %v", m.Light, m.Payload["brightness"])
			}
		}
	}
	if kicks == 0 || kicks != returns {
		t.Errorf("%d kicks, %d returns", kicks, returns)
	}
}

func TestSlowerPaceStartsOnASharedMoment(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	p.Start(t0)
	p.Beat(moment(4), 0)
	p.SetEffects(style(func(e *config.Effects) { e.Cadence = 8 * time.Second }))
	var got []int
	for i := 5; i <= 40; i++ {
		if p.Beat(moment(i), 0) != nil {
			got = append(got, i)
		}
	}
	// 8 s at 120 BPM is 16 beats; the grid of 4 holds until beat 16, on both.
	if want := []int{8, 12, 16, 32}; !slices.Equal(got, want) {
		t.Errorf("moments at %v, want %v", got, want)
	}
}
