package show

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestMoreBreatheLightsTakeTurns(t *testing.T) {
	c := whites("pulse", 5)
	p := NewPlanner(c)
	p.Start(t0)
	var got []string
	for i := range 3 {
		var ups []string
		for _, m := range p.Beat(moment(4*i), false) {
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

// TestWhiteTakesTurnsWhenTheRoomIsShort.
func TestWhiteTakesTurnsWhenTheRoomIsShort(t *testing.T) {
	c := whites("pulse", 2)
	c.Budget.MaxRate = 8
	c.Effects.Pace = 2
	p := NewPlanner(c)
	p.Start(t0)
	var got []string
	for i := range 4 {
		ws := white(p.Beat(moment(2*i), false), c.Fixtures[1].Lights)
		if len(ws) != 2 {
			t.Fatalf("moment %d: %v", i, ws)
		}
		got = append(got, strings.SplitN(ws[0], ":", 2)[0])
	}
	if want := []string{"lamp-4", "lamp-5", "lamp-4", "lamp-5"}; !slices.Equal(got, want) {
		t.Errorf("pulsed %v, want %v", got, want)
	}
}
