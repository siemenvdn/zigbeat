package engine

import (
	"fmt"
	"maps"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// Due expires old messages and returns what must be published now.
func (e *Engine) Due(now time.Time) []Send {
	e.expire(now)
	if e.stopping {
		return nil
	}
	if e.bo.Level() < flow.PauseLevel {
		e.enqueue(e.plan.Settle(now))
	}
	e.dropLate(now)
	var out []Send
	if e.canary.Due(now) && e.InFlight() < e.maxInFlight() && e.lim.AllowN(now, 1) {
		e.canary.Sent(now)
		out = append(out, Send{Light: e.canary.Name(), Get: true, Payload: z2m.StateQuery})
	}
	if e.bo.Level() >= flow.PauseLevel {
		return out
	}
	out = append(out, e.retryAway(now)...)
	return append(out, e.send(now)...)
}

func (e *Engine) expire(now time.Time) {
	for _, l := range e.fl.Expire(now) {
		if !e.failed(now, l) {
			e.raise(now, flow.PauseLevel, l+" did not answer within "+cli.Seconds(z2m.LostAfter))
		}
	}
	if e.canary.Expire(now) {
		e.raise(now, flow.PauseLevel, "canary did not answer within "+cli.Seconds(z2m.LostAfter))
	}
	if e.bo.Tick(now) {
		e.note(now, fmt.Sprintf("backoff %d: quiet for %s", e.bo.Level(), cli.Seconds(flow.RecoverAfter)))
	}
}

func (e *Engine) send(now time.Time) []Send {
	var out []Send
	for _, name := range e.ready(now) {
		if e.InFlight() >= e.maxInFlight() {
			break
		}
		l := e.lights[name]
		n := 0
		for n < len(l.queue) && !l.queue[n].sendAt.After(now) {
			n++
		}
		payload := map[string]any{}
		for _, m := range l.queue[:n] {
			maps.Copy(payload, m.Payload)
		}
		frames := z2m.Frames(payload)
		if !e.lim.AllowN(now, frames) {
			break
		}
		l.queue = l.queue[n:]
		e.fl.Add(name, now, frames)
		e.frames += frames
		out = append(out, Send{Light: name, Payload: payload})
	}
	return out
}
