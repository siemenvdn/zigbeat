package show

import (
	"slices"
	"strings"
	"testing"

	"github.com/siemenvdn/zigbeat/internal/music"
)

func beatWith(i int, e float64) music.Beat {
	b := moment(i)
	b.Energy = e
	return b
}

func kicks(p *Planner, ps []Message) bool {
	return slices.ContainsFunc(ps, func(m Message) bool {
		return m.Payload["color"] != nil && m.Payload["brightness"] != nil || m.Light == "lamp-4" && m.Payload["brightness"] == p.breathe.high
	})
}

// TestQuietPartPlaysCalmer.
func TestQuietPartPlaysCalmer(t *testing.T) {
	c := testConfig("separate")
	c.Effects.BreatheVariant = "pulse"
	p := NewPlanner(c)
	p.Start(t0)
	p.TakeNotes()
	if ps := p.Beat(beatWith(0, 0.1), false); changed(ps) == "" || kicks(p, ps) {
		t.Errorf("a quiet bar: %+v", ps)
	}
	if ps := p.Beat(beatWith(4, 0.1), false); ps != nil {
		t.Errorf("a quiet part plays every 2 bars, not every bar: %+v", ps)
	}
	if ps := p.Beat(beatWith(8, 0.9), false); !kicks(p, ps) {
		t.Errorf("a loud bar kicks: %+v", ps)
	}
	want := []string{"energy quiet, no kicks from bar 0: every 2 bars", "energy full from bar 2: every bar"}
	n := slices.DeleteFunc(p.TakeNotes(), func(s string) bool { return !strings.HasPrefix(s, "energy") })
	if !slices.Equal(n, want) {
		t.Errorf("notes %q, want %q", n, want)
	}
}

func TestFollowEnergyOff(t *testing.T) {
	c := testConfig("separate")
	c.Effects.FollowEnergy = false
	p := NewPlanner(c)
	p.Start(t0)
	if ps := p.Beat(beatWith(0, 0), false); !kicks(p, ps) {
		t.Errorf("without follow_energy a quiet bar still kicks: %+v", ps)
	}
}

// TestNewTrackIsFull.
func TestNewTrackIsFull(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	p.Start(t0)
	p.Beat(beatWith(0, 0), false)
	p.Track(music.Track{BPM: music.DefaultBPM})
	pickup := beatWith(5, music.UnknownEnergy)
	pickup.InBar = 3
	p.Beat(pickup, false)
	if p.energy.quiet {
		t.Error("a new track starts quiet")
	}
}

// TestSwitchOffInAQuietPart.
func TestSwitchOffInAQuietPart(t *testing.T) {
	c := testConfig("separate")
	p := NewPlanner(c)
	p.Start(t0)
	p.Beat(beatWith(0, 0), false)
	c.Effects.FollowEnergy = false
	p.SetEffects(c.Effects)
	p.TakeNotes()
	p.Beat(beatWith(1, 0), false)
	if p.energy.quiet || !slices.Contains(p.TakeNotes(), "energy full from bar 0: every bar") {
		t.Error("the switch did not end the quiet part")
	}
}

// TestQuietPartHolds.
func TestQuietPartHolds(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	p.Start(t0)
	for i, x := range []struct {
		energy float64
		quiet  bool
	}{{0.9, false}, {0.4, false}, {0, true}, {0.38, true}, {0.31, true}, {0.21, true}, {0.3, true}, {0.5, false}, {0.35, false}} {
		p.Beat(beatWith(i*4, x.energy), false)
		if p.energy.quiet != x.quiet {
			t.Errorf("bar %d at energy %.2f: quiet %v, want %v", i, x.energy, p.energy.quiet, x.quiet)
		}
	}
}
