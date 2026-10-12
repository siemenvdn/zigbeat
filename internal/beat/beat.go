// Package beat delivers musical beats ahead of the moment they become audible.
package beat

import (
	"context"
	"time"
)

const (
	PerBar     = 4   // beats per bar: 4/4 time
	DefaultBPM = 120 // a neutral tempo until the beats say otherwise
	// Lookahead is how long before it sounds a source delivers a beat: time to
	// plan a moment and send it ahead of the network's latency.
	Lookahead = time.Second
)

type Beat struct {
	At     time.Time // when the beat is audible
	BPM    float64
	Index  int     // beats since start; Index 0 is the first beat of a bar
	InBar  int     // 0..PerBar-1
	Energy float64 // 0..1

	// Clear marks no beat but a jump: the beats delivered before it no longer
	// apply, after a seek, a new track or the music stopping.
	Clear bool
}

// Period is the time between two beats.
func (b Beat) Period() time.Duration {
	return time.Duration(float64(time.Minute) / b.BPM)
}

// Source delivers every beat at least Lookahead before it is audible.
// The channel is closed when the source ends or ctx is done.
type Source interface {
	Beats(ctx context.Context) <-chan Beat
}
