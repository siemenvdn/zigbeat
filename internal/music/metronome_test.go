package music

import (
	"context"
	"testing"
	"time"
)

func TestMetronomeGrid(t *testing.T) {
	m := Metronome{BPM: 120}
	start := time.Date(2026, 1, 1, 20, 0, 0, 0, time.UTC)
	b := m.beat(start, 5)
	if !b.At.Equal(start.Add(2500*time.Millisecond)) || b.Index != 5 || b.InBar != 1 || b.Energy != UnknownEnergy {
		t.Errorf("beat 5: %+v", b)
	}
	if p := (Track{BPM: 120}).Period(); p != 500*time.Millisecond {
		t.Errorf("period %s", p)
	}
	if s := m.String(); s != "metronome at 120 BPM" {
		t.Errorf("String() = %q", s)
	}
}

func TestMetronomeSendsTheTrackThenBeatsAhead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := Metronome{BPM: 300, Lookahead: 150 * time.Millisecond}
	events := m.Events(ctx)
	if tr, ok := (<-events).(Track); !ok || tr.BPM != 300 || tr.Palette != nil {
		t.Fatalf("the first event is not the track at 300 BPM: %+v", tr)
	}
	for i := 0; i < 3; i++ {
		b, ok := (<-events).(Beat)
		if !ok || b.Index != i {
			t.Fatalf("event %d is not beat %d: %+v", i+1, i, b)
		}
		if ahead := time.Until(b.At); ahead < 100*time.Millisecond {
			t.Errorf("beat %d delivered only %s ahead", i, ahead)
		}
	}
	cancel()
	for range events {
	}
}
