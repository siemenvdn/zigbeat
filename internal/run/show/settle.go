package show

import "time"

// Settle gives every breathe light its low end, once per gap: when the latest
// beat sounded more than a beat ago and no newer one came. A flip or a fall
// that a pause cleared can leave a light high while the music is quiet.
func (p *Planner) Settle(now time.Time) []Message {
	if len(p.breathe) == 0 || p.settled || p.lastBeat.IsZero() || now.Before(p.lastBeat.Add(p.lastPeriod)) {
		return nil
	}
	p.settled = true
	clear(p.lit)
	var out []Message
	for _, l := range p.breathe {
		out = append(out, Message{Light: l, Land: now, Payload: bright(p.low)})
	}
	return out
}
