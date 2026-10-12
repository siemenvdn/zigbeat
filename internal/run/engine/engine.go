// Package engine is the show as a state machine: the loop feeds it events and
// the current time, and publishes what Due returns. It does no I/O itself.
package engine

import (
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
	"github.com/siemenvdn/zigbeat/internal/run/show"
)

// Send is one message for the loop to publish.
type Send struct {
	Light   string
	Get     bool
	Payload map[string]any
}

// Engine is one show. Only the loop goroutine may call it.
type Engine struct {
	plan     *show.Planner
	lim      *flow.Limiter
	bo       flow.Backoff
	fl       *flow.InFlight
	canary   flow.Canary
	lights   []string
	queue    map[string][]queued // per light, by send time
	started  map[int]bool        // units with a message out
	fails    map[string]int      // failures in a row per light
	away     map[string]time.Time
	offset   time.Duration
	stopping bool
	notes    []string

	frames, errs, dropped int
	lastStatus            time.Time
}

func millis(n int) time.Duration { return time.Duration(n) * time.Millisecond }

// New starts a show from c and queues its start pass.
func New(c config.Config, canaryLim time.Duration, now time.Time) *Engine {
	e := &Engine{
		plan:       show.NewPlanner(c),
		lim:        flow.NewLimiter(c.Budget.MaxRate, now),
		fl:         flow.NewInFlight(millis(c.Budget.LatencyMS)),
		canary:     flow.NewCanary(c.Canary, canaryLim, now),
		lights:     c.Lights(),
		queue:      map[string][]queued{},
		started:    map[int]bool{},
		fails:      map[string]int{},
		away:       map[string]time.Time{},
		offset:     c.Budget.AudioOffset(),
		lastStatus: now,
	}
	e.enqueue(e.plan.Start(now))
	return e
}

// SetOffset sets the audio offset for every message planned from now on.
func (e *Engine) SetOffset(d time.Duration) { e.offset = d }

// SetEffects takes the style of the show from the next moment on; it sends
// nothing itself.
func (e *Engine) SetEffects(now time.Time, eff config.Effects) {
	e.plan.SetEffects(eff)
	e.planNotes(now)
}

// Halt stops planning and sending; messages in flight can still be confirmed.
func (e *Engine) Halt() {
	e.stopping = true
	clear(e.queue)
}

// Disconnected forgets what was in flight on a connection that is gone. The
// losses before a drop that the keepalive found late may be the connection's,
// so failure counts start over and lights left alone are read again at once.
func (e *Engine) Disconnected() {
	e.fl.Clear()
	clear(e.queue)
	e.canary.Forget()
	clear(e.fails)
	for l := range e.away {
		e.away[l] = time.Time{}
	}
}

func (e *Engine) InFlight() int {
	n := e.fl.Count()
	if e.canary.Waiting() {
		n++
	}
	return n
}

// maxInFlight is one moment's worth of messages in flight in total, or one from
// the calm level.
func (e *Engine) maxInFlight() int {
	if e.bo.Level() >= flow.CalmLevel {
		return 1
	}
	return flow.MaxPerMoment
}

func (e *Engine) note(now time.Time, s string) {
	e.notes = append(e.notes, now.Format(cli.Clock)+"  "+s)
}

// TakeNotes returns the lines to print since the last call.
func (e *Engine) TakeNotes() []string {
	n := e.notes
	e.notes = nil
	return n
}
