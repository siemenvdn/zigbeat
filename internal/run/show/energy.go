package show

import "github.com/siemenvdn/zigbeat/internal/music"

const (
	quietEnergy = 0.3
	fullEnergy  = music.UnknownEnergy
)

type energy struct {
	follow  bool
	lowPart bool
	quiet   bool
}

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
