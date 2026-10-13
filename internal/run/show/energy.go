package show

import "github.com/siemenvdn/zigbeat/internal/music"

// A bar below quietEnergy starts a quiet part, a bar from fullEnergy on ends
// it. On real music a breakdown measured 0, a verse 0.15 to 0.2, a pre-chorus
// 0.77 and a drop above 0.9; a build-up after a breakdown wavered between 0.2
// and 0.4, which one threshold turned into a flicker between quiet and full.
const (
	quietEnergy = 0.3
	fullEnergy  = music.UnknownEnergy
)

// energy is what the show keeps of the music's energy.
type energy struct {
	follow  bool // effects.follow_energy
	lowPart bool // the music plays a low part: since a bar below quietEnergy, until one from fullEnergy
	quiet   bool // the show plays a quiet part: no kicks, the breathe lights low, one pace slower
}

// hear follows the energy of b: at each bar line the bar's energy may start or
// end a quiet part. It reports whether the show turned quiet or full.
func (e *energy) hear(b music.Beat) bool {
	switch {
	case b.InBar != 0:
	case b.Energy < quietEnergy:
		e.lowPart = true
	case b.Energy >= fullEnergy:
		e.lowPart = false
	}
	quiet := e.follow && e.lowPart
	changed := quiet != e.quiet
	e.quiet = quiet
	return changed
}

// String is how the show plays now, for a note.
func (e energy) String() string {
	if e.quiet {
		return "quiet, no kicks"
	}
	return "full"
}
