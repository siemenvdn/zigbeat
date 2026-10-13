package show

import (
	"maps"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/siemenvdn/zigbeat/internal/config"
)

// playing returns a planner for variant over colour fixtures of the given sizes,
// with a fixed seed.
func playing(variant config.Variant, mode config.Mode, sizes ...int) *Planner {
	c := fixtures(mode, sizes...)
	c.Effects.Variant = variant
	p := NewPlanner(c)
	p.colors.rng = rand.New(rand.NewPCG(1, 1))
	p.Start(t0)
	return p
}

func TestShuffleChangesEveryUnitOnceBeforeAnyRepeats(t *testing.T) {
	p := playing("shuffle", "separate", 6)
	var orders []string
	for round := 0; round < 3; round++ {
		var seen []string
		for i := 0; i < 2; i++ { // 3 lights per moment, 6 lights
			seen = append(seen, strings.Split(changed(p.Beat(moment(4*(2*round+i)), false)), ",")...)
		}
		sorted := slices.Clone(seen)
		slices.Sort(sorted)
		if strings.Join(sorted, ",") != "f0-1,f0-2,f0-3,f0-4,f0-5,f0-6" {
			t.Fatalf("round %d changed %v", round, seen)
		}
		orders = append(orders, strings.Join(seen, ","))
	}
	if orders[0] == orders[1] && orders[1] == orders[2] {
		t.Errorf("the deck is shuffled again every round: %v", orders)
	}
}

func TestSwapAlternatesTwoGroupsOfOneColour(t *testing.T) {
	p := playing("swap", "separate", 6)
	var got []string
	for i := 0; i < 4; i++ {
		ps := p.Beat(moment(4*i), false)
		got = append(got, changed(ps))
		distinct := map[string]bool{}
		for _, c := range colours(ps) {
			distinct[c] = true
		}
		if len(distinct) != 1 {
			t.Errorf("moment %d: one colour per group, got %v", i, colours(ps))
		}
	}
	if want := "f0-1,f0-3,f0-5 f0-2,f0-4,f0-6 f0-1,f0-3,f0-5 f0-2,f0-4,f0-6"; strings.Join(got, " ") != want {
		t.Errorf("groups %v, want %s", got, want)
	}
	if p.colors.current["f0-1"] == p.colors.current["f0-2"] {
		t.Errorf("the two groups show two colours: %v", p.colors.current)
	}
	p = playing("swap", "separate", 8)
	got = nil
	for i := 0; i < 3; i++ {
		got = append(got, changed(p.Beat(moment(4*i), false)))
	}
	if want := "f0-1,f0-4,f0-7 f0-2,f0-5,f0-8 f0-3,f0-6"; strings.Join(got, " ") != want {
		t.Errorf("8 lights make 3 groups: %v", got)
	}
}

func TestSwapChangesTheColourOfEachGroup(t *testing.T) {
	const firstRound = 3 // turns until every group has had one
	for _, c := range []struct {
		lights  int
		palette []string
	}{
		{8, []string{"#406cff", "#ff943d", "#7340ff"}}, // 3 groups, 3 colours
		{4, []string{"#406cff", "#ff943d"}},            // 2 groups, 2 colours
	} {
		cfg := fixtures("separate", c.lights)
		cfg.Effects.Variant, cfg.Effects.Palette = "swap", c.palette
		p := NewPlanner(cfg)
		p.Start(t0)
		for i := 0; i < 12; i++ {
			before := maps.Clone(p.colors.current)
			ls := strings.Split(changed(p.Beat(moment(4*i), false)), ",")
			if i < firstRound {
				continue // the groups take one colour each
			}
			for _, l := range ls {
				if p.colors.current[l] == before[l] {
					t.Fatalf("%d lights, %d colours, moment %d: %s keeps %s", c.lights, len(c.palette), i, l, before[l])
				}
			}
		}
	}
}

func TestBuildFillsTheRoomWithOneColour(t *testing.T) {
	p := playing("build", "separate", 3)
	var targets []string
	for i := 0; i < 6; i++ {
		ps := p.Beat(moment(4*i), false)
		if got := changed(ps); len(strings.Split(got, ",")) != 1 {
			t.Fatalf("moment %d changed %q, want one light", i, got)
		}
		if i%3 == 2 {
			if p.colors.current["f0-1"] != p.colors.current["f0-2"] || p.colors.current["f0-2"] != p.colors.current["f0-3"] {
				t.Errorf("after moment %d the room shows one colour: %v", i, p.colors.current)
			}
			targets = append(targets, p.colors.current["f0-1"])
		}
	}
	if targets[0] == targets[1] {
		t.Errorf("then the target moves on: %v", targets)
	}
}

func TestRotation(t *testing.T) {
	p := playing("rotation", "separate", 3)
	for index, want := range map[int]config.Variant{-4: "build", 0: "build", 60: "build", 64: "walk", 128: "shuffle", 192: "swap", 256: "walk"} {
		if got := p.colors.variantAt(index); got != want {
			t.Errorf("index %d: %s, want %s", index, got, want)
		}
	}
	if got := playing("swap", "separate", 3).colors.variantAt(0); got != "swap" {
		t.Errorf("a fixed variant plays from the start: %s", got)
	}
}

func TestVariantNotes(t *testing.T) {
	p := playing("rotation", "separate", 3)
	p.TakeNotes()
	p.Beat(moment(60), false)
	p.Beat(moment(64), false)
	if n := p.TakeNotes(); len(n) != 2 || n[0] != "variant build, breathe pulse from bar 15" || n[1] != "variant walk, breathe flip from bar 16" {
		t.Errorf("notes %q", n)
	}
	p.Beat(moment(68), false)
	if n := p.TakeNotes(); n != nil {
		t.Errorf("no note while the variant plays on: %q", n)
	}
	colourOnly := fixtures("separate", 3)
	colourOnly.Fixtures = colourOnly.Fixtures[:1]
	colourOnly.Effects.Variant = "swap"
	q := NewPlanner(colourOnly)
	q.Beat(moment(60), false)
	q.TakeNotes()
	q.Beat(moment(64), false)
	if n := q.TakeNotes(); n != nil {
		t.Errorf("without breathe lights a new phrase of the same variant says nothing: %q", n)
	}
}
