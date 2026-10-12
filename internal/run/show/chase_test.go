package show

import (
	"fmt"
	"strings"
	"testing"
)

func TestChaseSeparate(t *testing.T) {
	p := NewPlanner(testConfig("separate"))
	var got []string
	for i := 0; i < 3; i++ {
		got = append(got, changed(p.Beat(moment(4*i), 0)))
	}
	if want := "lamp-1,lamp-2 lamp-3,lamp-1 lamp-2,lamp-3"; strings.Join(got, " ") != want {
		t.Errorf("chase %v, want %s", got, want)
	}
}

func TestAlternateFlipsEveryFourMoments(t *testing.T) {
	p := NewPlanner(testConfig("alternate"))
	var got []string
	for i := 0; i < 9; i++ {
		got = append(got, changed(p.Beat(moment(4*i), 0)))
	}
	all := "lamp-1,lamp-2,lamp-3"
	for i, g := range got {
		together := i/4%2 == 1
		if together != (g == all) {
			t.Errorf("moment %d changed %q (together %v)", i, g, together)
		}
	}
}

func TestTogetherSharesOneColour(t *testing.T) {
	p := NewPlanner(testConfig("together"))
	p.Start(t0)
	for i := 0; i < 6; i++ {
		before := p.current["lamp-1"]
		colours := map[string]bool{}
		for _, pl := range p.Beat(moment(4*i), 0) {
			if c, ok := pl.Payload["color"]; ok {
				colours[fmt.Sprint(c)] = true
			}
		}
		if len(colours) != 1 {
			t.Fatalf("moment %d: %d colours in a together unit", i, len(colours))
		}
		if p.current["lamp-1"] == before {
			t.Fatalf("moment %d: the fixture kept colour %s", i, before)
		}
	}
}

func TestSingleRoleConfigs(t *testing.T) {
	breatheOnly := testConfig("separate")
	breatheOnly.Fixtures = breatheOnly.Fixtures[1:]
	p := NewPlanner(breatheOnly)
	p.Start(t0)
	if ps := p.Beat(moment(0), 0); len(ps) != 2 || ps[0].Light != "lamp-4" || ps[1].Light != "lamp-4" {
		t.Errorf("breathe only: a pulse up and its fall: %v", ps)
	}
	colorOnly := testConfig("together")
	colorOnly.Fixtures = colorOnly.Fixtures[:1]
	p = NewPlanner(colorOnly)
	p.Start(t0)
	if got := changed(p.Beat(moment(0), 0)); got != "lamp-1,lamp-2,lamp-3" {
		t.Errorf("color only: %q", got)
	}
}
