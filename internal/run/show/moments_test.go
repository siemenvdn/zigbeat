package show

import (
	"testing"

	"github.com/siemenvdn/zigbeat/internal/run/flow"
)

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
	p.Beat(moment(400), 0) // pace 4: a moment every 4 beats
	p.pace = 8             // a moment every 8 beats, as Home Assistant may choose
	if ps := p.Beat(moment(-4), 0); ps != nil {
		t.Error("beat -4 is on the old grid only")
	}
	if ps := p.Beat(moment(0), 0); len(ps) == 0 {
		t.Error("the downbeat is a moment")
	}
}

// TestFasterPaceStartsAtOnce: a faster pace from Home Assistant takes the next
// beat on its grid.
func TestFasterPaceStartsAtOnce(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	p.Beat(moment(4), 0)
	p.pace = 2
	if p.Beat(moment(6), 0) == nil {
		t.Error("beat 6 is on the grid of 2")
	}
}
