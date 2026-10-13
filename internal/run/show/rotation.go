package show

import (
	"fmt"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
)

const (
	// phraseBars: dance music is built in sections of 8 and 16 bars.
	phraseBars  = 16
	phraseBeats = phraseBars * music.PerBar
	// minSwapGroups: two groups dealt round-robin, so each group spans the room.
	minSwapGroups = 2
)

// rotation is the order of the variants after the build that opens a
// timeline: from many colours to two.
var rotation = []config.Variant{config.Walk, config.Shuffle, config.Swap}

// variantAt is the variant of the moment at index: the one in the config, or
// in rotation build for a timeline's first phrase and then one per phrase.
func (p *Planner) variantAt(index int) config.Variant {
	if p.setting != config.Rotation {
		return p.setting
	}
	if index < phraseBeats {
		return config.Build
	}
	return rotation[(index/phraseBeats-1)%len(rotation)]
}

// noteVariants starts the variants of the moment at b and notes a change of
// either: one line for the colour and the breathe variant.
func (p *Planner) noteVariants(b music.Beat) {
	v, w := p.variantAt(b.Index), p.breatheAt(b.Index)
	if v != p.playing {
		p.variantState = variantState{}
	}
	if v == p.playing && w == p.breathing {
		return
	}
	p.playing, p.breathing = v, w
	note := "variant " + string(v)
	if w != "" {
		note += ", breathe " + string(w)
	}
	p.notes = append(p.notes, fmt.Sprintf("%s from bar %d", note, b.Bar()))
}

// pickFor lists the units variant v changes at this moment.
func (p *Planner) pickFor(v config.Variant) [][]string {
	switch v {
	case config.Shuffle:
		return p.shuffled()
	case config.Swap:
		return p.swapped()
	case config.Build:
		return p.built()
	}
	return p.walk()
}

// colorOf is the colour unit u takes in variant v. A swap group avoids its own
// colour too: with as many groups as colours, the others leave it no other.
func (p *Planner) colorOf(v config.Variant, u []string) string {
	switch v {
	case config.Build:
		return p.target
	case config.Swap:
		return p.colorFor(u, append(p.outside(u), u...))
	}
	return p.colorFor(u, p.fixtureOf[u[0]])
}
