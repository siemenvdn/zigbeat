package show

import (
	"fmt"
	"slices"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
)

const (
	phraseBars  = 16
	phraseBeats = phraseBars * music.PerBar

	chaseShare = 2

	minSwapGroups = 2

	buildPasses = 2
)

var rotation = []config.Variant{config.Walk, config.Shuffle, config.Swap}

type variant interface {
	pick(c *colorShow, st stroke) [][]string
	color(c *colorShow, u []string) string
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

func (c *colorShow) variantAt(index int) config.Variant {
	if c.setting != config.Rotation {
		return c.setting
	}
	if index < phraseBeats {
		return config.Build
	}
	return rotation[(index/phraseBeats-1)%len(rotation)]
}

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

func fixtureColor(c *colorShow, u []string) string { return c.colorFor(u, c.fixtureOf[u[0]]) }

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

type shuffle struct {
	deck  [][]string
	dealt string
}

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

func (*swap) color(c *colorShow, u []string) string { return c.colorFor(u, append(c.outside(u), u...)) }

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
