package show

import (
	"maps"
	"testing"
)

func TestKickAndReturn(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	ps := p.Beat(moment(0), false)
	kick, ret := ps[0], ps[1]
	if kick.Payload["brightness"] != 229 || !kick.Drop || !kick.Land.Equal(t0) {
		t.Errorf("kick: %+v", kick)
	}
	if ret.Payload["brightness"] != 152 || ret.Drop || !ret.Land.Equal(at(500)) {
		t.Errorf("return: %+v", ret)
	}
	for _, pl := range p.Beat(moment(4), true) {
		if _, ok := pl.Payload["color"]; ok && pl.Payload["brightness"] != nil {
			t.Errorf("backoff 1 has no kick: %v", pl.Payload)
		}
	}
}

func TestMissedKeepsANewerColour(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	kick := p.Beat(moment(0), false)[0]
	p.colors.current[kick.Light] = "#000000"
	p.Missed(kick)
	if p.colors.current[kick.Light] != "#000000" {
		t.Errorf("a dropped older kick took back the newer colour: %q", p.colors.current[kick.Light])
	}
}

func TestMissedGivesBackThePreviousColour(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	p.Start(t0)
	was := maps.Clone(p.colors.current)
	kick := p.Beat(moment(0), false)[0]
	p.Missed(kick)
	if got := p.colors.current[kick.Light]; got != was[kick.Light] {
		t.Errorf("%s counts as showing %q after a dropped kick, want its previous %q", kick.Light, got, was[kick.Light])
	}
}

// TestKicksAllOrNone.
func TestKicksAllOrNone(t *testing.T) {
	c := testConfig("separate")
	c.Budget.MaxRate = 5
	c.Effects.Pace = 2
	c.Effects.BreatheVariant = "pulse"
	p := NewPlanner(c)
	p.Start(t0)
	ps := p.Beat(moment(0), false)
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

// TestTwoMissedChangesOfOneLight.
func TestTwoMissedChangesOfOneLight(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	p.Start(t0)
	shown := p.colors.current["lamp-1"]
	first := p.colors.paint([]string{"lamp-1"}, "#111111", stroke{at: at(1000)})
	second := p.colors.paint([]string{"lamp-1"}, "#222222", stroke{at: at(1500)})
	p.Missed(first[0], second[0])
	if p.colors.current["lamp-1"] != shown {
		t.Errorf("lamp-1 counts as showing %s, want %s", p.colors.current["lamp-1"], shown)
	}
}

// TestBlendFitsTheMoment.
func TestBlendFitsTheMoment(t *testing.T) {
	c := testConfig("together")
	for pace, want := range map[int]float64{1: 0.25, 2: 0.5, 4: colorBlend} {
		c.Effects.Pace = pace
		p := NewPlanner(c)
		p.Start(t0)
		for _, m := range p.Beat(moment(0), false) {
			if m.Payload["color"] != nil && m.Payload["transition"] != want {
				t.Errorf("pace %d: %s blends %v s, want %v", pace, m.Light, m.Payload["transition"], want)
			}
		}
	}
}
