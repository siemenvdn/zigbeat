// Package music describes what plays, ahead of time.
package music

import (
	"context"
	"time"
)

const (
	PerBar     = 4
	DefaultBPM = 120

	Lookahead = time.Second

	UnknownEnergy = 0.5
)

// Source reports what plays, ahead of time, as one ordered stream.
type Source interface {
	Events(ctx context.Context) <-chan Event
}

// Event is a Clear, a Track or a Beat.
type Event interface{ isEvent() }

// Clear.
type Clear struct{}

// RGB is a colour as the source reports it.
type RGB [3]uint8

// Track holds the properties of what plays.
type Track struct {
	BPM     float64
	Palette []RGB
}

// Period is the time between two beats.
func (t Track) Period() time.Duration {
	return time.Duration(float64(time.Minute) / t.BPM)
}

// Beat is one beat of the current track, delivered Lookahead before it sounds.
type Beat struct {
	At     time.Time
	Index  int
	InBar  int
	Energy float64
}

func (Clear) isEvent() {}
func (Track) isEvent() {}
func (Beat) isEvent()  {}

// Bar is the number of the bar b falls in, counted from the track's first downbeat.
func (b Beat) Bar() int { return (b.Index - b.InBar) / PerBar }
