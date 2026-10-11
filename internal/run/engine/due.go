package engine

import (
	"fmt"
	"maps"
	"time"

	"github.com/siemenvdn/zigbeat/internal/run/flow"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// Due expires old messages and returns what must be published now.
func (e *Engine) Due(now time.Time) []Send {
	for _, l := range e.fl.Expire(now) {
		if !e.failed(now, l) {
			e.raise(now, flow.PauseLevel, l+" did not answer within 5 s")
		}
	}
	if e.canary.Expire(now) {
		e.raise(now, flow.PauseLevel, "canary did not answer within 5 s")
	}
	if e.bo.Tick(now) {
		e.note(now, fmt.Sprintf("backoff %d: quiet for a minute", e.bo.Level()))
	}
	if e.stopping {
		return nil
	}
	e.dropLate(now)

	var out []Send
	if e.canary.Due(now) && e.InFlight() < e.maxInFlight() && e.lim.Take(now, 1) {
		e.canary.Sent(now)
		out = append(out, Send{Light: e.canary.Name(), Get: true, Payload: map[string]any{"state": ""}})
	}
	if e.bo.Level() >= flow.PauseLevel {
		return out
	}
	out = append(out, e.retryAway(now)...)
	for _, l := range e.ready(now) {
		if e.InFlight() >= e.maxInFlight() {
			break
		}
		q := e.queue[l]
		n := 0
		for n < len(q) && !q[n].sendAt.After(now) {
			n++
		}
		payload := map[string]any{}
		for _, m := range q[:n] {
			maps.Copy(payload, m.Payload)
		}
		frames := z2m.Frames(payload)
		if !e.lim.Take(now, frames) {
			break
		}
		for _, m := range q[:n] {
			if m.Unit > 0 {
				e.started[m.Unit] = true
			}
		}
		e.queue[l] = q[n:]
		e.fl.Add(l, now, z2m.Echoes(payload), frames)
		e.frames += frames
		out = append(out, Send{Light: l, Payload: payload})
	}
	return out
}
