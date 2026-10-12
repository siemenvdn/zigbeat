package music

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
	period := Track{BPM: m.BPM}.Period()
	return Beat{At: start.Add(time.Duration(i) * period), Index: i, InBar: i % PerBar, Energy: UnknownEnergy}
}

// Events sends one Track, then a beat every period.
func (m Metronome) Events(ctx context.Context) <-chan Event {
	out := make(chan Event)
	go func() {
		defer close(out)
		select {
		case out <- Track{BPM: m.BPM}:
		case <-ctx.Done():
			return
		}
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
