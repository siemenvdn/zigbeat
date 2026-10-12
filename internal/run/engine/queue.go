package engine

import (
	"slices"
	"time"

	"github.com/siemenvdn/zigbeat/internal/run/show"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

const lateLimit = 150 * time.Millisecond // a colour change this late is dropped

type queued struct {
	show.Message
	sendAt time.Time
}

func (e *Engine) enqueue(ps []show.Message) {
	est := e.fl.Estimate()
	for _, p := range ps {
		if _, gone := e.away[p.Light]; gone {
			continue
		}
		frames := time.Duration(max(1, z2m.Frames(p.Payload)))
		q := queued{Message: p, sendAt: p.Land.Add(e.offset - frames*est)}
		list := e.queue[p.Light]
		i := 0
		for i < len(list) && !list[i].sendAt.After(q.sendAt) {
			i++
		}
		e.queue[p.Light] = slices.Insert(list, i, q)
	}
}

// ready lists the free lights with a due message, oldest send time first.
func (e *Engine) ready(now time.Time) []string {
	var ls []string
	for _, l := range e.lights {
		if q := e.queue[l]; len(q) > 0 && !q[0].sendAt.After(now) && !e.fl.Busy(l) {
			ls = append(ls, l)
		}
	}
	slices.SortStableFunc(ls, func(a, b string) int {
		return e.queue[a][0].sendAt.Compare(e.queue[b][0].sendAt)
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
	for l, q := range e.queue {
		var missed []show.Message
		e.queue[l] = slices.DeleteFunc(q, func(m queued) bool {
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

// clearQueue empties the queue of every light for which gone holds; the
// planner hears of each colour change that will not go.
func (e *Engine) clearQueue(gone func(light string) bool) {
	e.drop(func(m queued) bool { return gone(m.Light) })
	for l := range e.queue {
		if gone(l) {
			delete(e.queue, l)
		}
	}
}

// every is a clearQueue filter for every light.
func every(string) bool { return true }
