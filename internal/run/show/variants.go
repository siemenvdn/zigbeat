package show

import (
	"fmt"
	"slices"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
)

const (
	// phraseBars: dance music is built in sections of 8 and 16 bars.
	phraseBars  = 16
	phraseBeats = phraseBars * music.PerBar
	// chaseShare: a moment changes about 1 in chaseShare of the colour lights, so
	// the chase moves and the room keeps its colour.
	chaseShare = 2
	// minSwapGroups: two groups dealt round-robin, so each group spans the room.
	minSwapGroups = 2
	// buildPasses: when every unit shows the target, a second pass looks for one
	// that does not show the next.
	buildPasses = 2
)

// rotation is the order of the variants after the build that opens a
// timeline: from many colours to two.
var rotation = []config.Variant{config.Walk, config.Shuffle, config.Swap}

// variant is one way the colour lights change; it keeps its own state between moments.
type variant interface {
	pick(c *colorShow, st stroke) [][]string // the units that change at this moment
	color(c *colorShow, u []string) string   // the colour unit u takes
}

func newVariant(v config.Variant) variant {
	switch v {
	case config.Shuffle:
		return &shuffle{}
	case config.Swap:
		return &swap{}
	case config.Build:
		return &build{}
	}
	return walk{}
}

// variantAt is the variant of the moment at index: the one in the config, or
// in rotation build for a timeline's first phrase and then one per phrase.
func (c *colorShow) variantAt(index int) config.Variant {
	if c.setting != config.Rotation {
		return c.setting
	}
	if index < phraseBeats {
		return config.Build
	}
	return rotation[(index/phraseBeats-1)%len(rotation)]
}

// fill takes units in order until 1 in chaseShare colour lights are covered,
// at most flow.MaxPerMoment messages; the first unit always goes, whatever its size.
func (c *colorShow) fill(units [][]string) [][]string {
	want := (c.n + chaseShare - 1) / chaseShare
	var picked [][]string
	lights := 0
	for _, u := range units {
		if len(picked) > 0 && (lights >= want || lights+len(u) > flow.MaxPerMoment) {
			break
		}
		picked = append(picked, u)
		lights += len(u)
	}
	return picked
}

// fixtureColor is a colour that no other light of u's fixture shows.
func fixtureColor(c *colorShow, u []string) string { return c.colorFor(u, c.fixtureOf[u[0]]) }

// walk takes units in config order from where the last moment stopped.
type walk struct{}

func (walk) pick(c *colorShow, st stroke) [][]string {
	us := c.unitList(st.moment)
	if len(us) == 0 {
		return nil
	}
	from := c.cursor % len(us)
	picked := c.fill(append(us[from:len(us):len(us)], us[:from]...))
	c.cursor = (from + len(picked)) % len(us)
	return picked
}

func (walk) color(c *colorShow, u []string) string { return fixtureColor(c, u) }

// shuffle takes as many units as walk from a shuffled deck: every unit once
// before any comes back.
type shuffle struct {
	deck  [][]string // the units still to come
	dealt string     // the unit list the deck was dealt from
}

// pick deals a new deck when it is empty or the unit list changed, after the flip of alternate.
func (s *shuffle) pick(c *colorShow, st stroke) [][]string {
	us := c.unitList(st.moment)
	if key := fmt.Sprint(us); len(s.deck) == 0 || key != s.dealt {
		s.deck, s.dealt = slices.Clone(us), key
		c.rng.Shuffle(len(s.deck), func(i, j int) { s.deck[i], s.deck[j] = s.deck[j], s.deck[i] })
	}
	picked := c.fill(s.deck)
	s.deck = s.deck[len(picked):]
	return picked
}

func (*shuffle) color(c *colorShow, u []string) string { return fixtureColor(c, u) }

// swap changes one group of units at a time; the groups take turns, each in one colour.
type swap struct{ turn int }

func (s *swap) pick(c *colorShow, st stroke) [][]string {
	gs := groups(c.unitList(st.moment))
	if len(gs) == 0 {
		return nil
	}
	g := gs[s.turn%len(gs)]
	s.turn++
	return [][]string{g}
}

// color avoids the group's own colour too: with as many groups as colours, the
// others leave it no other.
func (*swap) color(c *colorShow, u []string) string { return c.colorFor(u, append(c.outside(u), u...)) }

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

// build changes the first unit, in unit order, that does not show the target;
// once every unit shows it, the target moves on to the next palette colour.
type build struct{ target string }

func (b *build) pick(c *colorShow, st stroke) [][]string {
	us := c.unitList(st.moment)
	if !slices.Contains(c.palette.colors, b.target) {
		b.next(c)
	}
	for range buildPasses {
		for _, u := range us {
			if !c.shows(u, b.target) {
				return [][]string{u}
			}
		}
		b.next(c)
	}
	return nil
}

func (b *build) color(*colorShow, []string) string { return b.target }

func (b *build) next(c *colorShow) { b.target, _ = c.palette.take(map[string]bool{b.target: true}) }
