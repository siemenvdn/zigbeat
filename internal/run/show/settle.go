package show

import "time"

// Settle gives every breathe light its low end, once per gap: when the latest
// beat sounded more than a beat ago and no newer one came. Without it a
// brightness_move runs on to the bulb's own limit while the music is quiet.
// The next moment goes up from low.
func (p *Planner) Settle(now time.Time) []Message {
	if len(p.breathe) == 0 || p.settled || p.lastBeat.IsZero() || now.Before(p.lastBeat.Add(p.lastPeriod)) {
		return nil
	}
	p.settled, p.up = true, true
	var out []Message
	for _, l := range p.breathe {
		out = append(out, Message{Light: l, Land: now, Payload: map[string]any{"brightness": p.low}})
	}
	return out
}
