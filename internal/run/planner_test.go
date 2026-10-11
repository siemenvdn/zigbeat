package run

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/beat"
	"github.com/siemenvdn/zigbeat/internal/config"
)

func testConfig(mode string) config.Config {
	return config.Config{
		MQTT:   config.MQTT{Broker: "b:1883", BaseTopic: "zigbee2mqtt"},
		Budget: config.Budget{MaxRate: 100, LatencyMS: 50},
		Fixtures: []config.Fixture{
			{Name: "pendant", Role: "color", Mode: mode, Lights: []string{"lamp-1", "lamp-2", "lamp-3"}},
			{Name: "floor", Role: "breathe", Lights: []string{"lamp-4"}},
		},
		Effects: config.Effects{Cadence: 2 * time.Second, Brightness: 60, Palette: config.DefaultPalette, BreatheRange: [2]int{30, 80}},
	}
}

// moment returns beat i at 120 BPM, where every 4th beat is a moment.
func moment(i int) beat.Beat {
	return beat.Beat{At: t0.Add(time.Duration(i) * 500 * time.Millisecond), BPM: 120, Index: i, InBar: i % 4, Energy: 1}
}

// changed lists the colour lights that got a colour in ps.
func changed(ps []planned) string {
	var ls []string
	for _, p := range ps {
		if _, ok := p.payload["color"]; ok {
			ls = append(ls, p.light)
		}
	}
	return strings.Join(ls, ",")
}

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
	p := newPlanner(testConfig("separate"))
	if ps := p.beat(moment(1), 0); ps != nil {
		t.Errorf("beat 1 is no moment: %v", ps)
	}
	if ps := p.beat(moment(4), 0); len(ps) == 0 {
		t.Error("beat 4 is a moment")
	}
	if ps := p.beat(moment(4), 2); ps != nil {
		t.Error("backoff 2 doubles the interval: beat 4 is no moment")
	}
}

func TestChaseSeparate(t *testing.T) {
	p := newPlanner(testConfig("separate"))
	var got []string
	for i := 0; i < 3; i++ {
		got = append(got, changed(p.beat(moment(4*i), 0)))
	}
	if want := "lamp-1,lamp-2 lamp-3,lamp-1 lamp-2,lamp-3"; strings.Join(got, " ") != want {
		t.Errorf("chase %v, want %s", got, want)
	}
}

func TestAlternateFlipsEveryFourMoments(t *testing.T) {
	p := newPlanner(testConfig("alternate"))
	var got []string
	for i := 0; i < 9; i++ {
		got = append(got, changed(p.beat(moment(4*i), 0)))
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
	p := newPlanner(testConfig("together"))
	p.start(t0)
	for i := 0; i < 6; i++ {
		before := p.current["lamp-1"]
		colours := map[string]bool{}
		for _, pl := range p.beat(moment(4*i), 0) {
			if c, ok := pl.payload["color"]; ok {
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

func TestNoRepeatedColour(t *testing.T) {
	p := newPlanner(testConfig("separate"))
	p.start(t0)
	for i := 0; i < 30; i++ {
		before := map[string]string{}
		for l, c := range p.current {
			before[l] = c
		}
		for _, pl := range p.beat(moment(4*i), 0) {
			if c, ok := pl.payload["color"].(map[string]string); ok && c["hex"] == before[pl.light] {
				t.Fatalf("moment %d: %s got its own colour %s again", i, pl.light, c["hex"])
			}
		}
		if p.current["lamp-1"] == p.current["lamp-2"] || p.current["lamp-2"] == p.current["lamp-3"] || p.current["lamp-1"] == p.current["lamp-3"] {
			t.Fatalf("moment %d: neighbours share a colour: %v", i, p.current)
		}
	}
}

func TestKickAndReturn(t *testing.T) {
	p := newPlanner(testConfig("separate"))
	ps := p.beat(moment(0), 0)
	kick, ret := ps[0], ps[1]
	if kick.payload["brightness"] != 229 || !kick.drop || kick.unit == 0 || !kick.land.Equal(t0) {
		t.Errorf("kick: %+v", kick)
	}
	if ret.payload["brightness"] != 152 || ret.drop || ret.unit != kick.unit || !ret.land.Equal(at(500)) {
		t.Errorf("return: %+v", ret)
	}
	for _, pl := range p.beat(moment(4), 1) {
		if _, ok := pl.payload["color"]; ok && pl.payload["brightness"] != nil {
			t.Errorf("backoff 1 has no kick: %v", pl.payload)
		}
	}
}

func TestBreathe(t *testing.T) {
	p := newPlanner(testConfig("separate"))
	var moves []string
	for i := 0; i < 5; i++ {
		for _, pl := range p.beat(moment(4*i), 0) {
			if pl.light == "lamp-4" {
				moves = append(moves, fmt.Sprint(pl.payload["brightness_move"], "/", pl.payload["brightness"]))
			}
		}
	}
	// 30%..80% is 76..203: 127 levels in 2 s is 64 per second. Moments 0 and 4 anchor.
	if want := "64/76 -64/<nil> 64/<nil> -64/<nil> 64/76"; strings.Join(moves, " ") != want {
		t.Errorf("breathe %v, want %s", moves, want)
	}
}

func TestStart(t *testing.T) {
	p := newPlanner(testConfig("separate"))
	ps := p.start(t0)
	if len(ps) != 4 {
		t.Fatalf("start covers every light, got %d", len(ps))
	}
	if ps[0].payload["state"] != "ON" || ps[0].payload["brightness"] != 152 || ps[0].drop {
		t.Errorf("colour start: %+v", ps[0])
	}
	if ps[3].payload["brightness"] != 76 || ps[3].payload["color"] != nil {
		t.Errorf("breathe start: %+v", ps[3])
	}
}

func TestSingleRoleConfigs(t *testing.T) {
	breatheOnly := testConfig("separate")
	breatheOnly.Fixtures = breatheOnly.Fixtures[1:]
	p := newPlanner(breatheOnly)
	p.start(t0)
	if ps := p.beat(moment(0), 0); len(ps) != 1 || ps[0].light != "lamp-4" {
		t.Errorf("breathe only: %v", ps)
	}
	colorOnly := testConfig("together")
	colorOnly.Fixtures = colorOnly.Fixtures[:1]
	p = newPlanner(colorOnly)
	p.start(t0)
	if got := changed(p.beat(moment(0), 0)); got != "lamp-1,lamp-2,lamp-3" {
		t.Errorf("color only: %q", got)
	}
}
