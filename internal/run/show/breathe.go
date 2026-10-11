package show

import (
	"math"
	"time"

	"github.com/siemenvdn/zigbeat/internal/beat"
)

const anchorEvery = 4 // moments between absolute breathe brightness

// breath flips every breathe light; every anchorEvery moments it also sets the
// turning point as an absolute level against drift.
func (p *Planner) breath(b beat.Beat, n int) []Message {
	if len(p.breathe) == 0 {
		return nil
	}
	until := time.Duration(n) * b.Period()
	rate := max(1, int(math.Round(float64(p.high-p.low)/until.Seconds())))
	move, bound := rate, p.low
	if !p.up {
		move, bound = -rate, p.high
	}
	p.up = !p.up
	var out []Message
	for _, l := range p.breathe {
		pl := map[string]any{"brightness_move": move}
		if p.moments%anchorEvery == 0 {
			pl["brightness"] = bound
		}
		out = append(out, Message{Light: l, Land: b.At, Payload: pl})
	}
	return out
}
