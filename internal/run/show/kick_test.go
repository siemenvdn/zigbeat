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
