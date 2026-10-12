package show

import "github.com/siemenvdn/zigbeat/internal/music"

const (
	// phraseBars: dance music is built in sections of 8 and 16 bars.
	phraseBars  = 16
	phraseBeats = phraseBars * music.PerBar
	// minSwapGroups: two groups dealt round-robin, so each group spans the room.
	minSwapGroups = 2
)

// rotation is the order of the variants after the build that opens a
// timeline: from many colours to two.
var rotation = []string{"walk", "shuffle", "swap"}

// variantAt is the variant of the moment at index: the one in the config, or
// in rotation build for a timeline's first phrase and then one per phrase.
func (p *Planner) variantAt(index int) string {
	if p.setting != "rotation" {
		return p.setting
	}
	if index < phraseBeats {
		return "build"
	}
	return rotation[(index/phraseBeats-1)%len(rotation)]
}

// pickFor lists the units variant v changes at this moment.
func (p *Planner) pickFor(v string) [][]string {
	switch v {
	case "shuffle":
		return p.shuffled()
	case "swap":
		return p.swapped()
	case "build":
		return p.built()
	}
	return p.walk()
}

// colorOf is the colour unit u takes in variant v.
func (p *Planner) colorOf(v string, u []string) string {
	switch v {
	case "build":
		return p.target
	case "swap":
		return p.colorFor(u, p.outside(u))
	}
	return p.colorFor(u, p.fixtureOf[u[0]])
}
