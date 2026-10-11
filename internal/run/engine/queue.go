package engine

import (
	"slices"
	"time"

	"github.com/siemenvdn/zigbeat/internal/run/show"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

const lateLimit = 150 * time.Millisecond // a moment message this late is dropped

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

// dropLate removes moment units that can no longer land on time, as long as
// none of their messages has left.
func (e *Engine) dropLate(now time.Time) {
	late := map[int]bool{}
	for _, q := range e.queue {
		for _, m := range q {
			if m.Drop && !e.started[m.Unit] && now.Sub(m.sendAt) > lateLimit {
				late[m.Unit] = true
			}
		}
	}
	if len(late) == 0 {
		return
	}
	e.dropped += len(late)
	for l, q := range e.queue {
		e.queue[l] = slices.DeleteFunc(q, func(m queued) bool { return late[m.Unit] })
	}
}
