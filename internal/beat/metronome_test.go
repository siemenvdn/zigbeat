package beat

import (
	"context"
	"testing"
	"time"
)

func TestMetronomeGrid(t *testing.T) {
	m := Metronome{BPM: 120}
	start := time.Date(2026, 1, 1, 20, 0, 0, 0, time.UTC)
	b := m.beat(start, 5)
	if !b.At.Equal(start.Add(2500*time.Millisecond)) || b.Index != 5 || b.InBar != 1 || b.Energy != 1 {
		t.Errorf("beat 5: %+v", b)
	}
	if b.Period() != 500*time.Millisecond {
		t.Errorf("period %s", b.Period())
	}
	if s := m.String(); s != "metronome at 120 BPM" {
		t.Errorf("String() = %q", s)
	}
}

func TestMetronomeDeliversAhead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := Metronome{BPM: 300, Lookahead: 150 * time.Millisecond}
	beats := m.Beats(ctx)
	for i := 0; i < 3; i++ {
		b := <-beats
		if b.Index != i {
			t.Fatalf("got index %d, want %d", b.Index, i)
		}
		if ahead := time.Until(b.At); ahead < 100*time.Millisecond {
			t.Errorf("beat %d delivered only %s ahead", i, ahead)
		}
	}
	cancel()
	for range beats {
	}
}
