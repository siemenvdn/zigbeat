package show

import (
	"fmt"
	"slices"
	"testing"

	"github.com/siemenvdn/zigbeat/internal/config"
)

// whites returns the test config with breathe lights lamp-4 up to lamp-<3+n>,
// playing variant.
func whites(variant string, n int) config.Config {
	c := testConfig("separate")
	c.Fixtures[1].Lights = nil
	for i := range n {
		c.Fixtures[1].Lights = append(c.Fixtures[1].Lights, fmt.Sprintf("lamp-%d", 4+i))
	}
	c.Effects.BreatheVariant = variant
	return c
}

// white lists the breathe messages of ps as light:level, then + when it may be dropped.
func white(ps []Message, lights []string) []string {
	var out []string
	for _, m := range ps {
		if slices.Contains(lights, m.Light) {
			s := fmt.Sprintf("%s:%v", m.Light, m.Payload["brightness"])
			if m.Drop {
				s += "+"
			}
			out = append(out, s)
		}
	}
	return out
}

func TestPulse(t *testing.T) {
	p := NewPlanner(whites("pulse", 1))
	p.Start(t0)
	b := moment(4)
	var up, down Message
	for _, m := range p.Beat(b, 0) {
		if m.Light == "lamp-4" && m.Land.Equal(b.At) {
			up = m
		} else if m.Light == "lamp-4" {
			down = m
		}
	}
	if up.Payload["brightness"] != level(100) || !up.Drop || len(up.Payload) != 1 {
		t.Errorf("up: %+v", up)
	}
	if down.Payload["brightness"] != level(10) || down.Drop || !down.Land.Equal(b.At.Add(p.period())) {
		t.Errorf("the fall lands a beat later and is never dropped: %+v", down)
	}
}

func TestFlipInCounterPhase(t *testing.T) {
	c := whites("flip", 2)
	p := NewPlanner(c)
	p.Start(t0)
	lights := c.Fixtures[1].Lights
	hi, lo := level(100), level(10)
	want := [][]string{
		{fmt.Sprintf("lamp-4:%d+", hi)}, // lamp-5 starts low already
		{fmt.Sprintf("lamp-4:%d+", lo), fmt.Sprintf("lamp-5:%d+", hi)},
		{fmt.Sprintf("lamp-4:%d+", hi), fmt.Sprintf("lamp-5:%d+", lo)},
	}
	for i, w := range want {
		got := white(p.Beat(moment(4*i), 0), lights)
		slices.Sort(got) // the turn decides who goes first
		if !slices.Equal(got, w) {
			t.Errorf("moment %d: %v, want %v", i, got, w)
		}
	}
}

func TestBreatheRotation(t *testing.T) {
	p := NewPlanner(whites("rotation", 1))
	for index, want := range map[int]string{-4: "pulse", 0: "pulse", 60: "pulse", 64: "flip", 128: "pulse", 192: "flip"} {
		if got := p.breatheAt(index); got != want {
			t.Errorf("index %d: %s, want %s", index, got, want)
		}
	}
	if got := NewPlanner(whites("flip", 1)).breatheAt(0); got != "flip" {
		t.Errorf("a fixed breathe variant plays from the start: %s", got)
	}
	none := testConfig("separate")
	none.Fixtures = none.Fixtures[:1]
	if got := NewPlanner(none).breatheAt(64); got != "" {
		t.Errorf("no breathe lights, no breathe variant: %q", got)
	}
}

// TestCalmHoldsTheWhite: from the calm level a breathe light that is high gets
// its low end once, never dropped, and then nothing.
func TestCalmHoldsTheWhite(t *testing.T) {
	c := whites("flip", 1)
	p := NewPlanner(c)
	p.Start(t0)
	lights := c.Fixtures[1].Lights
	p.Beat(moment(0), 0) // lamp-4 goes high
	if got := white(p.Beat(moment(4), 1), lights); !slices.Equal(got, []string{fmt.Sprintf("lamp-4:%d", level(10))}) {
		t.Errorf("calm: %v", got)
	}
	if got := white(p.Beat(moment(8), 1), lights); got != nil {
		t.Errorf("calm again: %v", got)
	}
}

func TestMoreBreatheLightsTakeTurns(t *testing.T) {
	c := whites("pulse", 5)
	p := NewPlanner(c)
	p.Start(t0)
	var got []string
	for i := range 3 {
		var ups []string
		for _, m := range p.Beat(moment(4*i), 0) {
			if m.Drop && m.Payload["color"] == nil {
				ups = append(ups, m.Light)
			}
		}
		got = append(got, fmt.Sprint(ups))
	}
	want := []string{"[lamp-4 lamp-5 lamp-6]", "[lamp-7 lamp-8 lamp-4]", "[lamp-5 lamp-6 lamp-7]"}
	if !slices.Equal(got, want) {
		t.Errorf("pulses %v, want %v", got, want)
	}
}

func TestBreatheRangeFromTheNextMoment(t *testing.T) {
	p := NewPlanner(whites("pulse", 1))
	p.Start(t0)
	p.SetEffects(style(func(e *config.Effects) { e.BreatheVariant, e.BreatheRange = "pulse", [2]int{20, 60} }))
	if got := white(p.Beat(moment(4), 0), []string{"lamp-4"}); !slices.Equal(got, []string{fmt.Sprintf("lamp-4:%d+", level(60)), fmt.Sprintf("lamp-4:%d", level(20))}) {
		t.Errorf("got %v", got)
	}
}
