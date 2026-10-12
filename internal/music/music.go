// Package music describes what plays, ahead of time: tracks and their beats.
package music

import (
	"context"
	"time"
)

const (
	PerBar     = 4   // beats per bar: 4/4 time
	DefaultBPM = 120 // a neutral tempo until a track says otherwise
	// Lookahead is how long before it sounds a source delivers a beat: time to
	// plan a moment and send it ahead of the network's latency.
	Lookahead = time.Second
	// UnknownEnergy is the energy of a beat whose track has shown no louder or
	// quieter part yet, and of every metronome beat.
	UnknownEnergy = 0.5
)

// Source reports what plays, ahead of time, as one ordered stream. The channel
// is closed when the source ends or ctx is done.
type Source interface {
	Events(ctx context.Context) <-chan Event
}

// Event is a Clear, a Track or a Beat.
type Event interface{ isEvent() }

// Clear: what was delivered no longer applies, after a seek, a new track or the
// music stopping.
type Clear struct{}

// RGB is a colour as the source reports it.
type RGB [3]uint8

// Track holds the properties of what plays. It comes before the first beat of
// a track, and again whenever one of them changes.
type Track struct {
	BPM     float64
	Palette []RGB // the album art's colours, most dominant first; nil: none
}

// Period is the time between two beats.
func (t Track) Period() time.Duration {
	return time.Duration(float64(time.Minute) / t.BPM)
}

// Beat is one beat of the current track, delivered Lookahead before it sounds.
type Beat struct {
	At     time.Time // when the beat is audible
	Index  int       // beats since the track's first downbeat; Index 0 is the first beat of a bar
	InBar  int       // 0..PerBar-1
	Energy float64   // loudness of the coming bar within the track, 0..1
}

func (Clear) isEvent() {}
func (Track) isEvent() {}
func (Beat) isEvent()  {}
