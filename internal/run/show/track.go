package show

import (
	"time"

	"github.com/siemenvdn/zigbeat/internal/music"
)

// Track takes the properties of the track that plays now: its tempo, and its
// album colours when the style wants them.
func (p *Planner) Track(t music.Track) {
	p.bpm, p.art, p.lowBar = t.BPM, t.Palette, false // a new track is full until a bar says otherwise
	p.choose()
}

// period is the time between two beats of the current track.
func (p *Planner) period() time.Duration { return music.Track{BPM: p.bpm}.Period() }

// TakeNotes returns what the operator should know since the last call.
func (p *Planner) TakeNotes() []string {
	n := p.notes
	p.notes = nil
	return n
}
