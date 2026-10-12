package show

import (
	"maps"
	"testing"
)

func TestKickAndReturn(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	ps := p.Beat(moment(0), 0)
	kick, ret := ps[0], ps[1]
	if kick.Payload["brightness"] != 229 || !kick.Drop || !kick.Land.Equal(t0) {
		t.Errorf("kick: %+v", kick)
	}
	if ret.Payload["brightness"] != 152 || ret.Drop || !ret.Land.Equal(at(500)) {
		t.Errorf("return: %+v", ret)
	}
	for _, pl := range p.Beat(moment(4), 1) {
		if _, ok := pl.Payload["color"]; ok && pl.Payload["brightness"] != nil {
			t.Errorf("backoff 1 has no kick: %v", pl.Payload)
		}
	}
}

func TestMissedKeepsANewerColour(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	kick := p.Beat(moment(0), 0)[0]
	p.current[kick.Light] = "#000000" // a later moment gave it another colour
	p.Missed(kick)
	if p.current[kick.Light] != "#000000" {
		t.Errorf("a dropped older kick took back the newer colour: %q", p.current[kick.Light])
	}
}

func TestMissedGivesBackThePreviousColour(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	p.Start(t0)
	was := maps.Clone(p.current)
	kick := p.Beat(moment(0), 0)[0]
	p.Missed(kick)
	if got := p.current[kick.Light]; got != was[kick.Light] {
		t.Errorf("%s counts as showing %q after a dropped kick, want its previous %q", kick.Light, got, was[kick.Light])
	}
}

// TestKicksAllOrNone: 5 frames/s at a moment per second hold two colour
// changes (2 frames) but not their kicks (6): no kick at all, and the white
// gets the 3 frames left, one pulse.
func TestKicksAllOrNone(t *testing.T) {
	c := testConfig("separate")
	c.Budget.MaxRate = 5
	c.Effects.Pace = 2
	c.Effects.BreatheVariant = "pulse"
	p := NewPlanner(c)
	p.Start(t0)
	ps := p.Beat(moment(0), 0)
	pulses := 0
	for _, m := range ps {
		if m.Payload["color"] != nil && m.Payload["brightness"] != nil {
			t.Errorf("%s kicks", m.Light)
		}
		if m.Light == "lamp-4" && m.Drop {
			pulses++
		}
	}
	if changed(ps) == "" || pulses != 1 {
		t.Errorf("colour %q, pulses %d", changed(ps), pulses)
	}
}

// TestTwoMissedChangesOfOneLight: the engine hands a light's dropped colour
// changes over in queue order; neither went, so the light shows its colour
// from before the first.
func TestTwoMissedChangesOfOneLight(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	p.Start(t0)
	shown := p.current["lamp-1"]
	first := p.paint([]string{"lamp-1"}, "#111111", stroke{at: at(1000)})
	second := p.paint([]string{"lamp-1"}, "#222222", stroke{at: at(1500)})
	p.Missed(first[0], second[0])
	if p.current["lamp-1"] != shown {
		t.Errorf("lamp-1 counts as showing %s, want %s", p.current["lamp-1"], shown)
	}
}

// TestBlendFitsTheMoment: at most half a moment, so a together fixture, whose
// bulbs land one after another, shows each colour together; at 120 BPM.
func TestBlendFitsTheMoment(t *testing.T) {
	c := testConfig("together")
	for pace, want := range map[int]float64{1: 0.25, 2: 0.5, 4: colorBlend} {
		c.Effects.Pace = pace
		p := NewPlanner(c)
		p.Start(t0)
		for _, m := range p.Beat(moment(0), 0) {
			if m.Payload["color"] != nil && m.Payload["transition"] != want {
				t.Errorf("pace %d: %s blends %v s, want %v", pace, m.Light, m.Payload["transition"], want)
			}
		}
	}
}
