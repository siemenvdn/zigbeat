package show

import (
	"slices"
	"strings"
	"time"

	"github.com/siemenvdn/zigbeat/internal/music"
)

// Track takes the properties of the track that plays now: its tempo, and its
// album colours when the config wants them.
func (p *Planner) Track(t music.Track) {
	p.bpm = t.BPM
	if !p.album {
		return
	}
	pal := AlbumColors(t.Palette, p.albumColors)
	note := "album colours " + strings.Join(pal, " ")
	if pal == nil {
		pal, note = p.fallback, "no album colours; the palette plays"
	}
	if slices.Equal(pal, p.palette) {
		return
	}
	p.palette, p.pi = pal, 0
	p.notes = append(p.notes, note)
}

// period is the time between two beats of the current track.
func (p *Planner) period() time.Duration { return music.Track{BPM: p.bpm}.Period() }

// TakeNotes returns what the operator should know since the last call.
func (p *Planner) TakeNotes() []string {
	n := p.notes
	p.notes = nil
	return n
}
