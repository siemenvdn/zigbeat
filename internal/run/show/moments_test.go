package show

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
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
	for backoff := range flow.PauseLevel {
		for _, i := range []int{8 + 8*backoff, 12 + 8*backoff} {
			if ps := p.Beat(moment(i), backoff); len(ps) == 0 {
				t.Errorf("backoff %d keeps the interval: beat %d is a moment", backoff, i)
			}
		}
	}
}

// TestPickupBeatsFollowTheNewGrid: a pickup beat before a new track's first
// downbeat is a moment only on that track's grid too, so the track does not
// start with two moments closer than its interval.
func TestPickupBeatsFollowTheNewGrid(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	p.Beat(moment(400), 0)         // 120 BPM: a moment every 4 beats
	p.Track(music.Track{BPM: 128}) // a moment every 8 beats
	if ps := p.Beat(moment(-4), 0); ps != nil {
		t.Error("beat -4 is on the old grid only")
	}
	if ps := p.Beat(moment(0), 0); len(ps) == 0 {
		t.Error("the downbeat is a moment")
	}
}

// TestFadesEndOnAMoment: however the pace and the tempo change, a moment's
// breathe fade lasts exactly until the next moment, so no fade runs on to the
// bulb's own limit.
func TestFadesEndOnAMoment(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 1))
	p := NewPlanner(testConfig("separate"))
	p.Start(t0)
	paces := []time.Duration{time.Second, 2 * time.Second, 3 * time.Second, 8 * time.Second, 16 * time.Second}
	tempos := []float64{60, 90, 120, 128, 170}
	last, planned := -1, 0 // the last moment, the beats its fade lasts
	for i := 0; i < 4000; i++ {
		if rng.IntN(8) == 0 {
			p.cadence = paces[rng.IntN(len(paces))]
		}
		if rng.IntN(32) == 0 {
			p.bpm = tempos[rng.IntN(len(tempos))]
		}
		move, ok := fade(p.Beat(moment(i), 0))
		if !ok {
			continue
		}
		if last >= 0 && i-last != planned {
			t.Fatalf("the fade at beat %d was planned for %d beats; the next moment came at beat %d", last, planned, i)
		}
		last, planned = i, fadeBeats(p, move)
	}
}

// fade returns the brightness_move of the breathe light lamp-4.
func fade(ms []Message) (int, bool) {
	for _, m := range ms {
		if move, ok := m.Payload["brightness_move"].(int); ok && m.Light == "lamp-4" {
			return move, true
		}
	}
	return 0, false
}

// fadeBeats is how many beats a fade at move takes across the breathe range.
func fadeBeats(p *Planner, move int) int {
	for n := 1; n <= 2*intervals[len(intervals)-1]; n *= 2 {
		until := time.Duration(n) * p.period()
		if max(1, int(math.Round(float64(p.high-p.low)/until.Seconds()))) == max(move, -move) {
			return n
		}
	}
	return 0
}
