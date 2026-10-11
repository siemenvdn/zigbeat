// Package beat delivers musical beats ahead of the moment they become audible.
package beat

import (
	"context"
	"time"
)

type Beat struct {
	At     time.Time // when the beat is audible
	BPM    float64
	Index  int     // beats since start; Index 0 is the first beat of a bar
	InBar  int     // 0..3
	Energy float64 // 0..1
}

// Period is the time between two beats.
func (b Beat) Period() time.Duration {
	return time.Duration(float64(time.Minute) / b.BPM)
}

// Source delivers every beat at least one second before it is audible.
// The channel is closed when the source ends or ctx is done.
type Source interface {
	Beats(ctx context.Context) <-chan Beat
}
