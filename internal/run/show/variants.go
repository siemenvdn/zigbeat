package show

import (
	"fmt"
	"slices"

	"github.com/siemenvdn/zigbeat/internal/run/flow"
)

// variantState is what the variant that plays keeps between moments; it starts
// over when another variant plays.
type variantState struct {
	deck   [][]string // shuffle: the units still to come
	dealt  string     // shuffle: the unit list the deck was dealt from
	turn   int        // swap: the group whose turn it is
	target string     // build: the colour the room fills with
}

// shuffled takes as many units as walk would from a shuffled deck: every unit
// once before any comes back. Another unit list, after the flip of alternate,
// gets a new deck.
func (p *Planner) shuffled() [][]string {
	us := p.unitList()
	if key := fmt.Sprint(us); len(p.deck) == 0 || key != p.dealt {
		p.deck, p.dealt = slices.Clone(us), key
		p.rng.Shuffle(len(p.deck), func(i, j int) { p.deck[i], p.deck[j] = p.deck[j], p.deck[i] })
	}
	picked := p.fill(p.deck)
	p.deck = p.deck[len(picked):]
	return picked
}

// swapped changes one group of units; the groups take turns.
func (p *Planner) swapped() [][]string {
	gs := groups(p.unitList())
	if len(gs) == 0 {
		return nil
	}
	g := gs[p.turn%len(gs)]
	p.turn++
	return [][]string{g}
}

// groups deals units round-robin over at least minSwapGroups groups, adding
// groups until none has more than flow.MaxPerMoment lights or each holds one unit.
func groups(us [][]string) [][]string {
	for n := min(minSwapGroups, len(us)); n > 0; n++ {
		gs := make([][]string, n)
		for i, u := range us {
			gs[i%n] = append(gs[i%n], u...)
		}
		tooBig := func(g []string) bool { return len(g) > flow.MaxPerMoment }
		if n == len(us) || !slices.ContainsFunc(gs, tooBig) {
			return gs
		}
	}
	return nil
}

// built changes the first unit, in unit order, that does not show the target;
// once every unit shows it, the target moves on to the next palette colour.
func (p *Planner) built() [][]string {
	us := p.unitList()
	if !slices.Contains(p.palette, p.target) {
		p.target = p.nextTarget()
	}
	for range buildPasses {
		for _, u := range us {
			if !p.shows(u, p.target) {
				return [][]string{u}
			}
		}
		p.target = p.nextTarget()
	}
	return nil
}

// buildPasses: when every unit shows the target, a second pass looks for one
// that does not show the next.
const buildPasses = 2

func (p *Planner) nextTarget() string {
	c, _ := p.take(map[string]bool{p.target: true})
	return c
}

// shows reports whether every light of u shows colour c.
func (p *Planner) shows(u []string, c string) bool {
	for _, l := range u {
		if p.current[l] != c {
			return false
		}
	}
	return true
}

// outside lists the colour lights that are not in u.
func (p *Planner) outside(u []string) []string {
	var out []string
	for _, f := range p.fixtures {
		for _, l := range f.Lights {
			if !slices.Contains(u, l) {
				out = append(out, l)
			}
		}
	}
	return out
}
