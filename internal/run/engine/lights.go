package engine

import (
	"slices"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/run/show"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

const (
	lateLimit = 150 * time.Millisecond // a colour change this late is dropped
	awayAfter = 2                      // failures in a row before a light is left alone
	retryAway = time.Minute            // how often a light that is left alone is read
)

// light is what the engine keeps per light.
type light struct {
	queue   []queued  // by send time
	fails   int       // failures in a row
	away    bool      // left alone: it did not answer awayAfter times in a row
	retryAt time.Time // when a light left alone is read again
}

type queued struct {
	show.Message
	sendAt time.Time
}

// enqueue queues each message to leave early by its frames times the latency
// estimate, plus the audio offset. Lights left alone get nothing.
func (e *Engine) enqueue(ps []show.Message) {
	est := e.fl.Estimate()
	for _, p := range ps {
		l := e.lights[p.Light]
		if l == nil || l.away {
			continue
		}
		frames := time.Duration(max(1, z2m.Frames(p.Payload)))
		q := queued{Message: p, sendAt: p.Land.Add(e.offset - frames*est)}
		i := 0
		for i < len(l.queue) && !l.queue[i].sendAt.After(q.sendAt) {
			i++
		}
		l.queue = slices.Insert(l.queue, i, q)
	}
}

// ready lists the free lights with a due message, oldest send time first.
func (e *Engine) ready(now time.Time) []string {
	var ls []string
	for _, name := range e.order {
		if q := e.lights[name].queue; len(q) > 0 && !q[0].sendAt.After(now) && !e.fl.Busy(name) {
			ls = append(ls, name)
		}
	}
	slices.SortStableFunc(ls, func(a, b string) int {
		return e.lights[a].queue[0].sendAt.Compare(e.lights[b].queue[0].sendAt)
	})
	return ls
}

// dropLate drops the colour changes that can no longer land on time, each on
// its own, and counts them. Their returns stay.
func (e *Engine) dropLate(now time.Time) {
	e.dropped += e.drop(func(m queued) bool { return now.Sub(m.sendAt) > lateLimit })
}

// forget drops the colour changes planned from beats that no longer apply,
// because the music jumped or stopped. Returns and breathe stay.
func (e *Engine) forget() { e.drop(func(queued) bool { return true }) }

// drop removes every droppable message for which late holds, tells the
// planner, and returns how many it removed. A light's messages go to the
// planner together, in queue order, so it can take back the newest first.
func (e *Engine) drop(late func(queued) bool) int {
	n := 0
	for _, l := range e.lights {
		var missed []show.Message
		l.queue = slices.DeleteFunc(l.queue, func(m queued) bool {
			gone := m.Drop && late(m)
			if gone {
				missed = append(missed, m.Message)
			}
			return gone
		})
		e.plan.Missed(missed...)
		n += len(missed)
	}
	return n
}

// clearQueues empties the queue of every light for which gone holds; the
// planner hears of each colour change that will not go.
func (e *Engine) clearQueues(gone func(light string) bool) {
	e.drop(func(m queued) bool { return gone(m.Light) })
	for name, l := range e.lights {
		if gone(name) {
			l.queue = nil
		}
	}
}

// every is a clearQueues filter for every light.
func every(string) bool { return true }

// Away reports whether light is left alone.
func (e *Engine) Away(light string) bool {
	l := e.lights[light]
	return l != nil && l.away
}

// failed counts a lost or refused message. A light that fails awayAfter times in a
// row is left alone, so one light switched off at the wall cannot drag the rest of
// the show into backoff. It returns true when the light is left alone.
func (e *Engine) failed(now time.Time, name string) bool {
	l := e.lights[name]
	if l == nil || l.away {
		return true
	}
	if l.fails++; l.fails < awayAfter {
		return false
	}
	l.away, l.retryAt = true, now.Add(retryAway)
	e.clearQueues(func(n string) bool { return n == name })
	e.note(now, name+" does not answer; leaving it alone and retrying every "+cli.Seconds(retryAway))
	return true
}

// retryAway reads each light that is left alone once per retryAway.
func (e *Engine) retryAway(now time.Time) []Send {
	var out []Send
	for _, name := range e.order {
		l := e.lights[name]
		if !l.away || now.Before(l.retryAt) || e.fl.Busy(name) || e.InFlight() >= e.maxInFlight() || !e.lim.AllowN(now, 1) {
			continue
		}
		l.retryAt = now.Add(retryAway)
		e.fl.Add(name, now, 1)
		out = append(out, Send{Light: name, Get: true, Payload: z2m.StateQuery})
	}
	return out
}

// awayCount counts the lights left alone.
func (e *Engine) awayCount() int {
	n := 0
	for _, l := range e.lights {
		if l.away {
			n++
		}
	}
	return n
}
