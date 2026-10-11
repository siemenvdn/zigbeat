package show

import (
	"testing"
	"time"
)

func TestInterval(t *testing.T) {
	cases := []struct {
		bpm  float64
		want int
	}{{60, 2}, {90, 4}, {120, 4}, {130, 8}, {150, 8}, {200, 8}, {300, 16}}
	for _, c := range cases {
		if got := interval(c.bpm, 2*time.Second); got != c.want {
			t.Errorf("interval(%g) = %d, want %d", c.bpm, got, c.want)
		}
	}
	if got := interval(120, time.Second); got != 2 {
		t.Errorf("cadence 1s at 120 BPM: %d, want 2", got)
	}
}

func TestOnlyMomentsPlan(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	if ps := p.Beat(moment(1), 0); ps != nil {
		t.Errorf("beat 1 is no moment: %v", ps)
	}
	if ps := p.Beat(moment(4), 0); len(ps) == 0 {
		t.Error("beat 4 is a moment")
	}
	if ps := p.Beat(moment(4), 2); ps != nil {
		t.Error("backoff 2 doubles the interval: beat 4 is no moment")
	}
}
