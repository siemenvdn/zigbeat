package palette

import (
	"math"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// KeyFunc returns how the key of a command is read back from a state echo.
func KeyFunc(mode Mode) func(z2m.State) (int, bool) {
	if mode == Brightness {
		return func(s z2m.State) (int, bool) {
			if s.Brightness == nil {
				return 0, false
			}
			return *s.Brightness, true
		}
	}
	return func(s z2m.State) (int, bool) {
		if s.Color == nil {
			return 0, false
		}
		return xKey(s.Color.X), true
	}
}

// xKeyScale keys x in thousandths: an echo lands within KeySlack of the key sent.
const xKeyScale = 1000

func xKey(x float64) int { return int(math.Round(x * xKeyScale)) }
