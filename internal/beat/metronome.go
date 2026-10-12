package beat

import (
	"context"
	"fmt"
	"time"
)

// Metronome is a fixed-tempo 4/4 source for development.
type Metronome struct {
	BPM       float64
	Lookahead time.Duration // how long before At each beat is delivered
}

func (m Metronome) String() string { return fmt.Sprintf("metronome at %g BPM", m.BPM) }

func (m Metronome) beat(start time.Time, i int) Beat {
	period := time.Duration(float64(time.Minute) / m.BPM)
	return Beat{At: start.Add(time.Duration(i) * period), BPM: m.BPM, Index: i, InBar: i % 4, Energy: 1}
}

func (m Metronome) Beats(ctx context.Context) <-chan Beat {
	out := make(chan Beat)
	go func() {
		defer close(out)
		start := time.Now().Add(m.Lookahead)
		for i := 0; ; i++ {
			b := m.beat(start, i)
			t := time.NewTimer(time.Until(b.At.Add(-m.Lookahead)))
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}
			select {
			case out <- b:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}
