package engine

import (
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
)

const (
	awayAfter = 2           // failures in a row before a light is left alone
	retryAway = time.Minute // how often a light that is left alone is read
)

// Away reports whether light is left alone.
func (e *Engine) Away(light string) bool {
	_, gone := e.away[light]
	return gone
}

// failed counts a lost or refused message. A light that fails awayAfter times in a
// row is left alone, so one light switched off at the wall cannot drag the rest of
// the show into backoff. It returns true when the light is left alone.
func (e *Engine) failed(now time.Time, light string) bool {
	if _, gone := e.away[light]; gone {
		return true
	}
	e.fails[light]++
	if e.fails[light] < awayAfter {
		return false
	}
	e.away[light] = now.Add(retryAway)
	delete(e.queue, light)
	e.note(now, light+" does not answer; leaving it alone and retrying every "+cli.Seconds(retryAway))
	return true
}

// retryAway reads each light that is left alone once per retryAway.
func (e *Engine) retryAway(now time.Time) []Send {
	var out []Send
	for _, l := range e.lights {
		next, gone := e.away[l]
		if !gone || now.Before(next) || e.fl.Busy(l) || e.InFlight() >= e.maxInFlight() || !e.lim.AllowN(now, 1) {
			continue
		}
		e.away[l] = now.Add(retryAway)
		e.fl.Add(l, now, true, 1)
		out = append(out, Send{Light: l, Get: true, Payload: map[string]any{"state": ""}})
	}
	return out
}
