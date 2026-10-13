package show

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
)

func TestOnlyMomentsPlan(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	if ps := p.Beat(moment(1), false); ps != nil {
		t.Errorf("beat 1 is no moment: %v", ps)
	}
	if ps := p.Beat(moment(4), false); len(ps) == 0 {
		t.Error("beat 4 is a moment")
	}
	for backoff := range flow.PauseLevel {
		for _, i := range []int{8 + 8*backoff, 12 + 8*backoff} {
			if ps := p.Beat(moment(i), backoff >= flow.CalmLevel); len(ps) == 0 {
				t.Errorf("backoff %d keeps the interval: beat %d is a moment", backoff, i)
			}
		}
	}
}

// TestPickupBeatsFollowTheNewGrid.
func TestPickupBeatsFollowTheNewGrid(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	p.Beat(moment(400), false)
	p.pace.beats = 8
	if ps := p.Beat(moment(-4), false); ps != nil {
		t.Error("beat -4 is on the old grid only")
	}
	if ps := p.Beat(moment(0), false); len(ps) == 0 {
		t.Error("the downbeat is a moment")
	}
}

// TestFasterPaceStartsAtOnce.
func TestFasterPaceStartsAtOnce(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	p.Beat(moment(4), false)
	p.pace.beats = 2
	if p.Beat(moment(6), false) == nil {
		t.Error("beat 6 is on the grid of 2")
	}
}

// TestFasterMusicIsNeverSlower.
func TestFasterMusicIsNeverSlower(t *testing.T) {
	const lowest, highest = 60, 300
	for _, pace := range config.Paces {
		c := testConfig("separate")
		c.Effects.Pace = pace
		p := NewPlanner(c)
		last := time.Duration(math.MaxInt64)
		for bpm := lowest; bpm <= highest; bpm++ {
			p.Track(music.Track{BPM: float64(bpm)})
			d := time.Duration(p.interval()) * p.pace.period()
			if d > last {
				t.Fatalf("pace %d: %d BPM gives %s, slower than %s a BPM lower", pace, bpm, d, last)
			}
			last = d
		}
	}
}

// TestTheBudgetStretchesThePace.
func TestTheBudgetStretchesThePace(t *testing.T) {
	c := testConfig("separate")
	c.Budget.MaxRate = 4
	c.Effects.Pace = 1
	p := NewPlanner(c)
	for _, x := range []struct {
		bpm  float64
		want int
	}{{60, 1}, {80, 1}, {81, 2}, {160, 2}, {161, 4}} {
		p.Track(music.Track{BPM: x.bpm})
		if got := p.interval(); got != x.want {
			t.Errorf("%g BPM: every %d beats, want %d", x.bpm, got, x.want)
		}
	}
}

func TestStretchNotes(t *testing.T) {
	c := testConfig("separate")
	c.Budget.MaxRate = 4
	c.Effects.Pace = 1
	p := NewPlanner(c)
	p.Track(music.Track{BPM: 81})
	p.TakeNotes()
	p.Beat(moment(0), false)
	if n := p.TakeNotes(); !slices.Contains(n, "pace every beat; the budget allows every 2 beats at 81 BPM") {
		t.Errorf("notes %q", n)
	}
	p.Beat(moment(2), false)
	if n := p.TakeNotes(); n != nil {
		t.Errorf("no note while it stays stretched: %q", n)
	}
	p.Track(music.Track{BPM: 60})
	p.Beat(moment(3), false)
	if n := p.TakeNotes(); !slices.Contains(n, "pace every beat again") {
		t.Errorf("notes %q", n)
	}
}

// TestATinyBudgetStillChangesColour.
func TestATinyBudgetStillChangesColour(t *testing.T) {
	c := testConfig("separate")
	c.Budget.MaxRate = 0.01
	p := NewPlanner(c)
	p.Start(t0)
	if n := p.interval(); n != config.Paces[len(config.Paces)-1] {
		t.Fatalf("every %d beats, want the longest pace", n)
	}
	if changed(p.Beat(moment(0), false)) == "" {
		t.Error("the colour changes go")
	}
}

func TestFallIsHalfTheIntervalAtMostABeat(t *testing.T) {
	for pace, want := range map[int]time.Duration{1: 250 * time.Millisecond, 2: 500 * time.Millisecond, 4: 500 * time.Millisecond} {
		c := testConfig("separate")
		c.Effects.Pace = pace
		if got := NewPlanner(c).pace.fall(pace); got != want {
			t.Errorf("pace %d at 120 BPM: the fall after %s, want %s", pace, got, want)
		}
	}
}
