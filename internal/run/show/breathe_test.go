package show

import (
	"fmt"
	"strings"
	"testing"
)

func TestBreathe(t *testing.T) {
	c := testConfig("separate")
	c.Effects.BreatheRange = [2]int{30, 80}
	p := NewPlanner(c)
	var moves []string
	for i := 0; i < 5; i++ {
		for _, pl := range p.Beat(moment(4*i), 0) {
			if pl.Light == "lamp-4" {
				moves = append(moves, fmt.Sprint(pl.Payload["brightness_move"], "/", pl.Payload["brightness"]))
			}
		}
	}
	// 30%..80% is 76..203: 127 levels in 2 s is 64 per second. Moments 0 and 4 anchor.
	if want := "64/76 -64/<nil> 64/<nil> -64/<nil> 64/76"; strings.Join(moves, " ") != want {
		t.Errorf("breathe %v, want %s", moves, want)
	}
}
