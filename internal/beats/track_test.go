package beats

import (
	"testing"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
)

func TestTrackLine(t *testing.T) {
	on, off := config.DefaultEffects, config.DefaultEffects
	off.AlbumPalette = false
	red := []music.RGB{{180, 30, 40}, {250, 210, 210}}
	for _, c := range []struct {
		track   music.Track
		effects config.Effects
		want    string
	}{
		{music.Track{BPM: 124, Palette: red}, on, "track 124.0 BPM, album colours #ff2b39 #ff872b #ff2ba3 (Music Assistant sent 2)"},
		{music.Track{BPM: 124}, on, "track 124.0 BPM, no album colours; the palette plays"},
		{music.Track{BPM: 80, Palette: []music.RGB{{200, 200, 200}}}, on, "track 80.0 BPM, no album colours; the palette plays (Music Assistant sent 1)"},
		{music.Track{BPM: 124, Palette: red}, off, "track 124.0 BPM, album colours off in the config; the palette plays (Music Assistant sent 2)"},
	} {
		if got := trackLine(c.track, c.effects); got != c.want {
			t.Errorf("%+v:\n%s\nwant\n%s", c.track, got, c.want)
		}
	}
}
