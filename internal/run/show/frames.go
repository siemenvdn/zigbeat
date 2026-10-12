package show

import "github.com/siemenvdn/zigbeat/internal/z2m"

// fits reports whether lights colour changes with their kicks fit room.
func (p *Planner) fits(lights int, room float64) bool {
	return float64(lights*p.changeFrames(true)) <= room
}

// changeFrames is what one light's colour change costs, counted as the rate
// limiter counts it: with its kick, also the return.
func (p *Planner) changeFrames(kick bool) int {
	n := z2m.Frames(p.changePayload("", stroke{blend: colorBlend, kick: kick}))
	if kick {
		n += z2m.Frames(p.returnPayload())
	}
	return n
}

// lightsIn counts the lights units change at this moment: a unit too large
// changes its first part now.
func lightsIn(units [][]string) int {
	n := 0
	for _, u := range units {
		n += len(split(u)[0])
	}
	return n
}

// framesOf is what messages cost, counted as the rate limiter counts them.
func framesOf(ms []Message) int {
	n := 0
	for _, m := range ms {
		n += z2m.Frames(m.Payload)
	}
	return n
}
