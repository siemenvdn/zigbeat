package music

import "fmt"

// Span names a number of beats the way a musician counts them: beat, 2 beats,
// bar, 2 bars. Whole bars are counted in bars.
func Span(beats int) string {
	switch {
	case beats == 1:
		return "beat"
	case beats == PerBar:
		return "bar"
	case beats > PerBar && beats%PerBar == 0:
		return fmt.Sprintf("%d bars", beats/PerBar)
	}
	return fmt.Sprintf("%d beats", beats)
}
