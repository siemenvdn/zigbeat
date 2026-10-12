package show

import (
	"fmt"
	"slices"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
)

// quietEnergy: a bar below it is a quiet part. On real music a breakdown
// measured 0, a verse 0.15 to 0.2, a pre-chorus 0.77 and a drop above 0.9.
const quietEnergy = 0.3

// energyState is what the show keeps of the music's energy.
type energyState struct {
	follow bool // effects.follow_energy
	lowBar bool // the bar that plays is below quietEnergy
	quiet  bool // the show plays a quiet part: no kicks, the white low, one pace slower
}

// hear follows the energy of b: at each bar line the bar's energy decides
// whether the show plays a quiet part, and a change is noted.
func (p *Planner) hear(b music.Beat) {
	if b.InBar == 0 {
		p.lowBar = b.Energy < quietEnergy
	}
	quiet := p.follow && p.lowBar
	if quiet == p.quiet {
		return
	}
	p.quiet = quiet
	how := "full"
	if quiet {
		how = "quiet, no kicks"
	}
	p.notes = append(p.notes, fmt.Sprintf("energy %s from bar %d: every %s", how, (b.Index-b.InBar)/music.PerBar, music.Span(p.paceNow())))
}

// paceNow is the pace that plays: effects.pace, one step slower in a quiet part.
func (p *Planner) paceNow() int {
	i := slices.Index(config.Paces, p.pace)
	if !p.quiet || i < 0 || i == len(config.Paces)-1 {
		return p.pace
	}
	return config.Paces[i+1]
}
