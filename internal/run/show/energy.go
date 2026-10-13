package show

import (
	"fmt"
	"slices"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
)

// A bar below quietEnergy starts a quiet part, a bar from fullEnergy on ends
// it. On real music a breakdown measured 0, a verse 0.15 to 0.2, a pre-chorus
// 0.77 and a drop above 0.9; a build-up after a breakdown wavered between 0.2
// and 0.4, which one threshold turned into a flicker between quiet and full.
const (
	quietEnergy = 0.3
	fullEnergy  = music.UnknownEnergy
)

// energyState is what the show keeps of the music's energy.
type energyState struct {
	follow  bool // effects.follow_energy
	lowPart bool // the music plays a low part: since a bar below quietEnergy, until one from fullEnergy
	quiet   bool // the show plays a quiet part: no kicks, the white low, one pace slower
}

// hear follows the energy of b: at each bar line the bar's energy may start or
// end a quiet part, and a change is noted.
func (p *Planner) hear(b music.Beat) {
	switch {
	case b.InBar != 0:
	case b.Energy < quietEnergy:
		p.lowPart = true
	case b.Energy >= fullEnergy:
		p.lowPart = false
	}
	quiet := p.follow && p.lowPart
	if quiet == p.quiet {
		return
	}
	p.quiet = quiet
	how := "full"
	if quiet {
		how = "quiet, no kicks"
	}
	p.notes = append(p.notes, fmt.Sprintf("energy %s from bar %d: every %s", how, b.Bar(), music.Span(p.paceNow())))
}

// paceNow is the pace that plays: effects.pace, one step slower in a quiet part.
func (p *Planner) paceNow() int {
	i := slices.Index(config.Paces, p.pace)
	if !p.quiet || i < 0 || i == len(config.Paces)-1 {
		return p.pace
	}
	return config.Paces[i+1]
}
